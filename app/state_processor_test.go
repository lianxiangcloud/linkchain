package app

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/lianxiangcloud/linkchain/accounts/abi"
	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	common "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/ringct"
	lktypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/xcrypto"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	types "github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/vm/evm"
	"github.com/lianxiangcloud/linkchain/wallet/wallet"
	"github.com/stretchr/testify/assert"
)

var (
	Bank []*keystore.Key
	SP   *StateProcessor
	VC   evm.Config
	APP  *LinkApplication
)

type utxoKey struct {
	Sks, Skv lktypes.SecretKey // secret key for spending & viewing
	Pks, Pkv lktypes.PublicKey // public key for spending & viewing
	Addr     lktypes.AccountAddress
	Acc      lktypes.AccountKey
	Keyi     map[lktypes.PublicKey]uint64
}

// without money
var (
	Accs     []*keystore.Key
	UtxoAccs []*utxoKey
)

type MyReader struct {
	I int
}

// TODO: Add a Read([]byte) (int, error) method to MyReader.
func (myR MyReader) Read(b []byte) (int, error) {
	b[0] = 'A' // 65
	return myR.I, nil
}

func init() {
	Bank = accounts
	APP, _ = initApp()
	SP = NewStateProcessor(nil, APP)
	APP.processor = SP
	VC = evm.Config{EnablePreimageRecording: false}
	types.SaveBalanceRecord = true

	LEN := 4
	Accs = make([]*keystore.Key, LEN)
	UtxoAccs = make([]*utxoKey, LEN)
	for i := 0; i < LEN; i++ {
		// gen acc
		s := string(crypto.Keccak512([]byte(string(i))))
		ask, err := ecdsa.GenerateKey(crypto.S256(), strings.NewReader(s))
		if err != nil {
			panic(err)
		}
		aaddr := crypto.PubkeyToAddress(ask.PublicKey)
		Accs[i] = &keystore.Key{
			PrivateKey: ask,
			Address:    aaddr,
		}
		// gen utxo
		sksr := ringct.ScalarmultH(ringct.H)
		for j := 0; j < i; j++ {
			sksr = ringct.ScalarmultH(sksr)
		}
		pksr := ringct.ScalarmultBase(sksr)
		//sksr, pksr := xcrypto.SkpkGen()
		skvr, pkvr := sksr, pksr
		sks, pks, skv, pkv := lktypes.SecretKey(sksr), lktypes.PublicKey(pksr), lktypes.SecretKey(skvr), lktypes.PublicKey(pkvr)
		addr := lktypes.AccountAddress{
			ViewPublicKey:  pkv,
			SpendPublicKey: pks,
		}
		acc := lktypes.AccountKey{
			Addr:      addr,
			SpendSKey: sks,
			ViewSKey:  skv,
			SubIdx:    uint64(0),
		}
		address := wallet.AddressToStr(&acc, uint64(0))
		acc.Address = address
		keyi := make(map[lktypes.PublicKey]uint64)
		keyi[acc.Addr.SpendPublicKey] = 0
		UtxoAccs[i] = &utxoKey{
			Sks:  sks,
			Skv:  skv,
			Pks:  pks,
			Pkv:  pkv,
			Addr: addr,
			Acc:  acc,
			Keyi: keyi,
		}
	}
	log.Debug("00", "a0", Accs[0].PrivateKey, "a1", Accs[1].PrivateKey, "u0", UtxoAccs[0].Sks, "u1", UtxoAccs[0].Sks)
}

func balancesChecker(t *testing.T, beforeBalanceIn, afterBalanceIn, beforeBalanceOut, afterBalanceOut []*big.Int, expectAmount, expectFee, actualFee *big.Int) {
	fmt.Println(beforeBalanceIn, afterBalanceIn, beforeBalanceOut, afterBalanceOut, expectAmount, expectFee, actualFee)
	// Sanity Check
	ins := len(beforeBalanceIn)
	for _, list := range [][]*big.Int{beforeBalanceIn, afterBalanceIn} {
		assert.Equal(t, ins, len(list))
	}
	outs := len(beforeBalanceOut)
	for _, list := range [][]*big.Int{beforeBalanceOut, afterBalanceOut} {
		assert.Equal(t, outs, len(list))
	}
	// Legality Check
	amountlist := make([]*big.Int, 0)
	for _, list := range [][]*big.Int{beforeBalanceIn, afterBalanceIn, beforeBalanceOut, afterBalanceOut} {
		amountlist = append(amountlist, list...)
	}
	for _, item := range []*big.Int{expectAmount, expectFee, actualFee} {
		amountlist = append(amountlist, item)
	}
	for _, amount := range amountlist {
		assert.True(t, amount.Sign() >= 0)
	}
	// Equality Check
	//delta(input) = fee + amount
	sumin := big.NewInt(0)
	for i := 0; i < ins; i++ {
		d := big.NewInt(0).Sub(beforeBalanceIn[i], afterBalanceIn[i])
		sumin = big.NewInt(0).Add(sumin, d)
	}
	assert.Equal(t, big.NewInt(0).Add(expectAmount, expectFee), sumin)
	//delta(output) = amount
	sumout := big.NewInt(0)
	for i := 0; i < outs; i++ {
		d := big.NewInt(0).Sub(afterBalanceOut[i], beforeBalanceOut[i])
		sumout = big.NewInt(0).Add(sumout, d)
	}
	assert.Equal(t, expectAmount, sumout)
	//fee=fee
	assert.Equal(t, expectFee, actualFee)
}

func resultChecker(t *testing.T, receipts types.Receipts, utxoOutputs []*types.UTXOOutputData, keyImages []*lktypes.Key, expReceiptsLen, expUtxoOutputsLen, expKeyImageLen int) {
	// Sanity Check
	assert.True(t, expReceiptsLen >= 0)
	assert.True(t, expUtxoOutputsLen >= 0)
	assert.True(t, expKeyImageLen >= 0)
	// Length Check
	assert.Equal(t, expReceiptsLen, len(receipts))
	assert.Equal(t, expUtxoOutputsLen, len(utxoOutputs))
	assert.Equal(t, expKeyImageLen, len(keyImages))
}

func othersChecker(t *testing.T, expnonce []uint64, nonce []uint64) {
	// Sanity Check
	assert.True(t, len(nonce) > 0)
	assert.Equal(t, len(nonce), len(expnonce))
	// Length Check
	for ind, n := range nonce {
		expn := expnonce[ind]
		assert.True(t, n >= 0)
		assert.Equal(t, expn, n)
	}
}

func hashChecker(t *testing.T, receiptHash, stateHash, balanceRecordHash common.Hash, exprecipts, expstate, expbalancerecord string) {

	assert.Equal(t, receiptHash, common.HexToHash(exprecipts))
	assert.Equal(t, stateHash, common.HexToHash(expstate))
	assert.Equal(t, balanceRecordHash, common.HexToHash(expbalancerecord))
}

func genBlock(txs types.Txs) *types.Block {
	block := &types.Block{
		Header: &types.Header{
			Height:     1,
			Coinbase:   common.HexToAddress("0x0000000000000000000000000000000000000000"),
			Time:       uint64(time.Now().Unix()),
			NumTxs:     uint64(len(txs)),
			TotalTxs:   uint64(len(txs)),
			ParentHash: common.EmptyHash,
			GasLimit:   1e19,
		},
		Data: &types.Data{
			Txs: txs,
		},
	}
	return block
}

func getBalance(tx *types.UTXOTransaction, skv, sks lktypes.SecretKey) (amount *big.Int, mask lktypes.Key) {
	amount = big.NewInt(-1) // if no input matched, return -1
	//gen acc & kI
	acc := lktypes.AccountKey{
		Addr: lktypes.AccountAddress{
			SpendPublicKey: lktypes.PublicKey(ringct.ScalarmultBase(lktypes.Key(sks))),
			ViewPublicKey:  lktypes.PublicKey(ringct.ScalarmultBase(lktypes.Key(skv))),
		},
		SpendSKey: sks,
		ViewSKey:  skv,
		SubIdx:    uint64(0),
	}
	address := wallet.AddressToStr(&acc, uint64(0))
	acc.Address = address
	keyi := make(map[lktypes.PublicKey]uint64)
	keyi[acc.Addr.SpendPublicKey] = 0
	// output
	outputID := -1
	outputCnt := len(tx.Outputs)
	for i := 0; i < outputCnt; i++ {
		o := tx.Outputs[i]
		switch ro := o.(type) {
		case *types.UTXOOutput:
			outputID++
			keyMaps := make(map[lktypes.KeyDerivation]lktypes.PublicKey, 0)
			derivationKeys := make([]lktypes.KeyDerivation, 0)
			derivationKey, err := xcrypto.GenerateKeyDerivation(tx.RKey, skv)
			if err != nil {
				log.Error("GenerateKeyDerivation fail", "rkey", tx.RKey, "err", err)
				continue
			}
			derivationKeys = append(derivationKeys, derivationKey)
			keyMaps[derivationKey] = tx.RKey
			if len(tx.AddKeys) > 0 {
				//we use a addinational key for utxo->account proof, maybe cause err here
				for _, addkey := range tx.AddKeys {
					derivationKey, err = xcrypto.GenerateKeyDerivation(addkey, skv)
					if err != nil {
						log.Info("GenerateKeyDerivation fail", "addkey", addkey, "err", err)
						continue
					}
					derivationKeys = append(derivationKeys, derivationKey)
					keyMaps[derivationKey] = addkey
				}
			}
			recIdx := uint64(outputID)
			realDeriKey, _, err := types.IsOutputBelongToAccount(&acc, keyi, ro.OTAddr, derivationKeys, recIdx)
			if err != nil {
				// trivial error for multi output tx
				//log.Info("IsOutputBelongToAccount fail", "ro.OTAddr", ro.OTAddr, "derivationKey", derivationKey, "recIdx", recIdx, "err", err)
				continue
			}
			ecdh := &lktypes.EcdhTuple{
				Mask:   tx.RCTSig.RctSigBase.EcdhInfo[outputID].Mask,
				Amount: tx.RCTSig.RctSigBase.EcdhInfo[outputID].Amount,
			}
			log.Debug("GenerateKeyDerivation", "derivationKey", realDeriKey, "amount", tx.RCTSig.RctSigBase.EcdhInfo[outputID].Amount)
			scalar, err := xcrypto.DerivationToScalar(realDeriKey, outputID)
			if err != nil {
				log.Error("DerivationToScalar fail", "derivationKey", realDeriKey, "outputID", outputID, "err", err)
				continue
			}
			ok := xcrypto.EcdhDecode(ecdh, lktypes.Key(scalar), false)
			if !ok {
				log.Error("EcdhDecode fail", "err", err)
				continue
			}
			amount = big.NewInt(0).Mul(types.Hash2BigInt(ecdh.Amount), big.NewInt(types.UTXO_COMMITMENT_CHANGE_RATE))
			mask = ecdh.Mask
		default:
		}
	}
	return
}

func calExpectAmount(amounts ...*big.Int) *big.Int {
	sum := big.NewInt(0)
	for _, amount := range amounts {
		sum.Add(sum, amount)
	}
	return sum
}

func calExpectFee(fees ...uint64) *big.Int {
	sum := big.NewInt(0)
	for _, fee := range fees {
		feeI := big.NewInt(0).SetUint64(fee)
		sum.Add(sum, feeI)
	}
	sum.Mul(sum, big.NewInt(types.ParGasPrice))
	return sum
}

//*********** Account Based Transactions Test **********
//tx
func TestAccount2Account(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address
	rAdd := Accs[0].Address

	amount1 := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(100))
	fee1 := types.CalNewAmountGas(amount1, types.EverLiankeFee)
	nonce := uint64(0)
	tx1 := types.NewTransaction(nonce, rAdd, amount1, fee1, gasPrice, nil)
	tx1.Sign(types.GlobalSTDSigner, sender)
	nonce++

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{statedb.GetBalance(rAdd)}
	block := genBlock(types.Txs{tx1})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{statedb.GetBalance(rAdd)}

	expectAmount := calExpectAmount(amount1)
	expectFee := calExpectFee(fee1)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{1}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 1, 0, 0)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0x9199391959b690a1684c077b1ddddaf4d3a1393bc14ffcc49981c1a943982c97", "0x7df8d9d4e16d19a0f755ba21710713ac3a1a0d782216e95279a4de54a02083c7", "0x56cef0615cc7e8f6a90ec993ec41fb7e09b29401423ce3130f77f41c5a06ec39")
}

//txc
func TestAccount2Contract(t *testing.T) {
	statedb := newTestState()
	log.Debug("INITSTATE", "sh", statedb.IntermediateRoot(false), "nonce", statedb.GetNonce(Bank[0].Address), "DP", statedb.JSONDumpKV())
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address

	amount1 := big.NewInt(0)
	fee1 := uint64(1494617)
	nonce := uint64(0)
	tx1 := newContractTx(sAdd, fee1, nonce, "../test/token/sol/SimpleToken.bin")
	tx1.Sign(types.GlobalSTDSigner, sender)
	fromAddress, _ := tx1.From()
	tkAdd := crypto.CreateAddress(fromAddress, tx1.Nonce(), tx1.Data())
	nonce++

	bin, err := ioutil.ReadFile("../test/token/sol/SimpleToken.abi")
	if err != nil {
		panic(err)
	}
	cabi, err := abi.JSON(bytes.NewReader(bin))
	if err != nil {
		panic(err)
	}
	var data []byte
	method := "transfertokentest"
	data, err = cabi.Pack(method, big.NewInt(0))
	if err != nil {
		panic(err)
	}

	amount2 := big.NewInt(0)
	fee2 := uint64(31539)
	tx2 := types.NewTransaction(nonce, tkAdd, big.NewInt(0), fee2, gasPrice, data)
	tx2.Sign(types.GlobalSTDSigner, sender)
	nonce++

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{big.NewInt(0)}
	log.Debug("SAVER", "balance", statedb.GetBalance(sAdd))

	block := genBlock(types.Txs{tx1, tx2})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	log.Debug("SAVER", "nonce", statedb.GetNonce(sAdd), "addr", receipts[0].ContractAddress, "codehash", statedb.GetCodeHash(receipts[0].ContractAddress))
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{big.NewInt(0)}

	expectAmount := calExpectAmount(amount1, amount2)
	expectFee := calExpectFee(fee1, fee2)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 2, 0, 0)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0x3fc4b8ef05b48115243598e0e033cdd4015236e171e586538002ae49e1fcd39d", "0x9acc0b0594de9a1d9cba3923e77ba511d7b44bc563c33937ec7a09579657d562", "0xb92e44ca35a317daadcc36dc6f02e0e44f812f4b9361dee5dee7a794c47a9be1")
}

//txc2(to Contract & Value transfer)
func TestAccount2Contract2(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address

	amount1 := big.NewInt(0)
	fee1 := uint64(107369)
	nonce := uint64(0)
	tx1 := newContractTx(sAdd, fee1, nonce, "../test/token/sol/t.bin")
	tx1.Sign(types.GlobalSTDSigner, sender)
	fromAddress, _ := tx1.From()
	tkAdd := crypto.CreateAddress(fromAddress, tx1.Nonce(), tx1.Data())
	nonce++

	bin, err := ioutil.ReadFile("../test/token/sol/t.abi")
	if err != nil {
		panic(err)
	}
	cabi, err := abi.JSON(bytes.NewReader(bin))
	if err != nil {
		panic(err)
	}
	var data []byte
	method := "set"
	data, err = cabi.Pack(method, big.NewInt(0))
	if err != nil {
		panic(err)
	}

	amount2 := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(100))
	fee2 := types.CalNewAmountGas(amount2, types.EverContractLiankeFee) + uint64(26596)
	tx2 := types.NewTransaction(nonce, tkAdd, amount2, fee2, gasPrice, data)
	tx2.Sign(types.GlobalSTDSigner, sender)
	nonce++

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{big.NewInt(0)}
	block := genBlock(types.Txs{tx1, tx2})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{statedb.GetBalance(tkAdd)}

	expectAmount := calExpectAmount(amount1, amount2)
	expectFee := calExpectFee(fee1, fee2)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 2, 0, 0)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0x864e11778563efdfc0be449b1b05fec1c65ca693e8798cbb1f0e9d5c3cb78052", "0x3ece9a6f16f1b26405ff10573cf89d4c2e11b49c4f3fb68362fe572b04ea2e9e", "0xbdac490b5fb39d6d64cf4faddec6319267edf112b1f74917d07e796bd6edfcbd")
}

//txc3(to Contract & Value transfer but fail)
func TestAccount2ContractVmerr(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address

	amount1 := big.NewInt(0)
	fee1 := uint64(107369)
	nonce := uint64(0)
	tx1 := newContractTx(sAdd, fee1, nonce, "../test/token/sol/t.bin")
	tx1.Sign(types.GlobalSTDSigner, sender)
	fromAddress, _ := tx1.From()
	tkAdd := crypto.CreateAddress(fromAddress, tx1.Nonce(), tx1.Data())
	nonce++

	bin, err := ioutil.ReadFile("../test/token/sol/t.abi")
	if err != nil {
		panic(err)
	}
	cabi, err := abi.JSON(bytes.NewReader(bin))
	if err != nil {
		panic(err)
	}
	var data []byte
	method := "set"
	data, err = cabi.Pack(method, big.NewInt(0))
	if err != nil {
		panic(err)
	}

	amount2 := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(100))
	fee2bf := types.CalNewAmountGas(amount2, types.EverContractLiankeFee) + uint64(26595)
	fee2 := uint64(26595)
	tx2 := types.NewTransaction(nonce, tkAdd, amount2, fee2bf, gasPrice, data)
	tx2.Sign(types.GlobalSTDSigner, sender)
	nonce++

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{big.NewInt(0)}
	block := genBlock(types.Txs{tx1, tx2})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{statedb.GetBalance(tkAdd)}

	expectAmount := calExpectAmount(amount1)
	expectFee := calExpectFee(fee1, fee2)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 2, 0, 0)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0xb3af327405f4da8478cce782e22876366c73b15808d7de6fcfed2a0a0b0e626f", "0x22e4d86a7a179a3e36666c39883e77ac7d65af94d6373c6c03372c795c447197", "0x135c73fc9bdc58b9f72829982914537df45f76b54c2fa747ae45e9d3d7db35bc")
}

//txc4(to Contract & Value transfer but fail due to transfer gas err)
func TestAccount2ContractVmerr2(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address

	amount1 := big.NewInt(0)
	fee1 := uint64(107369)
	nonce := uint64(0)
	tx1 := newContractTx(sAdd, fee1, nonce, "../test/token/sol/t.bin")
	tx1.Sign(types.GlobalSTDSigner, sender)
	fromAddress, _ := tx1.From()
	tkAdd := crypto.CreateAddress(fromAddress, tx1.Nonce(), tx1.Data())
	nonce++

	bin, err := ioutil.ReadFile("../test/token/sol/t.abi")
	if err != nil {
		panic(err)
	}
	cabi, err := abi.JSON(bytes.NewReader(bin))
	if err != nil {
		panic(err)
	}
	var data []byte
	method := "set"
	data, err = cabi.Pack(method, big.NewInt(0))
	if err != nil {
		panic(err)
	}

	amount2 := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(100))
	fee2bf := types.CalNewAmountGas(amount2, types.EverContractLiankeFee) + uint64(21399)
	fee2 := uint64(21400)
	tx2 := types.NewTransaction(nonce, tkAdd, amount2, fee2bf, gasPrice, data)
	tx2.Sign(types.GlobalSTDSigner, sender)
	nonce++

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{big.NewInt(0)}
	block := genBlock(types.Txs{tx1, tx2})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{statedb.GetBalance(tkAdd)}

	expectAmount := calExpectAmount(amount1)
	expectFee := calExpectFee(fee1, fee2)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 2, 0, 0)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0xb5ca9210543551de790b7620ab33f58c98d779793fc531361c4044dbbf64728f", "0xf46776e7e232805fae4534d650e11cbcecf48c3df7a7b5cf663c9c66e272e213", "0x2e04cbf22bae78de96f82727b6df45c947743742a3e5f75df6903962b3653d88")
}

//txt
func TestAccount2AccountToken(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address
	rAdd := Accs[0].Address

	amount1 := big.NewInt(0)
	fee1 := uint64(1494617)
	nonce := uint64(0)
	tx1 := newContractTx(sAdd, fee1, nonce, "../test/token/sol/SimpleToken.bin")
	tx1.Sign(types.GlobalSTDSigner, sender)
	nonce++
	fromAddress, _ := tx1.From()
	tkAdd := crypto.CreateAddress(fromAddress, tx1.Nonce(), tx1.Data())

	amount2 := big.NewInt(0)
	fee2 := types.CalNewAmountGas(amount2, types.EverLiankeFee)
	tkamount := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(100))
	tx2 := types.NewTokenTransaction(tkAdd, 1, rAdd, tkamount, fee2, gasPrice, nil)
	tx2.Sign(types.GlobalSTDSigner, sender)
	nonce++

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{statedb.GetBalance(rAdd)}
	bftkBalanceIn := []*big.Int{statedb.GetTokenBalance(sAdd, tkAdd)}
	bftkBalanceOut := []*big.Int{statedb.GetTokenBalance(rAdd, tkAdd)}
	block := genBlock(types.Txs{tx1, tx2})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{statedb.GetBalance(rAdd)}
	aftkBalanceIn := []*big.Int{statedb.GetTokenBalance(sAdd, tkAdd)}
	aftkBalanceOut := []*big.Int{statedb.GetTokenBalance(rAdd, tkAdd)}

	expectAmount := calExpectAmount(amount1, amount2)
	expecttkAmount := calExpectAmount(tkamount)
	expectFee := calExpectFee(fee1, fee2)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	balancesChecker(t, bftkBalanceIn, aftkBalanceIn, bftkBalanceOut, aftkBalanceOut, expecttkAmount, big.NewInt(0), big.NewInt(0))
	resultChecker(t, receipts, utxoOutputs, keyImages, 2, 0, 0)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0x9835cf7e62ae08fb00d725b3b07b427140c8664d6066eac36e3d01aa53ba0693", "0x8f471a6cad341c53425ec81e267b070a237388428e61f9355864cb8e8b67c238", "0xd6e243526bd1c7d6f726f5ee7098b39caf475b90b7e7caed3a315dbdf6437fe8")
}

//cct
func TestContractCreation(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address
	nonce := uint64(0)
	amount1 := big.NewInt(0)
	fee1 := uint64(1494617)
	tx := newContractTx(accounts[0].Address, fee1, nonce, "../test/token/sol/SimpleToken.bin")
	tx.Sign(types.GlobalSTDSigner, sender)
	nonce++

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{}
	block := genBlock(types.Txs{tx})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{}

	expectAmount := calExpectAmount(amount1)
	expectFee := calExpectFee(fee1)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 1, 0, 0)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0xf024b92d0f56ea2570b9e9aea10f84056d4fd2f2fda8fd32982cc7fe959c83dd", "0x7938c0befd59f2279354c97cd553e43bf78419b1e4514816139d250e3777d202", "0x3cb350f960cb54c88aac6c8b9d2cda0ff4989c47d9eafca6f30bc584d331f0c2")
}

//cct2
func TestContractCreation2(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address
	nonce := uint64(0)
	amount1 := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(1e16))
	fee1 := uint64(799135)
	tx1 := newContractTx2(accounts[0].Address, fee1, nonce, "../test/token/sol/a.bin", amount1)
	tx1.Sign(types.GlobalSTDSigner, sender)
	nonce++
	fromAddress, _ := tx1.From()
	tkAdd := crypto.CreateAddress(fromAddress, tx1.Nonce(), tx1.Data())
	//log.Debug("tx", "tx", tx1)

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{big.NewInt(0)}
	block := genBlock(types.Txs{tx1})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{statedb.GetBalance(tkAdd)}
	log.Debug("add", "tkadd", tkAdd, "acadd", receipts[0].ContractAddress, "receipt", receipts[0])

	expectAmount := calExpectAmount(amount1)
	expectFee := calExpectFee(fee1)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 1, 0, 0)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0xeabae586564b1d338523ddbb528403a781dc9096313b8acff72dad66d2f1addb", "0xf6ba8eb94a3c46556ef80f30d853d0b9e52985bffd39ec887855e58f268831e2", "0xfecf95ac258291a191335ba2ac3eb8a5bf1ca235ab7e82386ff1bce1e4fb12c4")
}

/* Not Support
cctbytx
func TestContractCreationBySendToEmptyAddress(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address

	var ccode []byte
	bin, err := ioutil.ReadFile("../test/token/sol/SimpleToken.bin")
	if err != nil {
		panic(err)
	}
	ccode = common.Hex2Bytes(string(bin))

	amount1 := big.NewInt(0)
	fee1 := uint64(1494617)
	nonce := uint64(0)
	tx := types.NewContractCreation(nonce, amount1, fee1, gasPrice, ccode)
	tx.Sign(types.GlobalSTDSigner, sender)
	tkAdd := crypto.CreateAddress(sAdd, tx.Nonce(), tx.Data())
	nonce++

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{big.NewInt(0)}
	block := genBlock(types.Txs{tx})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{statedb.GetBalance(tkAdd)}

	expectAmount := calExpectAmount(amount1)
	expectFee := calExpectFee(fee1)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 1, 0, 0)
	othersChecker(t, expectNonce, actualNonce)
}
*/

//cut
func TestContractUpdate(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address

	amount1 := big.NewInt(0)
	fee1 := uint64(1995447)
	nonce := uint64(0)
	tx1 := newContractTx(accounts[0].Address, fee1, nonce, "../test/token/tcvm/TestToken.bin")
	tx1.Sign(types.GlobalSTDSigner, sender)
	fromAddress, _ := tx1.From()
	contractAddr := crypto.CreateAddress(fromAddress, tx1.Nonce(), tx1.Data())
	nonce++

	amount2 := big.NewInt(0)
	fee2 := uint64(0)
	tx2 := genContractUpgradeTx(fromAddress, contractAddr, nonce, "../test/token/tcvm/TestToken.bin")
	tx2.Sign(types.GlobalSTDSigner, sender)
	nonce++

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{}
	block := genBlock(types.Txs{tx1, tx2})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{}

	expectAmount := calExpectAmount(amount1, amount2)
	expectFee := calExpectFee(fee1, fee2)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 2, 0, 0)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0xba3b228862af7d9e1efaedfc16169f7b116d13b9f1c0b6095bf239bfd0349dc4", "0x6154d4374568c16e723f87ec0856a3706e9f5986e2ce42ef7a9cfabe1bd2a24d", "0xcd1f574e804d049399b17a069ea3f59d509df1363381944db53b012a4f10af9b")
}

//*********** UTXO Based Transactions Test **********

/* Not Support
//A->A
func TestSingleAccount2SingleAccount(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address
	rAdd := Accs[0].Address

	amount1bf := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(100))
	fee1 := types.CalNewAmountGas(amount1bf, types.EverLiankeFee)
	fee1i := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee1), big.NewInt(types.ParGasPrice))
	amount1 = big.NewInt(0).Sub(amount1bf, fee1i)
	nonce := uint64(0)
	ain := types.AccountSourceEntry{
		From:   sAdd,
		Nonce:  nonce,
		Amount: amount1bf,
	}
	aout := types.AccountDestEntry{
		To:     rAdd,
		Amount: amount1,
		Data:   nil,
	}
	tx, _, err := types.NewAinTransaction(&ain, []types.DestEntry{&aout}, common.EmptyAddress, nil)
	if err != nil {
		panic(err)
	}
	tx.Sign(types.GlobalSTDSigner, sender)
	nonce++

	log.Debug("TXENC","TX",hex.EncodeToString(ser.MustEncodeToBytes(tx)))
	tx =genUTXOTransaction("f90139f856cf4852c16c3232f84d8089056bc75e2d63100000a00000000000000000000000000000000000000000000000000000000000000000a085598d81489afb5283fddcfb9cddb6a81b85ae91cbec9f75fb036238f5072589f84ac77cca059e4aa7f841945033532c906ef2a0d8fb25dda97559212312d823890564d702d38f5e000080a00f223bdf27f978296c3516b93031b18c6e8cf38a8aa1249d2f3bc474f0344a89940000000000000000000000000000000000000000a00ec60868c995feef4dbb392b97827fb9efa86e44fc984bd018f6fae53e47f3d5c08806f05b59d3b2000080f84582e3e5a0b7a53aae5b671a036d10fd2394b1cf4e8df81c642f88c511581bbc79d18b87bba04635704ab3f230a7772e0359665c5f4b8934b572d589d06838b93df0a11f65fcccc580c0c0c080c5c0c0c0c0c0")

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{statedb.GetBalance(rAdd)}
	block := genBlock(types.Txs{tx})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{statedb.GetBalance(rAdd)}

	expectAmount := calExpectAmount(amount1)
	expectFee := calExpectFee(fee1)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 1, 0, 0)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())

	for _, br := range types.BlockBalanceRecordsInstance.TxRecords {
		log.Debug("BR","h",types.RlpHash(br),"br",br)
	}
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0xce2c1556aa22916a1f567798f0fc891ce64a96ae57d6fac3f46ce96a90f572b5", "0x20b162604aa0bb2403994f21b234f8e5b8cc6fe721b94ae8cf785621b750e90f", "0xfc7211151f6979ad63f537c3c5b61f2416e50004c37a1e8fc567de3587eb92c9")
}
*/

/* Not Support
//A->C
func TestSingleAccount2Contract(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address

	amount1 := big.NewInt(0)
	fee1 := uint64(0)
	nonce := uint64(0)
	tx1 := newContractTx(sAdd, 1000000, nonce, "../test/token/sol/SimpleToken.bin")
	tx1.Sign(types.GlobalSTDSigner, sender)
	tkAdd := crypto.CreateAddress(tx1.FromAddr, tx1.Nonce(), tx1.Data())
	nonce++

	bin, err := ioutil.ReadFile("../test/token/sol/SimpleToken.abi")
	if err != nil {
		panic(err)
	}
	cabi, err := abi.JSON(bytes.NewReader(bin))
	if err != nil {
		panic(err)
	}
	var data []byte
	method := "transfertokentest"
	data, err = cabi.Pack(method, big.NewInt(0))
	if err != nil {
		panic(err)
	}

	fee2 := uint64(31539)
	fee2i := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee2), big.NewInt(types.ParGasPrice))
	amount2 := big.NewInt(0)
	ain := types.AccountSourceEntry{
		From:   sAdd,
		Nonce:  1,
		Amount: fee2i,
	}
	aout := &types.AccountDestEntry{
		To:     tkAdd,
		Amount: big.NewInt(0),
		Data:   data,
	}
	tx2, _, err := types.NewAinTransaction(&ain, []types.DestEntry{aout}, common.EmptyAddress, nil)
	if err != nil {
		panic(err)
	}
	tx2.Sign(types.GlobalSTDSigner, sender)
	nonce++

	//log.Debug("TXENC","TX2",hex.EncodeToString(ser.MustEncodeToBytes(tx2)))

	tx2 = genUTXOTransaction("f90151f854cf4852c16c3232f84b01870b347491283800a00000000000000000000000000000000000000000000000000000000000000000a01cbf280ce5b51845873c886cc0193a2f36d01024ec355c4894f3bbc77a51091df865c77cca059e4aa7f85c94c7c22a8e08d3b0643a55e7c087a416171b45922f80a459abe2bf0000000000000000000000000000000000000000000000000000000000000000a00100000000000000000000000000000000000000000000000000000000000000940000000000000000000000000000000000000000a0397fcf8ec85576cb471c0d51e327d396f2f92458451acf0ccd37ed58293a8285c0870b34749128380080f84582e3e6a0ebf25ff04239af5657eb67fcb079f942d672385e1bd5686ecd28ae6d59b3c2dba029b0f9f1e1b36026942f98e5144af74697fde77c52c2818b3ca22e14985d09baccc580c0c0c080c5c0c0c0c0c0")
	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{big.NewInt(0)}
	block := genBlock(types.Txs{tx1, tx2})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{big.NewInt(0)}

	expectAmount := calExpectAmount(amount1, amount2)
	expectFee := calExpectFee(fee1, fee2)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 2, 0, 0)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0x3b0eb21c2226494244a3ae1095faad378eba594c32bdb48ff733cb43bab7d247", "0xd9e63cf2b06c275b643581541f002ea28c4057b3c9dafa66d23c6e9935d8b5f7", "0xea8e5d15009bd2b1d0cb648841fce4eedcfd20d22926adef9655921fad3117b2")
}
*/

//A->U+
func TestSingleAccount2MulitipleUTXO(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address
	utxo1, utxo2 := UtxoAccs[0], UtxoAccs[1]

	amount1bf := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(100))
	nonce := uint64(0)
	fee1 := types.CalNewAmountGas(amount1bf, types.EverLiankeFee)
	fee1i := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee1), big.NewInt(types.ParGasPrice))
	amount1 = big.NewInt(0).Sub(amount1bf, fee1i)
	amount1a := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(50))
	amount1b := big.NewInt(0).Sub(amount1, amount1a)
	ain := types.AccountSourceEntry{
		From:   sAdd,
		Nonce:  nonce,
		Amount: amount1bf,
	}
	uout1 := types.UTXODestEntry{
		Addr:   utxo1.Addr,
		Amount: amount1a,
	}
	uout2 := types.UTXODestEntry{
		Addr:   utxo2.Addr,
		Amount: amount1b,
	}
	tx1, _, err := types.NewAinTransaction(&ain, []types.DestEntry{&uout1, &uout2}, common.EmptyAddress, nil)
	if err != nil {
		panic(err)
	}
	tx1.Sign(types.GlobalSTDSigner, sender)
	nonce++

	log.Debug("TXENC", "TX", hex.EncodeToString(ser.MustEncodeToBytes(tx1)))
	tx1 = genUTXOTransaction("f90624f856cf4852c16c3232f84d8089056bc75e2d63100000a064c93b76e172c25e51d2ed65f88b6b992a8b05154613f971ff075430d15d5a0fa0e628956cf7cc1e599636dd03bee06553ec4bd287d669f17f2332af9ef5a5e78af8985842699137a517f843a0baef31faceed6c772ebe8baf2563f8952d64b21a0b898a09dad97c55cc2baa1580a0bb56a26a95e365cb763ce337bcda1c9aff572e7e09b17ae9701cfac309d23abd5842699137a517f843a0ecf640c653da3173bcf298038a22d5e9cdb8ed1bce0dcac430acfd721b65a61280a0c8bdc7d05ebde7756b9f59952fce32bb77a862590b91025ff30b294482711860940000000000000000000000000000000000000000a07faa822b33122468e417cb13e2b5c700250825a45c023a28add087f35052a508f842a0a9269270b98c69fe2484126f6526ea52ccbafb1557edbdeb8a4081f13c22dd65a038e765c8588dcbbd9809fc9137015c0c5bfa43b53f5aadeecefca47a0e46ee4a8806f05b59d3b2000080f84582e3e6a077ccee67836e37e5071821853cf2139f1174dec4e6c46f4b0cb0b7bae8a75547a04fb257413b5cc281ef3a763cf0139759b507b17b045ab13bc40feb7304c479d5f90464f9015980c0f8caf863a08a6eb8acba266d25c5b7906e46362afde4775b1523a95c7d9b0472b5cad47e0fa0c81df5adde80e84cf0d1af889926ae4c176aa8954d54a1aaddd24d3d72711407a00000000000000000000000000000000000000000000000000000000000000000f863a076082246cfcae1a7fa63e29a76e6c24c2dfdc909727724be1ebff6a3e037420ca0e44eb3428fc8e3ac379cb8ddf77047e4b49e64ea9545c66e642a396880c1a80ca00000000000000000000000000000000000000000000000000000000000000000f888f842a00000000000000000000000000000000000000000000000000000000000000000a01044b65650fbf9379ca026c16516e65fa15afbafb211a54f093151467a48263af842a00000000000000000000000000000000000000000000000000000000000000000a08cd9f05f5d51d3a495f850572c102ed329d225b74b7a8bd0694ff582025842fe80f90305c0f902fef902fba0acb66b91fef3405baebe1c6f65b27036802bf74d080a7107fd6d36a6faf1d985a04ca2029652ca0835b296ae270208bc864f1eaba01d18d414756909caa3b90cb9a0c277e48fbead6d727453a341bf5fa25f70e464a95e01046577b85ef2aa96c9bca03f1095aec10a2c4ade6530d8a0d460b5376969708c4d2673041403c1c2cfcbd8a01d4c8d490dc30227e015b71442e0329a42f86f0ad030125a6e7493a1b5f87c07a029ca5011d143707f82a6402966d854d2074c83456ec4b3599f7bbd21dfd1780ff8e7a0658632e1bc6a36e253e4448aa5daf2f9264dc6a6b77a74836fb5ace33b484e9da048b1d070d00a582a08b1821dbda323e3e15d76013eefb5381d5c970165c0e315a0efef28d2d4b67cd87c0b04e5a67419cb341ed89faa3086c07942ee7c7281b9a4a08762c1540f5f85e6f9ce3aa3cef917aec87d27d8d57082565d317e73a5a8a813a0ad40a4bbe77a575d939b4b3759aeffc3ee70b7f943fa606bad1c3eeeab360b01a036d4f2a13e78c76c58e931eccc065c55e6eff22f801695d56415b62f1bc7bbcca054099f92ae63dd2a3237fbe247f01675101632d933499db72ccffd7f487da469f8e7a08814dab33a867dd1b2b92b92a29fd5daff0ed8783f263609fef05cc19bf93145a0bfcf9c1fcb2d4a2344115b9fd1eddef070059f75b7ac9add32dd2b787d2843d2a0570dd4813895218e3d477db9a3bde5fc1b26f2fdb91d8b190f70e3a97ce1f9eea08bbf8162dc849b8ebae4f02deb0012d2a5ace02d401c43df0df5e41ad8c55113a09c632caa13e249eac6408b9f68937b625b69632a0212e41006d80bc32e120b26a05e00624e71bc999d03fc2dc9949fd69a687b608af0ee1c4739a53d5a48f7f5d9a03666d077eee34d9ee06a8809ac630499728d15b1c1f084cfcdfb92934a526e67a0568aa6b462c9c31c5ab05d822afdb104949a399b5434878068ed47d723c7500da00e7beb42addbdd8224fa37719604f655b03d237050df0e8fff141bef54026c08a0340babcfad0339d589a6651681f3bb258585d038a5add4e5bd5a78018f11f806c0c0c0")

	balance11, _ := getBalance(tx1, lktypes.SecretKey(utxo1.Skv), lktypes.SecretKey(utxo1.Sks))
	balance12, _ := getBalance(tx1, lktypes.SecretKey(utxo2.Skv), lktypes.SecretKey(utxo2.Sks))

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{big.NewInt(0), big.NewInt(0)}
	block := genBlock(types.Txs{tx1})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{balance11, balance12}

	expectAmount := calExpectAmount(amount1)
	expectFee := calExpectFee(fee1)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 1, 2, 0)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0xd23f858ddae95dee46e20542e5cb81f52cc8c03ee51424eabf745cf562bb06b5", "0x3a80d4707631a4827f7fb4f1c9c19f346a42fb02de9eea46f55896b312215d32", "0x3bcd1702f8386a7cb40ab37475f3522742ebda85b699edb24ae6c2d13199f075")
}

/* Not Support
//A->U+token
func TestSingleAccount2MulitipleUTXOToken(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address
	utxo1, utxo2 := UtxoAccs[0], UtxoAccs[1]

	amount1 := big.NewInt(0)
	fee1 := uint64(0)
	nonce := uint64(0)
	tx1 := newContractTx(sAdd, 100000, nonce, "../test/token/sol/SimpleToken.bin")
	tx1.Sign(types.GlobalSTDSigner, sender)
	tkAdd := crypto.CreateAddress(tx1.FromAddr, tx1.Nonce(), tx1.Data())
	nonce++

	amount2 := big.NewInt(0)
	tkamount2 := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(1))
	fee2 := types.CalNewAmountGas(big.NewInt(0), types.EverLiankeFee)
	//fee2i := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee2), big.NewInt(types.ParGasPrice))
	tkamount2a := big.NewInt(0).Mul(big.NewInt(1e17), big.NewInt(5))
	tkamount2b := big.NewInt(0).Sub(tkamount2, tkamount2a)
	ain := types.AccountSourceEntry{
		From:   sAdd,
		Nonce:  nonce,
		Amount: tkamount2,
	}
	uout1 := types.UTXODestEntry{
		Addr:   utxo1.Addr,
		Amount: tkamount2a,
	}
	uout2 := types.UTXODestEntry{
		Addr:   utxo2.Addr,
		Amount: tkamount2b,
	}
	tx2, _, err := types.NewAinTransaction(&ain, []types.DestEntry{&uout1, &uout2}, tkAdd, nil)
	if err != nil {
		panic(err)
	}
	tx2.Sign(types.GlobalSTDSigner, sender)
	nonce++
	balance21, _ := getBalance(tx2, lktypes.SecretKey(utxo1.Skv), lktypes.SecretKey(utxo1.Sks))
	balance22, _ := getBalance(tx2, lktypes.SecretKey(utxo2.Skv), lktypes.SecretKey(utxo2.Sks))

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{big.NewInt(0), big.NewInt(0)}
	bftkBalanceIn := []*big.Int{statedb.GetTokenBalance(sAdd, tkAdd)}
	bftkBalanceOut := []*big.Int{big.NewInt(0), big.NewInt(0)}
	block := genBlock(types.Txs{tx1, tx2})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{big.NewInt(0), big.NewInt(0)}
	aftkBalanceIn := []*big.Int{statedb.GetTokenBalance(sAdd, tkAdd)}
	aftkBalanceOut := []*big.Int{balance21, balance22}

	expectAmount := calExpectAmount(amount1, amount2)
	expecttkAmount := calExpectAmount(tkamount2)
	expectFee := calExpectFee(fee1, fee2)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	balancesChecker(t, bftkBalanceIn, aftkBalanceIn, bftkBalanceOut, aftkBalanceOut, expecttkAmount, big.NewInt(0), big.NewInt(0))
	resultChecker(t, receipts, utxoOutputs, keyImages, 2, 2, 0)
	othersChecker(t, expectNonce, actualNonce)

}
*/

//U->A
func TestSingleUTXO2Account(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address
	utxo1, utxo2 := UtxoAccs[0], UtxoAccs[1]

	amount1bf := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(100))
	nonce := uint64(0)
	fee1 := types.CalNewAmountGas(amount1bf, types.EverLiankeFee)
	fee1i := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee1), big.NewInt(types.ParGasPrice))
	amount1 = big.NewInt(0).Sub(amount1bf, fee1i)
	amount1a := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(50))
	amount1b := big.NewInt(0).Sub(amount1, amount1a)
	ain := types.AccountSourceEntry{
		From:   sAdd,
		Nonce:  nonce,
		Amount: amount1bf,
	}
	uout1 := types.UTXODestEntry{
		Addr:   utxo1.Addr,
		Amount: amount1a,
	}
	uout2 := types.UTXODestEntry{
		Addr:   utxo2.Addr,
		Amount: amount1b,
	}
	tx1, _, err := types.NewAinTransaction(&ain, []types.DestEntry{&uout1, &uout2}, common.EmptyAddress, nil)
	if err != nil {
		panic(err)
	}
	tx1.Sign(types.GlobalSTDSigner, sender)
	nonce++
	balance11, mask11 := getBalance(tx1, lktypes.SecretKey(utxo1.Skv), lktypes.SecretKey(utxo1.Sks))
	balance12, _ := getBalance(tx1, lktypes.SecretKey(utxo2.Skv), lktypes.SecretKey(utxo2.Sks))

	amount2bf := amount1a
	fee2 := types.CalNewAmountGas(amount2bf, types.EverLiankeFee)
	fee2i := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee2), big.NewInt(types.ParGasPrice))
	amount2 := big.NewInt(0).Sub(amount2bf, fee2i)
	sEntey1 := &types.UTXOSourceEntry{
		Ring: []types.UTXORingEntry{types.UTXORingEntry{
			Index:  0,
			OTAddr: tx1.Outputs[0].(*types.UTXOOutput).OTAddr,
		}},
		RingIndex: 0,
		RKey:      tx1.RKey,
		OutIndex:  0,
		Amount:    balance11,
		Mask:      mask11,
	}
	aDest := &types.AccountDestEntry{
		To:     sAdd,
		Amount: amount2,
	}
	tx2, ie, mk, _, err := types.NewUinTransaction(&utxo1.Acc, utxo1.Keyi, []*types.UTXOSourceEntry{sEntey1}, []types.DestEntry{aDest}, common.EmptyAddress, common.EmptyAddress, []byte{})
	if err != nil {
		panic(err)
	}
	err = types.UInTransWithRctSig(tx2, []*types.UTXOSourceEntry{sEntey1}, ie, []types.DestEntry{aDest}, mk)
	if err != nil {
		panic(err)
	}

	//log.Debug("TXENC","TX1",hex.EncodeToString(ser.MustEncodeToBytes(tx1)))
	//log.Debug("TXENC","TX2",hex.EncodeToString(ser.MustEncodeToBytes(tx2)))
	tx1 = genUTXOTransaction("f90624f856cf4852c16c3232f84d8089056bc75e2d63100000a0a3d8a86fa1ad455cb600c3831c86a96622f32a46754758ceac7c6f3894b42e06a04a9ce3d0d3f5b5570d342491478ac5bc7f4ccb4d69353c07c598dcb87a21e1b9f8985842699137a517f843a0ce6c3c6c4304233362147119bffd94c7c72712c60aedcd7909001b86e07487ee80a09231fa21a07f2399f70ede8f4f7dac7bc25e4012fce978dc47b173bc817915aa5842699137a517f843a0d57a6ca5c51a4c8663da36cb75f52240844c4d2641a06f0f92657687806c723280a082a19b93b5cce0dbd079ca4b9e1e752365925888508e9961ff7735a4991208a0940000000000000000000000000000000000000000a035946bc4e7058cda3c8c80053c2d99bfb53a82db8256b1c925a5f7966963459ff842a0d3b66cca17702105cac92e0a0e50563091e6088ca4258db30609300f6c0f9d44a07379dc13b0225aa83b138aba1e60303184a5e06c95101a56abd18f6f05935cc98806f05b59d3b2000080f84582e3e5a00ca0ad1e3a0167ee14c3e0a6048fdd23ed6b25a9adc4290f8439df493364ace9a00abeaf0dc08f85ba5ab0a0522a08723754ffe646b4be53770ef7af6df38c44a6f90464f9015980c0f8caf863a058919a4534d832e36c88212e1c16f6259c28d24aebe7602715c571259b5f020da054e508b87b235df1a44d9a1466984f3251657c1d7c58e6799644754b184c150aa00000000000000000000000000000000000000000000000000000000000000000f863a0c59ed6cd9456d1426296573d59cd8e4e0fda76050ba5ad1071863bb9c0b6e40aa07cabaf46ac8840b43f095c591a2be7ba899c88456def67bccbc014cddec8c301a00000000000000000000000000000000000000000000000000000000000000000f888f842a00000000000000000000000000000000000000000000000000000000000000000a0b083d65e016112ae4dc33ff2e626aa55286364a0e34750e1fdaa1c6be0a8afcdf842a00000000000000000000000000000000000000000000000000000000000000000a05f9355dc46515da8ac30dc01aae2c7aad24cf5e59de855d2a1c6ed43e5b80a5680f90305c0f902fef902fba007e1a6a64161a05aa382fced7e6bd584b18b059181f4d13cacf3b809e32ed3d3a0aa553d87ad40681b53f7d78d6ff0bfec6255f72cee84f0267e7c9e4ec5743bc9a0e656795665b40eaf6f9ef9eed1434743293bdec2da8e91f6ec82f90fa97ec75fa0848f6499df056093083555673ecaf50533cf3057daba956431dba8c0a3261f2ea093f5e435580af8c40f53e3899c98795a739a67b0874b2ae822507fe0e092d908a02b9df08c16782d655593064331952780d3a2e99113eeaf9471bf7e2fba2bb805f8e7a0cb85c431bdfa1b1ec4c407f3a495ca99fb03619b6e0e2f5d2cd5130a9e458343a0f85bcec86ee3c1ff8fb51d1414874e33c4c0d8fd298e3564e38faef72f0241dfa00f26dcaeaca283344ecc826276ffd6fdad6b156e9fbe341e521976f9dfa4e36ea0e89a8cdeb9b7dce446a880475852b96cd1356a65ca3661e472901c570c2baab5a03b84c9c7f5d2353632716dc155e84e6fb0eb40f0cfa501f5b5e5a93aea56cebda0a4d4ecb7e0f0df7cc3cff2386c5b4467121e54d23f1c75ccaaca66223c35608da0b224f9999d767a178af846b2001ccce2eb8a6ab83804097aef7b5e7a6cd1bd9bf8e7a098649fe5e545ef7c9c7b1e67c374987dfd61a4c19f7276e7d0aa9edd23a8a566a0ff45a650dba51814cd1717595ae1d69809702d7d1a595cd48b99a59b17a76651a0ab48b857dc91ade50d37c4baba8c8e576f3c33279d9d91103c6f4261c6dd47c7a0e44be1a74345b01178149134a3f2cbc705b7f7aa005a3eb4ae0117d23aeb67c3a0080448836bab3234035538d46ebc4360a4072e3fb98c1bd44ad38583e1f1e716a0bc21f87314d09864835b667f01a22cd595afdc4ade942cf3494b77b8f704b4eda0e0b1a893b5f9ab56099fa057057548659e54fe77c79ef423c9f5facd25ae625aa053104ee5989dec865c8f5876368549887c83f472ed78441adbd33941dbf1fc00a0ab74024e62fb1910310594191263a5eea20950c1e0e113b40966a53fd101960aa06aa9ccad63bd2ff87e00ce6412a15ee2d9d1cd72928f1bf3babd7d50b7a9a10dc0c0c0")
	tx2 = genUTXOTransaction("f90176eb10c698b1d37308e3c180a0f76f073c93c12ff7c81ae52909f5b4c35cd4f3971c8929f5e33b3d0a94f221a7f84ac77cca059e4aa7f8419454fb1c7d0f011dd63b08f85ed7b518ab820281008902b26b8169c7af000080a0d8949fcfac9adee186adba37a8df12c4b33d403ad29caa8048a7cc49713de930940000000000000000000000000000000000000000a065bf9739340e99b55cefb165a3f7fbd542c374141222bb1a8eff9f4e7d8a3263e1a09205d9ee768a05eb788cd15ac2b583973e344aeea2e9c7ec29da1a5aa6fcdc9a8803782dace9d9000080c3808080f896c503c0c0c080f88ec0c0e3e2c0a00000000000000000000000000000000000000000000000000000000000000000e1a0ab039980de18235e5fd3e89fb5999fbb7f6233e73981ef146d0472930b706831f844f842a0735577dd3486a9489e4a234c1ef7a1c3a746fee18c7c0babf34aff98b252a404a0dff7bd6b6dad1565959184b3d57df2ee9229203106b29b556bef5e1217d64806")

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{big.NewInt(0)}
	block := genBlock(types.Txs{tx1, tx2})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{balance12}

	expectAmount := calExpectAmount(amount1b)
	expectFee := calExpectFee(fee1, fee2)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee) //#1
	resultChecker(t, receipts, utxoOutputs, keyImages, 2, 2, 1)
	othersChecker(t, expectNonce, actualNonce)

	for _, rece := range receipts {
		rece.TxHash = common.EmptyHash
	}

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0x3ee82caac92dba922003ea688c86377d615aa8ad5c5e157d3661a68319556819", "0x49ef6201a2e4059bc3680d424dce0cc093a1bc3abcb55cdc624331435e48a41b", "0xd80dbebf15da6965937bd5ed2b99b7bdc8f080bd66d1b009c2270d870be8e3cd")
}

//U->M
func TestSingleUTXO2Mix(t *testing.T) {
	statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address
	utxo1, utxo2 := UtxoAccs[0], UtxoAccs[1]

	amount1bf := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(100))
	fee1 := types.CalNewAmountGas(amount1bf, types.EverLiankeFee)
	fee1i := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee1), big.NewInt(types.ParGasPrice))
	amount1 := big.NewInt(0).Sub(amount1bf, fee1i)
	amount1a := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(90))
	amount1b := big.NewInt(0).Sub(amount1, amount1a)
	nonce := uint64(0)
	ain := types.AccountSourceEntry{
		From:   sAdd,
		Nonce:  nonce,
		Amount: amount1bf,
	}
	uout1 := types.UTXODestEntry{
		Addr:   utxo1.Addr,
		Amount: amount1a,
	}
	uout2 := types.UTXODestEntry{
		Addr:   utxo2.Addr,
		Amount: amount1b,
	}
	tx1, _, err := types.NewAinTransaction(&ain, []types.DestEntry{&uout1, &uout2}, common.EmptyAddress, nil)
	if err != nil {
		panic(err)
	}
	tx1.Sign(types.GlobalSTDSigner, sender)
	nonce++
	balance11, mask11 := getBalance(tx1, lktypes.SecretKey(utxo1.Skv), lktypes.SecretKey(utxo1.Sks))
	balance12, _ := getBalance(tx1, lktypes.SecretKey(utxo2.Skv), lktypes.SecretKey(utxo2.Sks))

	sEntey1 := &types.UTXOSourceEntry{
		Ring: []types.UTXORingEntry{types.UTXORingEntry{
			Index:  0,
			OTAddr: tx1.Outputs[0].(*types.UTXOOutput).OTAddr,
		}},
		RingIndex: 0,
		RKey:      tx1.RKey,
		OutIndex:  0,
		Amount:    balance11,
		Mask:      mask11,
	}

	amount2bf := amount1a
	fee2 := types.CalNewAmountGas(amount2bf, types.EverLiankeFee) + uint64(5e8)
	fee2i := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee2), big.NewInt(types.ParGasPrice))
	amount2 := big.NewInt(0).Sub(amount2bf, fee2i)
	amount2a := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(10))
	amount2b := big.NewInt(0).Sub(amount2, amount2a)
	aDest := &types.AccountDestEntry{
		To:     sAdd,
		Amount: amount2a,
	}
	uDest := &types.UTXODestEntry{
		Addr:   uout2.Addr,
		Amount: amount2b,
	}
	tx2, ie, mk, _, err := types.NewUinTransaction(&utxo1.Acc, utxo1.Keyi, []*types.UTXOSourceEntry{sEntey1}, []types.DestEntry{aDest, uDest}, common.EmptyAddress, common.EmptyAddress, []byte{})
	if err != nil {
		panic(err)
	}
	err = types.UInTransWithRctSig(tx2, []*types.UTXOSourceEntry{sEntey1}, ie, []types.DestEntry{aDest, uDest}, mk)
	if err != nil {
		panic(err)
	}
	balance22, _ := getBalance(tx2, lktypes.SecretKey(utxo2.Skv), lktypes.SecretKey(utxo2.Sks))

	//log.Debug("TXENC","TX1",hex.EncodeToString(ser.MustEncodeToBytes(tx1)))
	//log.Debug("TXENC","TX2",hex.EncodeToString(ser.MustEncodeToBytes(tx2)))
	tx1 = genUTXOTransaction("f90624f856cf4852c16c3232f84d8089056bc75e2d63100000a006aa3e1262edd4c09476ea730763282ea4ecdbb2dcb85222a805be37ee9e2b04a0efb5ca413adf519ce25263588aea3d7fa3ccc658f4a45e87cddb2759974231faf8985842699137a517f843a0462d04f9b5784c44f1730978b799822a67144132c13726f55e4e90431114446480a055f7002d46cd6e3a6394999ef226ce925346f02c9ce3e1a24f39831b1341ba7f5842699137a517f843a0919f0b80380a77f48a7d31de2c9c2064463e5dce287400b9fc5180d83c533de180a0561a692b3128af8abc8d7bd3e6866c8785d7e234caaa1d49a51311348e918911940000000000000000000000000000000000000000a09e5d1d9e20a28d9c8787090159f7c6abcf25809efc6a2a2ca5ce9ddb4715e2d7f842a05516b012c7abdde514d5ce96f41eecf534ada82eee16f8b802b29482076957a4a0c5ae855a4c6ab7c6463f783b325a6f1709639418ee852693870862232fdaeb3a8806f05b59d3b2000080f84582e3e5a0321a3108defe6003f28edf825fa8d74233e18492f21242457eba5c4b7789c1e5a05803a784c5770e7dbc4a1ee8a657e5d4385808f44a4b3c87c144b62863c91477f90464f9015980c0f8caf863a05734d3c0b952b6841be2b6dd1b14c91b22f269b7cbef38f45bb5f8af43451a0ca0a7f9bf2b290704cc15486a78328711b6d4e8619406ee2d26acbb50053d64a80ca00000000000000000000000000000000000000000000000000000000000000000f863a05fe4bbd57c2a75ad838b218d8d2858454bd629a2863cdbee120b3fc421b7420ba07ebacc7c5ffb6e86f7a83aa039fe4b2c048306c9e5cd84802ce38af0d8eb370da00000000000000000000000000000000000000000000000000000000000000000f888f842a00000000000000000000000000000000000000000000000000000000000000000a0ba47f765121f835edca32c1436880087afce49400d19b54893797a4c113ba605f842a00000000000000000000000000000000000000000000000000000000000000000a05b03d4f65de7db1a0cdf8955ca0d2577a4a6dbf169c219c298d827da82ad979f80f90305c0f902fef902fba0c6119702fdd1ac9220b13df65150e2ae1a200ad939f0810cc7d33d3e9eddefdea0a058650a878214b4e7d91c65e081db4ddd95c2b00633428f8b16c0c0ccadb030a08be9b33597feb73d69ba01e216a80c9c0873d81f17d40557f17c573e93d41367a0454e57facddb9937c1289ce3d659b1bcba4807ab70ec3953b27210d6df13e2d9a023bcb0895ed6b7c220165bfa97ce4088171ee537ce5289bd3910093b8f533e0aa07d198795073422cc06ea3334b0d6f1f3f217c5ee2187a083b0dddbecda543702f8e7a0f224902cc7e28a8407ddcb49206dfd14abeb954dfde408643f775000036b4849a0f31c8bc89f8b1eee9be6fca229e5ae10e8e8c848549722f2351862283ed32262a0f2911976ea9dc296cae3799e505c88c137392e97eb2e49f727fac5c51a5310f6a02a28aedfd4a89133d9fe7529e993dd55d1896e250dcc4306f55e9d3727263235a024c23222dd272801d980ca30083e1f72ee7a561f9056e28e6c7137410df91ddea047e937c6ea316b7f0d2636ee55870260e3309ebcafcaa2ba78b29cba241849f0a00aed3710f482aaf9d34843a433d12fc8c1d4e4b3383c6b60af16e037e62f411bf8e7a06ab8b1cfa0e1bf437ec02b2cef36851a253f57969f2ead0463db26cd924313bda0b9405c5c859c5ef2b3857ad9cd2b70080fdbbd0251168f370d8b8e43a5d4216ba019199105824b484705556b5881accc38a93da98d4fe7e96544fc137b93ace5a8a07a5b71a0772b098ee19142df524222ada7b1d9f6809ca5be2ad3bad10aa911a9a0f7092f184e69aef577b0b6ea70f3071a608b503497e21218d3740b290948d9eca05313fd4f3dc6c7cec549613ad691518bcc37ab10e4445cb514f6668fc465161ba0cc90d4e34895f1551a2fd1d32b56028a2ad7ac3db0d3f9d7f0c98815fc05b61ca0a43383c0b8b4db9e85f364796ac7ef7fd664263a91c781702edd4d7098890a0da00e4108c7f53eedf30a94566866bec38d8821ac182b20b9ad56f2f4bed2042e05a0afc0a36e3534dd987dccf2b8913f4f74293d94f697810818eb6d9728422d8100c0c0c0")
	tx2 = genUTXOTransaction("f90550eb10c698b1d37308e3c180a06eb3e752ec002520a76bf097a2822ab461d46239af8b1d642019cbd014c076eff895c77cca059e4aa7f8409454fb1c7d0f011dd63b08f85ed7b518ab82028100888ac7230489e8000080a07a7f1a606d645260f9b7aac6f219a178364870071522d7ad8022538e90f3b07c5842699137a517f843a0c03853355f0edf8b3ef7d6d1cb5483f179638d66e1ff2901dfbf3eb99fe4f97e80a02ea64cc3d07a23db249743b88f2cc5460fa46bbdf2a5daebf4f1dcab6a189d1f940000000000000000000000000000000000000000a0fa6ba9366c95a7088f65ced06e01a2dc3a7df8b9732c657e64a02bc3b64ed793f842a0ad0131ec8f1b60f621da929934c65e4081303128ba223d141ffc744276eeb64ca0958c5896b3095399affd774b09c987afacf1319e61979bc31f308ae6dfb295c98902bc2267b45675000080c3808080f90401f8b003c0f865f863a0025a8e84dadcb83b128da0c5d0a21ab43781cf8d35a88b803b4efb9edc80e805a06731037b6b3658d63c3425be5fb046ea3bdff459011ac2c370eb06209f64ff0ba00000000000000000000000000000000000000000000000000000000000000000f844f842a00000000000000000000000000000000000000000000000000000000000000000a0898fe65a1f60430004e2e69ad3662e8d68d0394e81eddfc17424496b18128a5a80f9034cc0f902bcf902b9a0cb5b3aab46a707808b898db23e7fa2bd1bcab8cd57a8019178986e79aed12eeca006b184b90f9694e4e05103911cdef55fb628e235a28d1a0fa14d56862fe5cf68a005055432073c946c92731e32c935a56d19cac6e2ee599c69dc10085d6a69390da02ecc40506c0b995635374f33fe296209aeec81c718691ce7f6f62e190e3bde6da014f606d1fd009dba719d8ffe915818824c440976c0477ff5f6354d7f32229c05a0366bfc247a9ba58c842de80c2b6dd2d98ad6a8e3da647284fbba263e6825530af8c6a0c6448eeba096a5c951935b95674ecea74cf640a30535e423b67a0742730a1fe0a0814819eb57a9f0ece7c0bc94023e4910392fcbfe6cb440634e738fa565139543a0e2f84715c66192dfffad5dbb6310c5833b584be259e79314704565ac69551433a0bde84b28b97b3d673982d9b4634676d2c161fcc1a612ab4e3d2984450dbbfd6fa0edfe0f33389b77ab6895a0599a38f7889e7dc2cc5a24b76f75becc8cca526421a0c732d51801dad6fe21938bfd25d211f9a04c8d130b50a4559a9fe6162a044474f8c6a0b80568f8555a3a772aabe8beed4f027de0ce103ae5bb3b78e4f2ce9c088691caa07d291ff24b1215c89481ef30ffc02c9eaf4afbf24c694319bb26a6b431da283ea09863af7a3b9482a3c1e87792bed5bb26d6dd91a00dfd1a5bf51b4a4eef02947fa0ce19e2717daf1a1a27f1c6f10f0e751334aafe6d1086dec293dfbd0533408511a03e4371b47d2b1ba7ca64c6e72231ab64b6c9b0ab9a3c989f71d800554d9821daa0a574c5a88cca20d1a3f4f3fe7da799afffa7bee2b90a38dddc571c9b4267013fa04a35b25ab44f9234fc9d8a8ada903d9e570609d0357154ee8a588f5c5b066d02a0a35b1ef490c8f2bf400ee77d5db46bd541a8fa0584eaaa650f486ef824a2dc05a06082702813c8fa649fad8822b99d52f120790dcb9ec3ba63848740396d41c50ae3e2c0a00000000000000000000000000000000000000000000000000000000000000000e1a0c7edc830e8ef359b4eedc88dadcbd3a0de2f51d2a0813702390f1f01f88e7cdef844f842a0596bdae6be14765383abaec4e27f07e1e176ad887b57fa8fb8cecb42f5727100a04c877f84728374df68e8788debe774461d14815002cfdcb172efd054eb31d90b")

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{big.NewInt(0), big.NewInt(0)}
	block := genBlock(types.Txs{tx1, tx2})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{balance12, balance22}

	expectAmount := calExpectAmount(amount1b, amount2b)
	expectFee := calExpectFee(fee1, fee2)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee) //#1
	resultChecker(t, receipts, utxoOutputs, keyImages, 2, 3, 1)
	othersChecker(t, expectNonce, actualNonce)

	receiptHash := receipts.Hash()
	stateHash := statedb.IntermediateRoot(false)
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.Json())
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0x86095bfcdb8ed5b69805acd146ff921e2d60aca66332e438357ecffa410af57f", "0x02cb53d193bf820f600346fb1e08e1aa1a3c0df9b76ccfa4814c140063f1642a", "0xe57dd5d7c5cdc7f930a2c4a5ef0c85a50c73c16ea419a71a5a56bb89a0ee9650")
}

/* Not Support
//U->M token
func TestSingleUTXO2MixToken2(t *testing.T) {
		statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()

	sender := Bank[0].PrivateKey
	sAdd := Bank[0].Address
	utxo1, utxo2 := UtxoAccs[0], UtxoAccs[1]

	amount1 := big.NewInt(0)
	fee1 := uint64(0)
	nonce := uint64(0)
	tx1 := newContractTx(sAdd, 100000, nonce, "../test/token/sol/SimpleToken.bin")
	tx1.Sign(types.GlobalSTDSigner, sender)
	tkAdd := crypto.CreateAddress(tx1.FromAddr, tx1.Nonce(), tx1.Data())
	nonce++

	amount2 := big.NewInt(0)
	fee2 := types.CalNewAmountGas(big.NewInt(0), types.EverLiankeFee)
	//fee2i := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee2), big.NewInt(types.ParGasPrice))
	tkamount2 := big.NewInt(10000)
	tkamount2a := big.NewInt(9999)
	tkamount2b := big.NewInt(0).Sub(tkamount2, tkamount2a)
	ain := types.AccountSourceEntry{
		From:   sAdd,
		Nonce:  nonce,
		Amount: tkamount2,
	}
	uout1 := types.UTXODestEntry{
		Addr:   utxo1.Addr,
		Amount: tkamount2a,
	}
	uout2 := types.UTXODestEntry{
		Addr:   utxo2.Addr,
		Amount: tkamount2b,
	}
	tx2, _, err := types.NewAinTransaction(&ain, []types.DestEntry{&uout1, &uout2}, tkAdd, nil)
	if err != nil {
		panic(err)
	}
	tx2.Sign(types.GlobalSTDSigner, sender)
	nonce++
	balance21, mask21 := getBalance(tx2, lktypes.SecretKey(utxo1.Skv), lktypes.SecretKey(utxo1.Sks))
	balance22, _ := getBalance(tx2, lktypes.SecretKey(utxo2.Skv), lktypes.SecretKey(utxo2.Sks))

	amount3 := big.NewInt(0)
	fee3 := types.CalNewAmountGas(big.NewInt(0), types.EverLiankeFee) + uint64(5e8)
	//fee3i := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee3), big.NewInt(types.ParGasPrice))
	tkamount3 := tkamount2a
	tkamount3a := big.NewInt(3000)
	tkamount3b := big.NewInt(0).Sub(tkamount3, tkamount3a)
	sEntey1 := &types.UTXOSourceEntry{
		Ring: []types.UTXORingEntry{types.UTXORingEntry{
			Index:  0,
			OTAddr: tx2.Outputs[0].(*types.UTXOOutput).OTAddr,
		}},
		RingIndex: 0,
		RKey:      tx2.RKey,
		OutIndex:  0,
		Amount:    balance21,
		Mask:      mask21,
	}
	aDest := &types.AccountDestEntry{
		To:     sAdd,
		Amount: tkamount3a,
	}
	uDest := &types.UTXODestEntry{
		Addr:   uout2.Addr,
		Amount: tkamount3b,
	}
	tx3, ie, mk, _, err := types.NewUinTransaction(&utxo1.Acc, utxo1.Keyi, []*types.UTXOSourceEntry{sEntey1}, []types.DestEntry{aDest, uDest}, tkAdd, common.EmptyAddress, nil)
	if err != nil {
		panic(err)
	}
	tx3.Sign(types.GlobalSTDSigner, sender)
	err = types.UInTransWithRctSig(tx3, []*types.UTXOSourceEntry{sEntey1}, ie, []types.DestEntry{aDest, uDest}, mk)
	if err != nil {
		panic(err)
	}
	nonce++
	balance32, _ := getBalance(tx3, lktypes.SecretKey(utxo2.Skv), lktypes.SecretKey(utxo2.Sks))

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bftkBalanceIn := []*big.Int{statedb.GetTokenBalance(sAdd, tkAdd)}
	bfBalanceOut := []*big.Int{}
	bftkBalanceOut := []*big.Int{big.NewInt(0), big.NewInt(0)}
	block := genBlock(types.Txs{tx1, tx2, tx3})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}
	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	aftkBalanceIn := []*big.Int{statedb.GetTokenBalance(sAdd, tkAdd)}
	afBalanceOut := []*big.Int{}
	aftkBalanceOut := []*big.Int{balance22, balance32}

	expectAmount := calExpectAmount(amount1, amount2, amount3)
	expecttkAmount := calExpectAmount(tkamount2b, tkamount3b)
	expectFee := calExpectFee(fee1, fee2, fee3)
	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))
	expectNonce := []uint64{nonce}
	actualNonce := []uint64{statedb.GetNonce(sAdd)}

	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, expectAmount, expectFee, actualFee)
	balancesChecker(t, bftkBalanceIn, aftkBalanceIn, bftkBalanceOut, aftkBalanceOut, expecttkAmount, big.NewInt(0), big.NewInt(0))
	resultChecker(t, receipts, utxoOutputs, keyImages, 3, 3, 1)
	othersChecker(t, expectNonce, actualNonce)
}
*/

/* Not Support
//U->C
func TestSingleUTXO2Contract(t *testing.T) {
		statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()
	sender := Bank[0].PrivateKey

	sAdd := crypto.PubkeyToAddress(sender.PublicKey)

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	bfBalanceOut := []*big.Int{big.NewInt(0)}

	tx1 := newContractTx(sAdd, 1000000, 0, "../test/token/sol/SimpleToken.bin")
	tx1.Sign(types.GlobalSTDSigner, sender)
	tkAdd := crypto.CreateAddress(tx1.FromAddr, tx1.Nonce(), tx1.Data())
	expectFee1 := big.NewInt(0)

	amount := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(100))
	ain := types.AccountSourceEntry{
		From:   sAdd,
		Nonce:  1,
		Amount: amount,
	}
	fee := types.CalNewAmountGas(amount, types.EverLiankeFee)
	expectFee2 := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee), big.NewInt(types.ParGasPrice))
	fee3 := uint64(31539)
	expectFee3 := big.NewInt(0).Add(big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee3), big.NewInt(types.ParGasPrice)), big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(50)))

	amount = big.NewInt(0).Sub(amount, expectFee2)
	amount1 := expectFee3 //big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(51))

	amount2 := big.NewInt(0).Sub(amount, amount1)

	sks1, pks1 := xcrypto.SkpkGen()
	skv1, pkv1 := xcrypto.SkpkGen()
	sks2, pks2 := xcrypto.SkpkGen()
	skv2, pkv2 := xcrypto.SkpkGen()

	rAddr1 := lktypes.AccountAddress{
		ViewPublicKey:  lktypes.PublicKey(pkv1),
		SpendPublicKey: lktypes.PublicKey(pks1),
	}
	rAddr2 := lktypes.AccountAddress{
		ViewPublicKey:  lktypes.PublicKey(pkv2),
		SpendPublicKey: lktypes.PublicKey(pks2),
	}
	var remark [32]byte
	uout1 := types.UTXODestEntry{
		Addr:         rAddr1,
		Amount:       amount1,
		IsSubaddress: false,
		IsChange:     false,
		Remark:       remark,
	}
	uout2 := types.UTXODestEntry{
		Addr:         rAddr2,
		Amount:       amount2,
		IsSubaddress: false,
		IsChange:     false,
		Remark:       remark,
	}

	tx2, _, err := types.NewAinTransaction(&ain, []types.DestEntry{&uout1, &uout2}, common.EmptyAddress, expectFee2, nil)
	if err != nil {
		panic(err)
	}
	err = tx2.Sign(types.GlobalSTDSigner, sender)
	if err != nil {
		panic(err)
	}
	sEntey1 := &types.UTXOSourceEntry{
		Ring: []types.UTXORingEntry{types.UTXORingEntry{
			Index:  0,
			OTAddr: tx2.Outputs[0].(*types.UTXOOutput).OTAddr,
			Commit: tx2.Outputs[0].(*types.UTXOOutput).Remark,
		}},
		RingIndex: 0,
		RKey:      tx2.RKey,
		OutIndex:  0,
		Amount:    big.NewInt(0).Set(amount1),
		Mask:      tx2.RCTSig.RctSigBase.EcdhInfo[0].Mask,
	}

	acc1 := lktypes.AccountKey{
		Addr: lktypes.AccountAddress{
			SpendPublicKey: lktypes.PublicKey(pks1),
			ViewPublicKey:  lktypes.PublicKey(pkv1),
		},
		SpendSKey: lktypes.SecretKey(sks1),
		ViewSKey:  lktypes.SecretKey(skv1),
		SubIdx:    uint64(0),
	}
	address := wallet.AddressToStr(&acc1, uint64(0))
	acc1.Address = address
	keyi1 := make(map[lktypes.PublicKey]uint64)
	keyi1[acc1.Addr.SpendPublicKey] = 0

	//var cabi abi.ABI
	bin, err := ioutil.ReadFile("../test/token/sol/SimpleToken.abi")
	if err != nil {
		panic(err)
	}
	cabi, err := abi.JSON(bytes.NewReader(bin))
	if err != nil {
		panic(err)
	}
	var data []byte
	method := "transfertokentest"
	data, err = cabi.Pack(method, big.NewInt(0))
	if err != nil {
		panic(err)
	}

	aDest := &types.AccountDestEntry{
		To:     tkAdd,
		Amount: big.NewInt(0),
		Data:   data,
	}

	tx3, ie, mk, _, err := types.NewUinTransaction(&acc1, keyi1, []*types.UTXOSourceEntry{sEntey1}, []types.DestEntry{aDest}, common.EmptyAddress, sAdd, expectFee3, []byte{})
	tx3.Sign(types.GlobalSTDSigner, sender)
	err = types.UInTransWithRctSig(tx2, []*types.UTXOSourceEntry{sEntey1}, ie, []types.DestEntry{aDest}, mk)
	if err != nil {
		panic(err)
	}
	if err != nil {
		panic(err)
	}

	block := genBlock(types.Txs{tx1, tx2, tx3})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}

	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd)}
	afBalanceOut := []*big.Int{getBalance(tx2, lktypes.SecretKey(skv2), lktypes.SecretKey(sks2))}

	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))

	amount = amount2
	expectFee := big.NewInt(0).Add(big.NewInt(0).Add(expectFee1, expectFee2), expectFee3)
	println(bfBalanceIn[0].String(), afBalanceIn[0].String(), bfBalanceOut[0].String(), afBalanceOut[0].String(), amount.String(), expectFee.String(), actualFee.String())
	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, amount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 3, 2, 1)
}
*/

/* Not Support
//U->C2 (value transfer)
func TestSingleUTXO2Contract2(t *testing.T) {
		statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()
	sender := Bank[0].PrivateKey

	sAdd := crypto.PubkeyToAddress(sender.PublicKey)

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd), big.NewInt(0)}
	bfBalanceOut := []*big.Int{big.NewInt(0), big.NewInt(0)}

	tx1 := newContractTx(sAdd, 1000000, 0, "../test/token/sol/t.bin")
	tx1.Sign(types.GlobalSTDSigner, sender)
	tkAdd := crypto.CreateAddress(tx1.FromAddr, tx1.Nonce(), tx1.Data())
	expectFee1 := big.NewInt(0)

	amount := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(1000))
	ain := types.AccountSourceEntry{
		From:   sAdd,
		Nonce:  1,
		Amount: amount,
	}
	fee := types.CalNewAmountGas(amount, types.EverLiankeFee)
	expectFee2 := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee), big.NewInt(types.ParGasPrice))
	amount = big.NewInt(0).Sub(amount, expectFee2)
	amount1 := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(500))
	amount2 := big.NewInt(0).Sub(amount, amount1)

	sks1, pks1 := xcrypto.SkpkGen()
	skv1, pkv1 := xcrypto.SkpkGen()
	sks2, pks2 := xcrypto.SkpkGen()
	skv2, pkv2 := xcrypto.SkpkGen()

	rAddr1 := lktypes.AccountAddress{
		ViewPublicKey:  lktypes.PublicKey(pkv1),
		SpendPublicKey: lktypes.PublicKey(pks1),
	}
	rAddr2 := lktypes.AccountAddress{
		ViewPublicKey:  lktypes.PublicKey(pkv2),
		SpendPublicKey: lktypes.PublicKey(pks2),
	}
	var remark [32]byte
	uout1 := types.UTXODestEntry{
		Addr:         rAddr1,
		Amount:       amount1,
		IsSubaddress: false,
		IsChange:     false,
		Remark:       remark,
	}
	uout2 := types.UTXODestEntry{
		Addr:         rAddr2,
		Amount:       amount2,
		IsSubaddress: false,
		IsChange:     false,
		Remark:       remark,
	}

	tx2, _, err := types.NewAinTransaction(&ain, []types.DestEntry{&uout1, &uout2}, common.EmptyAddress, expectFee2, nil)
	if err != nil {
		panic(err)
	}
	err = tx2.Sign(types.GlobalSTDSigner, sender)
	if err != nil {
		panic(err)
	}
	sEntey1 := &types.UTXOSourceEntry{
		Ring: []types.UTXORingEntry{types.UTXORingEntry{
			Index:  0,
			OTAddr: tx2.Outputs[0].(*types.UTXOOutput).OTAddr,
			Commit: tx2.Outputs[0].(*types.UTXOOutput).Remark,
		}},
		RingIndex: 0,
		RKey:      tx2.RKey,
		OutIndex:  0,
		Amount:    big.NewInt(0).Set(amount1),
		Mask:      tx2.RCTSig.RctSigBase.EcdhInfo[0].Mask,
	}

	acc1 := lktypes.AccountKey{
		Addr: lktypes.AccountAddress{
			SpendPublicKey: lktypes.PublicKey(pks1),
			ViewPublicKey:  lktypes.PublicKey(pkv1),
		},
		SpendSKey: lktypes.SecretKey(sks1),
		ViewSKey:  lktypes.SecretKey(skv1),
		SubIdx:    uint64(0),
	}
	address := wallet.AddressToStr(&acc1, uint64(0))
	acc1.Address = address
	keyi1 := make(map[lktypes.PublicKey]uint64)
	keyi1[acc1.Addr.SpendPublicKey] = 0

	//var cabi abi.ABI
	bin, err := ioutil.ReadFile("../test/token/sol/t.abi")
	if err != nil {
		panic(err)
	}
	cabi, err := abi.JSON(bytes.NewReader(bin))
	if err != nil {
		panic(err)
	}
	var data []byte
	method := "set"
	data, err = cabi.Pack(method, big.NewInt(0))
	if err != nil {
		panic(err)
	}
	amount3 := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(100))
	aDest := &types.AccountDestEntry{
		To:     tkAdd,
		Amount: amount3,
		Data:   data,
	}
	fee3 := uint64(26596)
	expectFee3 := big.NewInt(0).Add(big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee3), big.NewInt(types.ParGasPrice)), big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(50)))

	txfee3 := big.NewInt(0).Sub(amount1, amount3)
	tx3, ie, mk, _, err := types.NewUinTransaction(&acc1, keyi1, []*types.UTXOSourceEntry{sEntey1}, []types.DestEntry{aDest}, common.EmptyAddress, sAdd, txfee3, []byte{})
	tx3.Sign(types.GlobalSTDSigner, sender)
	err = types.UInTransWithRctSig(tx2, []*types.UTXOSourceEntry{sEntey1}, ie, []types.DestEntry{aDest}, mk)
	if err != nil {
		panic(err)
	}
	if err != nil {
		panic(err)
	}

	block := genBlock(types.Txs{tx1, tx2, tx3})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}

	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd), big.NewInt(0)}
	afBalanceOut := []*big.Int{getBalance(tx2, lktypes.SecretKey(skv2), lktypes.SecretKey(sks2)), statedb.GetBalance(tkAdd)}

	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))

	amount = big.NewInt(0).Add(amount2, amount3)
	expectFee := big.NewInt(0).Add(big.NewInt(0).Add(expectFee1, expectFee2), expectFee3)
	println(expectFee1.String(), expectFee2.String(), expectFee3.String(), actualFee.String())
	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, amount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 3, 2, 1)

}
*/

/* Not Support
//U->C2E (value transfer gas usage test) this test is designated to throw err
func TestSingleUTXO2Contract3(t *testing.T) {
		statedb := newTestState()
	types.SaveBalanceRecord = true
	types.BlockBalanceRecordsInstance = types.NewBlockBalanceRecords()
	sender := Bank[0].PrivateKey

	sAdd := crypto.PubkeyToAddress(sender.PublicKey)

	bfBalanceIn := []*big.Int{statedb.GetBalance(sAdd), big.NewInt(0)}
	bfBalanceOut := []*big.Int{big.NewInt(0), big.NewInt(0)}

	tx1 := newContractTx(sAdd, 1000000, 0, "../test/token/sol/t.bin")
	tx1.Sign(types.GlobalSTDSigner, sender)
	tkAdd := crypto.CreateAddress(tx1.FromAddr, tx1.Nonce(), tx1.Data())
	expectFee1 := big.NewInt(0)

	amount := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(1000))
	ain := types.AccountSourceEntry{
		From:   sAdd,
		Nonce:  1,
		Amount: amount,
	}
	fee := types.CalNewAmountGas(amount, types.EverLiankeFee)
	expectFee2 := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee), big.NewInt(types.ParGasPrice))
	amount = big.NewInt(0).Sub(amount, expectFee2)
	amount1 := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(500))
	amount2 := big.NewInt(0).Sub(amount, amount1)

	sks1, pks1 := xcrypto.SkpkGen()
	skv1, pkv1 := xcrypto.SkpkGen()
	sks2, pks2 := xcrypto.SkpkGen()
	skv2, pkv2 := xcrypto.SkpkGen()

	rAddr1 := lktypes.AccountAddress{
		ViewPublicKey:  lktypes.PublicKey(pkv1),
		SpendPublicKey: lktypes.PublicKey(pks1),
	}
	rAddr2 := lktypes.AccountAddress{
		ViewPublicKey:  lktypes.PublicKey(pkv2),
		SpendPublicKey: lktypes.PublicKey(pks2),
	}
	var remark [32]byte
	uout1 := types.UTXODestEntry{
		Addr:         rAddr1,
		Amount:       amount1,
		IsSubaddress: false,
		IsChange:     false,
		Remark:       remark,
	}
	uout2 := types.UTXODestEntry{
		Addr:         rAddr2,
		Amount:       amount2,
		IsSubaddress: false,
		IsChange:     false,
		Remark:       remark,
	}

	tx2, _, err := types.NewAinTransaction(&ain, []types.DestEntry{&uout1, &uout2}, common.EmptyAddress, expectFee2, nil)
	if err != nil {
		panic(err)
	}
	err = tx2.Sign(types.GlobalSTDSigner, sender)
	if err != nil {
		panic(err)
	}
	sEntey1 := &types.UTXOSourceEntry{
		Ring: []types.UTXORingEntry{types.UTXORingEntry{
			Index:  0,
			OTAddr: tx2.Outputs[0].(*types.UTXOOutput).OTAddr,
			Commit: tx2.Outputs[0].(*types.UTXOOutput).Remark,
		}},
		RingIndex: 0,
		RKey:      tx2.RKey,
		OutIndex:  0,
		Amount:    big.NewInt(0).Set(amount1),
		Mask:      tx2.RCTSig.RctSigBase.EcdhInfo[0].Mask,
	}

	acc1 := lktypes.AccountKey{
		Addr: lktypes.AccountAddress{
			SpendPublicKey: lktypes.PublicKey(pks1),
			ViewPublicKey:  lktypes.PublicKey(pkv1),
		},
		SpendSKey: lktypes.SecretKey(sks1),
		ViewSKey:  lktypes.SecretKey(skv1),
		SubIdx:    uint64(0),
	}
	address := wallet.AddressToStr(&acc1, uint64(0))
	acc1.Address = address
	keyi1 := make(map[lktypes.PublicKey]uint64)
	keyi1[acc1.Addr.SpendPublicKey] = 0

	//var cabi abi.ABI
	bin, err := ioutil.ReadFile("../test/token/sol/t.abi")
	if err != nil {
		panic(err)
	}
	cabi, err := abi.JSON(bytes.NewReader(bin))
	if err != nil {
		panic(err)
	}
	var data []byte
	method := "set"
	data, err = cabi.Pack(method, big.NewInt(0))
	if err != nil {
		panic(err)
	}
	fee3 := uint64(26596) + uint64(5e8)
	expectFee3 := big.NewInt(0).Mul(big.NewInt(0).SetUint64(fee3), big.NewInt(types.ParGasPrice))
	amount3 := big.NewInt(0).Sub(amount1, big.NewInt(0).Sub(expectFee3, big.NewInt(1e11))) //big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(100))
	aDest := &types.AccountDestEntry{
		To:     tkAdd,
		Amount: amount3,
		Data:   data,
	}

	tx3, ie, mk, _, err := types.NewUinTransaction(&acc1, keyi1, []*types.UTXOSourceEntry{sEntey1}, []types.DestEntry{aDest}, common.EmptyAddress, sAdd, expectFee3, []byte{})
	tx3.Sign(types.GlobalSTDSigner, sender)
	err = types.UInTransWithRctSig(tx2, []*types.UTXOSourceEntry{sEntey1}, ie, []types.DestEntry{aDest}, mk)
	if err != nil {
		panic(err)
	}
	if err != nil {
		panic(err)
	}

	block := genBlock(types.Txs{tx1, tx2, tx3})
	receipts, _, blockGas, _, utxoOutputs, keyImages, err := SP.Process(block, statedb, VC)
	if err != nil {
		panic(err)
	}

	afBalanceIn := []*big.Int{statedb.GetBalance(sAdd), big.NewInt(0)}
	afBalanceOut := []*big.Int{getBalance(tx2, lktypes.SecretKey(skv2), lktypes.SecretKey(sks2)), statedb.GetBalance(tkAdd)}

	actualFee := big.NewInt(0).Mul(big.NewInt(0).SetUint64(blockGas), big.NewInt(types.ParGasPrice))

	amount = big.NewInt(0).Add(amount2, amount3)
	expectFee := big.NewInt(0).Add(big.NewInt(0).Add(expectFee1, expectFee2), expectFee3)
	println(expectFee1.String(), expectFee2.String(), expectFee3.String(), actualFee.String())
	balancesChecker(t, bfBalanceIn, afBalanceIn, bfBalanceOut, afBalanceOut, amount, expectFee, actualFee)
	resultChecker(t, receipts, utxoOutputs, keyImages, 3, 2, 1)
}
*/
