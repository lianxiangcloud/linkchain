package types

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/lianxiangcloud/linkchain/libs/common"
	lcrypto "github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"

	"github.com/lianxiangcloud/linkchain/libs/cryptonote/crypto"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/ringct"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/xcrypto"
)

//UTXOKind catagory UTXOTransaction kind by input and output.
type UTXOKind byte

const (
	IllKind UTXOKind = 0x00
	//  1   1   1    1
	// uin ain uout aout
	Uin       UTXOKind = 0x01
	Ain       UTXOKind = 0x02
	Uout      UTXOKind = 0x04
	Aout      UTXOKind = 0x08
	AinAout   UTXOKind = Ain | Aout
	UinUout   UTXOKind = Uin | Uout
	AinUout   UTXOKind = Ain | Uout
	UinAout   UTXOKind = Uin | Aout
	MixinAout UTXOKind = Uin | Ain | Aout
	MixinUout UTXOKind = Uin | Ain | Uout
	MixinAll  UTXOKind = Uin | Ain | Uout | Aout
)

var (
	ErrDerivationKey              = errors.New("derivaion key fail")
	ErrDerivationSecretKey        = errors.New("derivaion secret key fail")
	ErrDerivationPublicKey        = errors.New("derivaion public key fail")
	ErrDerivationSubaddrPublicKey = errors.New("derivaion subaddr public key fail")
	ErrDerivationScalar           = errors.New("derivaion scalar fail")
	ErrGenerateKeyImage           = errors.New("generate key image fail")
	ErrOutputNotBelongToAccount   = errors.New("output not belong to account")
	ErrInMoneyLessThanOutMoney    = errors.New("input money less than output money")
	ErrOutSkSizeNotMatch          = errors.New("outSk size does not match outputs")
	ErrUtxoOutSizeNotExpect       = errors.New("utxo output size not expect")
	ErrMoneyInvalid               = errors.New("money invalid")
	ErrOutsAndMkeysNotMatch       = errors.New("utxoOuts and mkeys does not match")
	ErrProveRangeBulletproof      = errors.New("prove range bulletproof err")
	ErrVerRangeBulletproofFailed  = errors.New("verify range bulletproof failed")
	ErrEcdhEncode                 = errors.New("ecdh encode err")
	ErrAccountInputSizeNotExpect  = errors.New("account input size not expect")
	ErrInputTypeNotExpect         = errors.New("input type not expect")
	ErrOutputTypeNotExpect        = errors.New("output type not expect")
	ErrGetInputFromDB             = errors.New("input index not exists")
	ErrCheckNoInput               = errors.New("no inputs")
	ErrCheckDupKeyImage           = errors.New("input KeyImage duplicated")
	ErrCheckDupRingMember         = errors.New("duplicate ring members")
	ErrCheckKeyImageInvalid       = errors.New("image not in valid domain")
	ErrCheckSecAddr               = errors.New("secret address illegal")
	ErrCheckAmountCommit          = errors.New("amount commit not equal(c!=aG+bH)")
	ErrCheckBadMGsSize            = errors.New("bad MGs size")
	ErrVerRingCTSignatures        = errors.New("verify rct signature failed")
	ERRCheckInvalidMixRing        = errors.New("invalid mixRing size")
	ErrCheckBadBulletproofSize    = errors.New("bad bullet proof size")
	ErrCheckBadBulletproofLSize   = errors.New("bad bullet proof L size")
	ErrCheckEmptyInputCommits     = errors.New("inputCommits Empty")
	ErrCheckInOutCommitNotEqual   = errors.New("sum of inputs commits not equal sum of output commits")
	ErrCheckAccountOutputsIllegal = errors.New("account outputs illegal")
	ErrMixRingMemberNotSupport    = errors.New("mix ring member not support")
	ErrRingCTSignaturesInvalid    = errors.New("rct signature invalid")
)

const (
	BULLETPROOF_MAX_OUTPUTS     int   = 16
	CRYPTONOTE_MAX_TX_SIZE      int   = 1000000
	SHORT_RING_MEMBER_NUM       int   = 1
	UTXO_COMMITMENT_CHANGE_RATE int64 = 1e10
)

var _ RegularTx = &UTXOTransaction{}

func RegisterUTXOTxData() {
	ser.RegisterConcrete(&UTXOTransaction{}, TxUTXO, nil)

	ser.RegisterInterface((*Input)(nil), nil)
	ser.RegisterConcrete(&UTXOInput{}, "UTXOInput", nil)
	ser.RegisterConcrete(&AccountInput{}, "AccountInput", nil)
	ser.RegisterConcrete(&MineInput{}, "MineInput", nil)

	ser.RegisterInterface((*Output)(nil), nil)
	ser.RegisterConcrete(&UTXOOutput{}, "UTXOOutput", nil)
	ser.RegisterConcrete(&AccountOutput{}, "AccountOutput", nil)
}

//Input represents a utxo or account input
type Input interface {
	Type() string
}

//Output represents a utxo or account output
type Output interface {
	Type() string
}

//UTXOInput represents a utxo input
type UTXOInput struct {
	//Amount    *big.Int  `json:"amount"`
	KeyOffset []uint64  `json:"key_offset"`
	KeyImage  types.Key `json:"key_image"`
}

//Type - InUTXO
func (*UTXOInput) Type() string {
	return InUTXO
}

func (u *UTXOInput) String() string {
	return fmt.Sprintf("[KeyOffset:%v,KeyImage:%x]", u.KeyOffset, u.KeyImage)
}

//UTXOOutput represents a utxo output
type UTXOOutput struct {
	OTAddr types.Key `json:"otaddr"`
	Amount *big.Int  `json:"amount"`
	Remark [32]byte  `json:"remark"`
}

//Type - OutUTXO
func (*UTXOOutput) Type() string {
	return OutUTXO
}

func (u *UTXOOutput) String() string {
	return fmt.Sprintf("[OTAddr:%x,Remark:%x]", u.OTAddr, u.Remark)
}

//AccountInput represents a account input
type AccountInput struct {
	//From   common.Address `json:"from"`
	Nonce  uint64    `json:"nonce"`
	Amount *big.Int  `json:"amount"` //Amount  =  user set amount + Fee, b in c = aG + bH
	CF     types.Key `json:"cf"`     //confusion factor, a in c = aG + bH
	Commit types.Key `json:"commit"` //Amount's commitment, c in c = aG + bH
}

//Type -InAc
func (*AccountInput) Type() string {
	return InAc
}

func (ai *AccountInput) String() string {
	return fmt.Sprintf("[Nonce:%d,Amount:%d,CF:%x,Commit:%x,]", ai.Nonce, ai.Amount.Uint64(), ai.CF, ai.Commit)
}

//AccountOutput represents a account output
type AccountOutput struct {
	To     common.Address `json:"to"`
	Amount *big.Int       `json:"amount"`
	Data   []byte         `json:"data"`   //contract data
	Commit types.Key      `json:"commit"` //Amount's commitment
}

//Type -OutAc
func (*AccountOutput) Type() string {
	return OutAc
}

func (ai *AccountOutput) String() string {
	return fmt.Sprintf("[To:%s,Amount:%d,Data:%x,Commit:%x,]", ai.To.String(), ai.Amount.Uint64(), ai.Data, ai.Commit)
}

type MineInput struct {
	Height uint64
}

//Type - InMine
func (*MineInput) Type() string {
	return InMine
}

//UTXOInputEphemeral represents a ephemeral status of input
type UTXOInputEphemeral struct {
	OTAddr   types.Key
	SKey     types.SecretKey
	KeyImage types.Key
}

//UTXOTransaction represents a utxo transaction for UTXOs
type UTXOTransaction struct {
	Inputs  []Input        `json:"inputs"`
	Outputs []Output       `json:"outputs"`
	TokenID common.Address `json:"token_id"` //current version one Tx support only one token
	//RefundAddr common.Address    `json:"refund_addr"`   //RefundAddr only valid when there is only UTXOInput, and accountOutput(UTXOOutput)
	RKey    types.PublicKey   `json:"r_key"`         //each tx with a random PKSK
	AddKeys []types.PublicKey `json:"add_keys"`      //one key per subaddr
	Fee     *big.Int          `json:"fee"`           //fee charge only LKC, without unit (different from gas)
	Extra   []byte            `json:"extra"`         //reserve not used yet
	Sigs    signdata          `json:"signature"`     //account signature
	RCTSig  types.RctSig      `json:"rct_signature"` //ringct signature
	// caches
	kind       atomic.Value
	hash       atomic.Value
	size       atomic.Value
	utxoInNum  atomic.Value
	utxoOutNum atomic.Value
	nonce      uint64
}

var _ Tx = &UTXOTransaction{}

//To return to address, here is none, just for interface
func (tx UTXOTransaction) To() *common.Address {
	return &common.EmptyAddress
}

//ToAddrs return to addresses, for receipt use
func (tx UTXOTransaction) ToAddrs() []common.Address {
	addrs := make([]common.Address, 0)
	for _, out := range tx.Outputs {
		switch aOutput := out.(type) {
		case *AccountOutput:
			addrs = append(addrs, aOutput.To)
		}
	}
	return addrs
}

//TokenAddress returns to tokenaddress
func (tx UTXOTransaction) TokenAddress() common.Address {
	return tx.TokenID
}

func (tx *UTXOTransaction) Hash() common.Hash {
	hash := transactionHash(&tx.hash, tx)
	return hash
}

//PrefixHash - compute prefix hash for ring sig
func (tx UTXOTransaction) PrefixHash() types.Key {
	return types.Key(rlpHash([]interface{}{tx.Inputs, tx.Outputs, tx.TokenID, tx.RKey,
		tx.AddKeys, tx.Fee, tx.Extra, tx.Sigs.R, tx.Sigs.S, tx.Sigs.V}))
}

//String - none
func (tx UTXOTransaction) String() string {
	return fmt.Sprintf(
		`
    Inputs :  %v\
    Outputs  :%v\
    TokenID   :%x\
    RKey       :%x\
    AddKeys    :%x\
    Fee       :%v\
    Extra      :%x\
    Sigs       :%v\
    RCTSig    :%x\
`, tx.Inputs,
		tx.Outputs,
		tx.TokenID,
		tx.RKey,
		tx.AddKeys,
		tx.Fee,
		tx.Extra,
		tx.Sigs,
		tx.RCTSig,
	)
}

//TypeName - none
func (tx UTXOTransaction) TypeName() string {
	return TxUTXO
}

func (tx *UTXOTransaction) Sign(signer STDSigner, prv *ecdsa.PrivateKey) error {
	r, s, v, err := sign(signer, prv, tx.signFields())
	if err != nil {
		return err
	}
	tx.Sigs.R, tx.Sigs.S, tx.Sigs.V = r, s, v
	return nil
}

func (tx *UTXOTransaction) StoreFrom(addr common.Address) {
	tx.Sigs.from().Store(stdSigCache{signer: GlobalSTDSigner, from: addr})
}

func (tx *UTXOTransaction) From() (common.Address, error) {
	return tx.Sender(GlobalSTDSigner)
}

func (tx *UTXOTransaction) Sender(signer STDSigner) (common.Address, error) {
	tx.Sigs.setSignFieldsFunc(tx.signFields)
	return sender(signer, &tx.Sigs)
}

//account input signFields
func (tx UTXOTransaction) signFields() []interface{} {
	return []interface{}{
		tx.Inputs,
		tx.Outputs,
		tx.TokenID,
		tx.RKey,
		tx.AddKeys,
		tx.Fee,
		tx.Extra,
	}
}

func (tx *UTXOTransaction) AsMessage() (Message, error) {
	utxoKind := tx.UTXOKind()
	if utxoKind == IllKind {
		return Message{}, fmt.Errorf("tx not checked")
	}
	if (utxoKind & AinAout) == IllKind {
		return Message{}, fmt.Errorf("tx without account input or output")
	}

	var msg *Message

	if (utxoKind & Ain) == Ain {
		from, err := tx.From()
		if err != nil {
			return Message{}, err
		}
		for _, in := range tx.Inputs {
			switch aInput := in.(type) {
			case *AccountInput:
				gas := big.NewInt(0).Div(tx.Fee, big.NewInt(0).SetInt64(ParGasPrice))
				msg = &Message{
					from:      from,
					nonce:     aInput.Nonce,
					gasLimit:  gas.Uint64(),
					gasPrice:  new(big.Int).SetInt64(ParGasPrice),
					to:        &common.EmptyAddress, //to addr no meaning here
					amount:    aInput.Amount,
					data:      nil, //accountInput no data
					tokenAddr: tx.TokenID,
					txType:    tx.TypeName(),
					utxoKind:  tx.UTXOKind(),
				}
			}
		}
	}
	if (utxoKind & Aout) == Aout {
		for _, out := range tx.Outputs {
			switch aOutput := out.(type) {
			case *AccountOutput:
				if msg == nil {
					gas := big.NewInt(0).Div(tx.Fee, big.NewInt(0).SetInt64(ParGasPrice))
					msg = &Message{
						//do not care nonce, use UTXO Input to keep from double spend
						gasLimit:  gas.Uint64(),
						gasPrice:  new(big.Int).SetInt64(ParGasPrice),
						amount:    big.NewInt(0),
						tokenAddr: tx.TokenID,
						txType:    tx.TypeName(),
						utxoKind:  tx.UTXOKind(),
					}
				}
				aoutputData := OutputData{
					To:     aOutput.To,
					Amount: aOutput.Amount,
					Data:   aOutput.Data,
				}
				msg.accountOutputs = append(msg.accountOutputs, aoutputData)
			}
		}
	}

	return *msg, nil
}

func (tx *UTXOTransaction) UTXOKind() UTXOKind {
	utxoKind := tx.kind.Load()
	if utxoKind == nil {
		var kind UTXOKind
		for _, in := range tx.Inputs {
			switch in.(type) {
			case *UTXOInput:
				kind |= Uin
			case *AccountInput:
				kind |= Ain
			}
		}
		for _, out := range tx.Outputs {
			switch out.(type) {
			case *UTXOOutput:
				kind |= Uout
			case *AccountOutput:
				kind |= Aout
			}
		}
		tx.kind.Store(kind)
		return kind
	}
	kind := utxoKind.(UTXOKind)
	return kind
}

func (tx *UTXOTransaction) Gas() uint64 {
	return big.NewInt(0).Div(tx.Fee, big.NewInt(0).SetInt64(ParGasPrice)).Uint64()
}
func (tx *UTXOTransaction) GasPrice() *big.Int { return new(big.Int).SetInt64(ParGasPrice) }
func (tx *UTXOTransaction) TxType() string     { return TxUTXO }
func (tx *UTXOTransaction) Data() []byte       { panic("should not call"); return nil }
func (tx *UTXOTransaction) Value() *big.Int    { panic("should not call"); return big.NewInt(0) }
func (tx *UTXOTransaction) Nonce() uint64 {
	if (tx.UTXOKind() & Ain) == Ain {
		return tx.nonce
	}
	panic("should not call for no AccountInput UTXOTransaction")
}

//CheckBasic check tx's input, output, commit, bulletproof, and ringct.
func (tx *UTXOTransaction) CheckBasic(censor TxCensor) error {
	log.Debug("CheckBasic", "tx", tx.String(), "txhash", tx.Hash(), "txsize", tx.Size())
	if err := tx.checkTxSemantic(censor); err != nil {
		log.Info("checkTxSemantic", "err", err)
		return err
	}

	// Heuristic limit, reject transactions over 32KB to prevent DOS attacks
	if tx.Size() > MaxPureTransactionSize {
		return ErrOversizedData
	}

	if err := tx.checkCommitEqual(); err != nil {
		log.Info("checkCommitEqual", "err", err)
		return err
	}

	txKind := tx.UTXOKind()
	if (txKind & AinAout) != IllKind {
		fromAddr, _ := tx.From()
		log.Debug("CheckBasic", "txhash", tx.Hash(), "preHash", tx.PrefixHash(), "account from", fromAddr)
	}
	if (txKind & UinUout) == IllKind {
		return nil
	}

	if err := tx.checkRctSigData(); err != nil {
		log.Info("checkRctSigData", "err", err)
		return err
	}

	if (txKind & Uout) == Uout {
		if err := tx.VerifyProofSemantic(); err != nil {
			log.Info("verifyBulletProof", "err", err, "hash", tx.Hash())
			return err
		}
	}

	if (txKind & Uin) == Uin {
		if err := tx.checkTxInputKeys(censor); err != nil {
			log.Info("checkTxInputKeys", "err", err)
			return err
		}
	}
	return nil
}

//CheckState check account state and utxostore if tx has corresponding input type.
func (tx UTXOTransaction) CheckState(censor TxCensor) error {
	return tx.checkState(censor)
}

//UTXORingEntry represents a ring entry for utxo
type UTXORingEntry struct {
	Index  uint64
	OTAddr types.Key
	Commit types.Key
}

//UTXOSourceEntry represents a input entry for utxo
type UTXOSourceEntry struct {
	Ring      []UTXORingEntry
	RingIndex uint64
	RKey      types.PublicKey
	OutIndex  uint64
	Amount    *big.Int
	Mask      types.Key
}

type DestEntry interface {
	Type() string
	GetAmount() *big.Int
}

//UTXODestEntry represents a output entry for utxo
type UTXODestEntry struct {
	Addr         types.AccountAddress
	Amount       *big.Int
	IsSubaddress bool
	IsChange     bool
	Remark       [32]byte
}

func (u *UTXODestEntry) Type() string {
	return TypeUTXODest
}

func (u *UTXODestEntry) GetAmount() *big.Int {
	return u.Amount
}

//UTXODest represents a output entry for utxo rpc
type UTXODest struct {
	Addr   string        `json:"addr"`
	Amount *hexutil.Big  `json:"amount"`
	Remark hexutil.Bytes `json:"remark"`
	Data   hexutil.Bytes `json:"data"`
}

//AccountSourceEntry represents a input entry for account
type AccountSourceEntry struct {
	From   common.Address
	Nonce  uint64
	Amount *big.Int
}

//AccountDestEntry represents a output entry for account
type AccountDestEntry struct {
	To     common.Address `json:"to"`
	Amount *big.Int       `json:"amount"`
	Data   []byte         `json:"data"`
}

func (a *AccountDestEntry) Type() string {
	return TypeAcDest
}

func (a *AccountDestEntry) GetAmount() *big.Int {
	return a.Amount
}

//UTXOOutputData represents utxo output entry in chain db
type UTXOOutputData struct {
	OTAddr  types.Key      `json:"out"`
	Height  uint64         `json:"height"`
	Commit  types.Key      `json:"commit"`
	TokenID common.Address `json:"token"`
	Remark  [32]byte       `json:"remark"`
}

//UTXOOutputDetail represents utxo output entry in local
type UTXOOutputDetail struct {
	BlockHeight  uint64
	TxID         types.Hash
	OutIndex     uint64
	GlobalIndex  uint64
	Spent        bool
	Frozen       bool
	SpentHeight  uint64
	RKey         types.PublicKey
	KeyImage     types.Key
	Mask         types.Key
	Amount       *big.Int
	SubAddrIndex uint64
	TokenID      common.Address
	Remark       [32]byte
}

func (u *UTXOOutputDetail) String() string {
	// b, _ := ser.MarshalJSON(u)
	// return string(b)
	return fmt.Sprintf(`[BlockHeight:%d,
    TxID:%x,
    OutIndex:%d,
    GlobalIndex:%d,
    Spent:%v,
    Frozen:%v,
    SpentHeight:%d,
    RKey:%x,
    KeyImage:%x,
    Mask:%x,
    Amount:%s,
    SubAddrIndex:%d,
    TokenID:%x
	Remark:%x]`, u.BlockHeight, u.TxID, u.OutIndex, u.GlobalIndex, u.Spent, u.Frozen,
		u.SpentHeight, u.RKey, u.KeyImage, u.Mask, u.Amount.String(), u.SubAddrIndex, u.TokenID, u.Remark)
}

//GenerateKeyImage return key-image and secret key of the input utxo
func GenerateKeyImage(acc *types.AccountKey, keyIndex map[types.PublicKey]uint64, sources []*UTXOSourceEntry) ([]*UTXOInputEphemeral, error) {
	utxoInEphs := make([]*UTXOInputEphemeral, len(sources))
	for i := 0; i < len(sources); i++ {
		utxoInEph, err := generateKeyImage(acc, keyIndex, sources[i])
		if err != nil {
			return nil, err
		}
		utxoInEphs[i] = utxoInEph
	}
	return utxoInEphs, nil
}

func generateKeyImage(acc *types.AccountKey, keyIndex map[types.PublicKey]uint64, source *UTXOSourceEntry) (*UTXOInputEphemeral, error) {
	derivationKey, err := xcrypto.GenerateKeyDerivation(source.RKey, acc.ViewSKey)
	if err != nil {
		//log.Error("xcrypto.GenerateKeyDerivation", "rPubkey", source.RKey, "err", err)
		return nil, ErrDerivationKey
	}
	otaddr := source.Ring[source.RingIndex].OTAddr

	subaddrIndex, err := isOutputBelongToAccount(acc, keyIndex, otaddr, derivationKey, source.OutIndex)
	if err != nil {
		return nil, err
	}
	secretKey, err := xcrypto.DeriveSecretKey(derivationKey, int(source.OutIndex), acc.SpendSKey)
	if err != nil {
		//log.Error("xcrypto.DeriveSecretKey", "derivationKey", derivationKey, "err", err)
		return nil, ErrDerivationSecretKey
	}
	sk1 := secretKey
	if subaddrIndex > 0 {
		subaddrSk := xcrypto.GetSubaddressSecretKey(acc.ViewSKey, uint32(subaddrIndex))
		sk1 = xcrypto.SecretAdd(secretKey, subaddrSk)
	}
	keyImage, err := xcrypto.GenerateKeyImage(types.PublicKey(otaddr), sk1)
	if err != nil {
		//log.Error("xcrypto.GenerateKeyImage", "otaddr", otaddr, "err", err)
		return nil, ErrGenerateKeyImage
	}
	utxoInEph := &UTXOInputEphemeral{
		OTAddr:   otaddr,
		SKey:     sk1,
		KeyImage: types.Key(keyImage),
	}
	return utxoInEph, nil
}

func isOutputBelongToAccount(acc *types.AccountKey, keyIndex map[types.PublicKey]uint64, otaddr types.Key, deriKey types.KeyDerivation, index uint64) (uint64, error) {
	spendPubKey, err := xcrypto.DeriveSubaddressPublicKey(types.PublicKey(otaddr), deriKey, int(index))
	if err != nil {
		log.Error("xcrypto.DeriveSubaddressPublicKey", "otaddr", fmt.Sprintf("%x", otaddr), "deriKey", fmt.Sprintf("%x", deriKey), "err", err)
		return 0, ErrDerivationSubaddrPublicKey
	}

	log.Debug("isOutputBelongToAccount", "spendPubKey", fmt.Sprintf("%x", spendPubKey))
	if subaddrIndex, exist := keyIndex[spendPubKey]; exist {
		return subaddrIndex, nil
	}
	return 0, ErrOutputNotBelongToAccount
}

//GenerateOneTimeAddress return one-time addresses of outputs
func GenerateOneTimeAddress(rSecKey types.Key, dests []*UTXODestEntry) ([]*UTXOOutput, types.KeyV, error) {
	utxoOuts := make([]*UTXOOutput, len(dests))
	mKeys := make(types.KeyV, len(dests))
	for i := 0; i < len(dests); i++ {
		utxoOut, mKey, err := generateOneTimeAddress(rSecKey, dests[i], i)
		if err != nil {
			return nil, nil, err
		}
		utxoOuts[i] = utxoOut
		mKeys[i] = mKey
	}
	return utxoOuts, mKeys, nil
}

func generateOneTimeAddress(rSecKey types.Key, dest *UTXODestEntry, index int) (*UTXOOutput, types.Key, error) {
	derivationKey, err := xcrypto.GenerateKeyDerivation(dest.Addr.ViewPublicKey, types.SecretKey(rSecKey))
	if err != nil {
		//log.Error("xcrypto.GenerateKeyDerivation", "viewPubKey", dest.Addr.ViewPublicKey, "err", err)
		return nil, types.Key{}, ErrDerivationKey
	}
	scalar, err := xcrypto.DerivationToScalar(derivationKey, index)
	if err != nil {
		//log.Error("xcrypto.DerivationToScalar", "derivationKey", derivationKey, "err", err)
		return nil, types.Key{}, ErrDerivationScalar
	}
	otAddr, err := xcrypto.DerivePublicKey(derivationKey, index, dest.Addr.SpendPublicKey)
	if err != nil {
		//log.Error("xcrypto.DerivePublicKey", "derivationKey", derivationKey, "err", err)
		return nil, types.Key{}, ErrDerivationPublicKey
	}
	utxoOut := &UTXOOutput{
		OTAddr: types.Key(otAddr),
		Amount: big.NewInt(0).Set(dest.Amount),
	}
	return utxoOut, types.Key(scalar), nil
}

func absoluteOffsetsToRelative(origin []uint64) []uint64 {
	//should sorted already
	if 0 == len(origin) {
		return origin
	}
	for i := len(origin) - 1; i > 0; i-- {
		origin[i] = origin[i] - origin[i-1]
	}
	return origin
}

//////////////////////////////
func (tx *UTXOTransaction) GetInputKeyImages() []*types.Key {
	keyImages := make([]*types.Key, 0, len(tx.Inputs))
	for _, txin := range tx.Inputs {
		switch input := txin.(type) {
		case *UTXOInput:
			keyImages = append(keyImages, &input.KeyImage)
		case *AccountInput:
		default:
		}
	}
	return keyImages
}

func (tx UTXOTransaction) GetOutputData(blockHeight uint64) []*UTXOOutputData {
	utxoOutputs := make([]*UTXOOutputData, 0, len(tx.Outputs))
	tmpOutput := make([]Output, 0, len(tx.Outputs))
	log.Debug("GetOutputData:", "len(outputs)", len(tx.Outputs))

	for _, output := range tx.Outputs {
		switch output.(type) {
		case *UTXOOutput:
			tmpOutput = append(tmpOutput, output)
		}
	}
	log.Debug("GetOutputData:", "len(utxoOutput)", len(tmpOutput))

	for idx, txout := range tmpOutput {
		switch output := txout.(type) {
		case *UTXOOutput:
			commitment := tx.RCTSig.OutPk[idx].Mask
			outputdata := &UTXOOutputData{
				OTAddr: output.OTAddr,
				Height: blockHeight,
				Commit: commitment,
				Remark: output.Remark,
			}
			utxoOutputs = append(utxoOutputs, outputdata)
		case *AccountInput:
		default:
		}
	}
	return utxoOutputs
}

func (tx *UTXOTransaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	rawTx, _ := ser.EncodeToBytes(tx)
	tx.size.Store(common.StorageSize(len(rawTx)))
	return common.StorageSize(len(rawTx))
}

func (tx *UTXOTransaction) SetSize(size uint64) {
	tx.size.Store(common.StorageSize(size))
}

func (tx *UTXOTransaction) checkRctSigData() error {
	utxoInNum := tx.utxoInNum.Load().(int)
	utxoOutNum := tx.utxoOutNum.Load().(int)

	if len(tx.RCTSig.P.MGs) != utxoInNum {
		return ErrCheckBadMGsSize
	}

	if len(tx.RCTSig.OutPk) != utxoOutNum || utxoOutNum > BULLETPROOF_MAX_OUTPUTS {
		return ErrOutSkSizeNotMatch
	}

	if utxoOutNum > 0 {
		log.Debug("checkRctSigData", "len Bulletproofs", len(tx.RCTSig.P.Bulletproofs), "utxoOutNum", utxoOutNum)
		if len(tx.RCTSig.P.Bulletproofs) != 1 {
			log.Warn("bad bulletproofs size in tx", "hash", tx.Hash())
			return ErrCheckBadBulletproofSize
		}

		if len(tx.RCTSig.P.Bulletproofs[0].L) < 6 {
			log.Warn("bad bulletproofs L size in tx", "hash", tx.Hash())
			return ErrCheckBadBulletproofLSize
		}

		maxOutPuts := 1 << uint32(len(tx.RCTSig.P.Bulletproofs[0].L)-6)
		if maxOutPuts < utxoOutNum {
			log.Warn("bad bulletproofs max outputs in tx", "hash", tx.Hash())
			return ErrCheckBadBulletproofLSize
		}

		tx.RCTSig.P.Bulletproofs[0].V = make(types.KeyV, utxoOutNum)
		for i := 0; i < utxoOutNum; i++ {
			var err error
			if tx.RCTSig.P.Bulletproofs[0].V[i], err = ringct.ScalarmultKey(tx.RCTSig.OutPk[i].Mask, ringct.INV_EIGHT); err != nil {
				return err
			}
		}
	}
	return nil
}

func (tx *UTXOTransaction) checkTxSemantic(censor TxCensor) error {
	if len(tx.Inputs) <= 0 {
		return ErrCheckNoInput
	}
	if !common.IsLKC(tx.TokenID) {
		return fmt.Errorf("thirdparty token not support yet!")
	}

	utxoInNum := 0
	utxoOutNum := 0
	accountInNum := 0

	var kind UTXOKind
	ki := make(map[types.Key]bool)
	mixin := math.MaxInt32
	for _, in := range tx.Inputs {
		switch input := in.(type) {
		case *UTXOInput:
			if ki[input.KeyImage] {
				return ErrCheckDupKeyImage
			}
			ki[input.KeyImage] = true

			for n := 1; n < len(input.KeyOffset); n++ {
				if input.KeyOffset[n] == 0 {
					return ErrCheckDupRingMember
				}
			}

			if len(input.KeyOffset)-1 < mixin {
				mixin = len(input.KeyOffset) - 1
			}

			retKey, err := ringct.ScalarmultKey(input.KeyImage, ringct.CurveOrder())
			if err != nil {
				return err
			}
			if retKey != ringct.Identity() {
				return ErrCheckKeyImageInvalid
			}

			kind |= Uin
			utxoInNum = utxoInNum + 1
		case *AccountInput:
			log.Debug("found AccountInput", "txhash", tx.Hash())
			if (kind & Ain) == Ain {
				return ErrAccountInputSizeNotExpect
			}
			if input.Amount == nil || input.Amount.Cmp(big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)) < 0 ||
				big.NewInt(0).Mod(input.Amount, big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)).Sign() != 0 {
				return ErrMoneyInvalid
			}

			kind |= Ain
			accountInNum++
			tx.nonce = input.Nonce
		default:
			return ErrInputTypeNotExpect
		}
	}

	contractAddrCount := 0
	normalAddrCount := 0
	hasOneAccountOutput := false
	for _, out := range tx.Outputs {
		switch output := out.(type) {
		case *UTXOOutput:
			outaddr := types.PublicKey(output.OTAddr)
			if !crypto.CheckKey(&outaddr) {
				return ErrCheckSecAddr
			}
			kind |= Uout
			utxoOutNum = utxoOutNum + 1
		case *AccountOutput:
			log.Debug("found AccountOutput", "hash", tx.Hash(), "amount", output.Amount)
			if output.Amount == nil || output.Amount.Sign() < 0 {
				return ErrMoneyInvalid
			}
			if output.Amount.Sign() > 0 && (output.Amount.Cmp(big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)) < 0 ||
				big.NewInt(0).Mod(output.Amount, big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)).Sign() != 0) {
				return ErrMoneyInvalid
			}
			if hasOneAccountOutput {
				return ErrAccountOutputTooMore
			}
			hasOneAccountOutput = true

			censor.LockState()
			if censor.State().IsContract(output.To) {
				contractAddrCount++
			} else {
				if output.Amount.Sign() == 0 {
					censor.UnlockState()
					return ErrMoneyInvalid
				}
				normalAddrCount++
			}
			censor.UnlockState()
			kind |= Aout
		default:
			return ErrOutputTypeNotExpect
		}
	}

	if (normalAddrCount > 0 && contractAddrCount > 0) || contractAddrCount >= 1 { // multi account output only support normal address
		return ErrCheckAccountOutputsIllegal
	}
	if accountInNum > 0 && (hasOneAccountOutput || utxoInNum > 0) {
		//not support
		return ErrInputTypeNotExpect
	}

	if (kind&Uin) == Uin && mixin != 10 {
		log.Debug("checkTxSemantic", "min ring size", mixin+1)
		//return false  //TODO current version no limit mixin
	}

	if tx.Fee == nil || tx.Fee.Sign() < 0 {
		return ErrUtxoTxFeeIllegal
	}

	feeModGas := big.NewInt(0).Mod(tx.Fee, big.NewInt(0).SetInt64(ParGasPrice))
	if feeModGas.Sign() != 0 {
		log.Warn("Fee not illegal, must be mutiple of GasPrice", "txhash", tx.Hash(), "Fee", tx.Fee)
		return ErrUtxoTxFeeIllegal
	}

	log.Debug("UTXOKind", "txhash", tx.Hash(), "kind", kind, "normalAddrCount", normalAddrCount, "contractAddrCount", contractAddrCount)
	if (kind&Ain) == Ain || contractAddrCount > 0 {
		//AccountInput or call contract need account signature
		fromAddr, err := tx.From()
		if err != nil {
			return ErrInvalidSig
		}
		tx.StoreFrom(fromAddr)
	} else {
		tx.StoreFrom(common.EmptyAddress)
	}
	tx.kind.Store(kind)
	tx.utxoInNum.Store(utxoInNum)
	tx.utxoOutNum.Store(utxoOutNum)
	return nil
}

func (tx *UTXOTransaction) VerifyProofSemantic() error {
	ret, err := ringct.VerBulletproof(&tx.RCTSig.P.Bulletproofs[0])
	if err != nil {
		return err
	}

	if !ret {
		return ErrVerRangeBulletproofFailed
	}
	return nil
}

func D2h(amount *big.Int) types.Key {
	amounth := types.Key{}
	val := amount
	i := 0
	for val.Cmp(new(big.Int).SetUint64(0)) != 0 && i <= 32 {
		d := new(big.Int).Mod(val, new(big.Int).SetUint64(256)).Bytes()
		if len(d) == 0 {
			amounth[i] = 0
		} else {
			amounth[i] = d[0]
		}
		val = val.Div(val, new(big.Int).SetUint64(256))
		i++
	}
	return amounth
}

//AmountCommit calc c = aG + bH
func AmountCommit(amount *big.Int, cf types.Key) types.Key {
	c := ringct.ScalarmultBase(cf)
	am, _ := BigInt2Hash(amount)
	bH := ringct.ScalarmultH(am)
	retKey, _ := ringct.AddKeys(c, bH)
	return retKey
}

func (tx *UTXOTransaction) checkCommitEqual() error {
	var err error
	var inputCommits types.KeyV
	if (tx.UTXOKind() & Ain) == Ain {
		for _, txin := range tx.Inputs {
			switch input := txin.(type) {
			case *UTXOInput:
			case *AccountInput:
				commit := AmountCommit(big.NewInt(0).Div(input.Amount, big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)), input.CF)
				if !commit.IsEqual(&input.Commit) {
					return ErrCheckAmountCommit
				}
				inputCommits = append(inputCommits, commit)
			default:
			}
		}
	}
	// log.Debug("checkCommitEqual", "inputCommits", len(inputCommits), "PseudoOuts", len(tx.RCTSig.P.PseudoOuts), "OutPks", len(tx.RCTSig.OutPk))
	inputCommits = append(inputCommits, tx.RCTSig.P.PseudoOuts...)
	if len(inputCommits) <= 0 {
		log.Warn("inputCommits Empty")
		return ErrCheckEmptyInputCommits
	}

	sumPseudoOuts := types.Key{}
	if len(inputCommits) == 1 {
		sumPseudoOuts = inputCommits[0]
	} else {
		sumPseudoOuts, err = ringct.AddKeyV(inputCommits)
		if err != nil {
			log.Warn("AddKeyV Error")
			return err
		}
	}

	//Output
	sumOutpks := types.Key{}
	mask := make([]types.Key, 0)
	for _, out := range tx.RCTSig.OutPk {
		mask = append(mask, out.Mask)
	}

	if len(mask) != 0 {
		sumOutpks, err = ringct.AddKeyV(mask)
		if err != nil {
			log.Warn("AddKeyV Error")
			return err
		}
	}

	if common.IsLKC(tx.TokenID) {
		txFeekey, err := BigInt2Hash(big.NewInt(0).Div(tx.Fee, big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)))
		if err != nil {
			log.Warn("UTXO BigInt2Hash Error")
			return err
		}
		TxFeeKey := ringct.ScalarmultH(txFeekey)
		if err != nil {
			log.Error("UTXO TxFee to Key Error")
			return err
		}
		emptyKey := types.Key{}
		if sumOutpks == emptyKey {
			sumOutpks = TxFeeKey
		} else {
			sumOutpks, _ = ringct.AddKeys(sumOutpks, TxFeeKey)
		}
	}

	if (tx.UTXOKind() & Aout) == Aout {
		for _, txin := range tx.Outputs {
			switch output := txin.(type) {
			case *UTXOOutput:
			case *AccountOutput:
				oAmountKey, err := BigInt2Hash(big.NewInt(0).Div(output.Amount, big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)))
				if err != nil {
					log.Warn("Amount bigInt2hash Error")
					return err
				}
				commit := ringct.ScalarmultH(oAmountKey)
				if !commit.IsEqual(&output.Commit) {
					log.Warn("AccountOut AmountCommit Not Equal")
					return ErrCheckAmountCommit
				}
				sumOutpks, _ = ringct.AddKeys(sumOutpks, output.Commit)
			default:
			}
		}
	}

	if sumPseudoOuts.IsEqual(&types.Key{}) || sumOutpks.IsEqual(&types.Key{}) {
		log.Warn("sum commits empty")
		return fmt.Errorf("sum of commit illegal")
	}

	if !sumPseudoOuts.IsEqual(&sumOutpks) {
		log.Warn("Sum Commit Not Equal", "In", sumPseudoOuts, "Out", sumOutpks)
		return ErrCheckInOutCommitNotEqual
	}
	return nil
}

func relativeOutputOffsetsToAbsolute(off []uint64) []uint64 {
	res := make([]uint64, len(off))
	copy(res, off)
	for i := 1; i < len(res); i++ {
		res[i] += res[i-1]
	}
	return res
}

func (tx *UTXOTransaction) checkTxInputKeys(censor TxCensor) error {
	pubkeys := make([][]types.Ctkey, len(tx.Inputs))
	var ok bool
	for i, txin := range tx.Inputs {
		switch input := txin.(type) {
		case *UTXOInput:
			pubkeys[i], ok = getTxInputKeys(censor.BlockChain(), censor.UTXOStore(), input, tx.TokenID)
			if !ok {
				log.Warn("getTxInputKeys failed")
				return ErrGetInputFromDB
			}
		}
	}
	tx.expandTransactionRctSig(pubkeys)
	if err := tx.checkRingctSignatures(pubkeys); err != nil {
		log.Warn("checkRingctSignatures failed", "err", err)
		return err
	}

	return nil
}

func getTxInputKeys(blockchain BlockChain, utxoStore UTXOStore, input *UTXOInput, tokenID common.Address) ([]types.Ctkey, bool) {
	if len(input.KeyOffset) <= 0 {
		return nil, false
	}

	absoluteOffset := relativeOutputOffsetsToAbsolute(input.KeyOffset)
	//TODO cache
	outputs, err := utxoStore.GetUtxoOutputs(absoluteOffset, tokenID)
	if err != nil {
		log.Warn("GetUtxoOutputs", "err", err)
		return nil, false
	}
	if len(absoluteOffset) != len(outputs) {
		log.Warn("Some outputs do not exist", "len(absoluteOffset)", len(absoluteOffset), "absoluteOffset", absoluteOffset, "len(outputs)", len(outputs))
		return nil, false
	}

	outputKeys := make([]types.Ctkey, 0)
	for _, output := range outputs {
		if output.TokenID != tokenID {
			log.Warn("utxo tokenID not match", "output", output, "tokenID", tokenID)
			return nil, false
		}
		outputKeys = append(outputKeys, types.Ctkey{types.Key(output.OTAddr), output.Commit})
	}

	return outputKeys, true
}

//fill Message MixRing Mgs
func (tx *UTXOTransaction) expandTransactionRctSig(pubkeys [][]types.Ctkey) {
	if len(pubkeys) == 0 || len(pubkeys[0]) == 0 {
		log.Warn("expandTransactionRctSig empty pubkeys")
		return
	}

	tx.RCTSig.Message = tx.PrefixHash()
	tx.RCTSig.MixRing = make(types.CtkeyM, len(pubkeys))
	for i := 0; i < len(pubkeys); i++ {
		tx.RCTSig.MixRing[i] = make(types.CtkeyV, len(pubkeys[i]))
		tx.RCTSig.MixRing[i] = pubkeys[i]
	}

	for i, txin := range tx.Inputs {
		switch input := txin.(type) {
		case *UTXOInput:
			tx.RCTSig.P.MGs[i].II = make(types.KeyV, 1)
			tx.RCTSig.P.MGs[i].II[0] = input.KeyImage
		}
	}
}

func (tx *UTXOTransaction) checkRingctSignatures(pubkeys [][]types.Ctkey) error {
	if len(tx.RCTSig.MixRing) != len(pubkeys) {
		return ERRCheckInvalidMixRing
	}
	for i := 0; i < len(pubkeys); i++ {
		if len(pubkeys[i]) != len(tx.RCTSig.MixRing[i]) {
			return ERRCheckInvalidMixRing
		}
	}

	//short ring member
	if len(tx.RCTSig.MixRing) > 0 && len(tx.RCTSig.MixRing[0]) == SHORT_RING_MEMBER_NUM {
		for i := 1; i < len(tx.RCTSig.MixRing); i++ {
			if len(tx.RCTSig.MixRing[0]) > 0 && len(tx.RCTSig.MixRing[0]) != SHORT_RING_MEMBER_NUM {
				return ErrMixRingMemberNotSupport
			}
		}
		hash, err := ringct.GetPreMlsagHash(&tx.RCTSig)
		if err != nil {
			return err
		}
		var wg sync.WaitGroup
		errs := make([]error, len(tx.Inputs))
		for i := 0; i < len(tx.Inputs); i++ {
			wg.Add(1)
			go func(idx int) {
				input, ok := tx.Inputs[idx].(*UTXOInput)
				if !ok {
					wg.Done()
					return
				}
				pubs := make([]types.PublicKey, 0)
				for _, ringMem := range tx.RCTSig.MixRing[idx] {
					pubs = append(pubs, types.PublicKey(ringMem.Dest))
				}
				if idx >= len(tx.RCTSig.P.Ss) {
					errs[idx] = ErrRingCTSignaturesInvalid
					wg.Done()
					return
				}
				if !xcrypto.CheckRingSignature(types.Hash(hash), types.KeyImage(input.KeyImage), pubs, &tx.RCTSig.P.Ss[idx]) {
					errs[idx] = ErrVerRingCTSignatures
				}
				wg.Done()
			}(i)
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				return err
			}
		}
		return nil
	}

	if !ringct.VerRctNonSemanticsSimple(&tx.RCTSig) {
		return ErrVerRingCTSignatures
	}

	return nil
}

//CheckStoreState check an UTXOTransaction stroe state
func (tx *UTXOTransaction) CheckStoreState(censor TxCensor, state State) error {
	aggInputAmount := big.NewInt(0)

	for _, txin := range tx.Inputs {
		switch input := txin.(type) {
		case *UTXOInput:
			if censor.UTXOStore().HaveTxKeyimgAsSpent(&input.KeyImage) {
				log.Debug("Key image already spent in blockchain", "KeyImage", input.KeyImage, "hash", tx.Hash())
				return ErrUtxoTxDoubleSpend
			}
		case *AccountInput:
			//check nonce
			fromAddr, err := tx.From()
			if err != nil {
				return err
			}
			nonce := state.GetNonce(fromAddr)
			if nonce > input.Nonce {
				log.Debug("nonce too low", "got", input.Nonce, "want", nonce, "fromAddr", fromAddr, "txHash", tx.Hash())
				return ErrNonceTooLow
			} else if nonce < input.Nonce {
				return ErrNonceTooHigh
			}
			//check balance
			if state.GetTokenBalance(fromAddr, tx.TokenID).Cmp(input.Amount) < 0 {
				return ErrInsufficientFunds
			}
			if !common.IsLKC(tx.TokenID) { //other token Fee
				if state.GetBalance(fromAddr).Cmp(tx.Fee) < 0 {
					return ErrInsufficientFunds
				}
			}
			aggInputAmount.Add(aggInputAmount, input.Amount)
		default:
		}
	}
	accOutAmount := big.NewInt(0)
	for _, txout := range tx.Outputs {
		switch output := txout.(type) {
		case *AccountOutput:
			accOutAmount.Add(accOutAmount, output.Amount)
		default:
		}
	}

	neededGas := big.NewInt(0)
	if aggInputAmount.Sign() > 0 && aggInputAmount.Cmp(tx.Fee) > 0 {
		neededGas.SetUint64(CalNewAmountGas(aggInputAmount.Sub(aggInputAmount, tx.Fee), EverLiankeFee))
	}

	kind := tx.UTXOKind()
	if (kind & Uin) == Uin {
		if accOutAmount.Sign() > 0 {
			neededGas.Add(neededGas, big.NewInt(0).SetUint64(CalNewAmountGas(accOutAmount, EverLiankeFee)))
		}
		if (kind & Uout) == Uout {
			utxoGas := censor.GetUTXOGas()
			neededGas.Add(neededGas, big.NewInt(0).SetUint64(utxoGas))
		}
	}

	neededFee := neededGas.Mul(neededGas, big.NewInt(0).SetInt64(ParGasPrice))
	if tx.Fee.Cmp(neededFee) < 0 {
		log.Warn("checkFee insufficient", "txhash", tx.Hash(), "Fee", tx.Fee, "neededFee", neededFee)
		return ErrUtxoTxFeeTooLow
	}

	return nil
}

func (tx *UTXOTransaction) checkState(censor TxCensor) error {
	censor.LockState()
	defer censor.UnlockState()

	aggInputAmount := big.NewInt(0)
	keyImages := make([]types.Key, 0, len(tx.Inputs))

	for _, txin := range tx.Inputs {
		switch input := txin.(type) {
		case *UTXOInput:
			if censor.UTXOStore().HaveTxKeyimgAsSpent(&input.KeyImage) {
				log.Debug("Key image already spent in blockchain", "KeyImage", input.KeyImage, "hash", tx.Hash())
				return ErrUtxoTxDoubleSpend
			}
			if censor.Mempool().KeyImageExists(input.KeyImage) {
				log.Debug("Key image already spent in other txs", "KeyImage", input.KeyImage, "hash", tx.Hash())
				return ErrUtxoTxDoubleSpend
			}
			keyImages = append(keyImages, input.KeyImage)

		case *AccountInput:
			//check nonce
			state := censor.State()
			fromAddr, _ := tx.From()
			nonce := state.GetNonce(fromAddr)
			if nonce > input.Nonce {
				log.Debug("nonce too low", "got", input.Nonce, "want", nonce, "fromAddr", fromAddr, "txHash", tx.Hash())
				return ErrNonceTooLow
			} else if nonce < input.Nonce {
				return ErrNonceTooHigh
			}
			//check balance
			if state.GetTokenBalance(fromAddr, tx.TokenID).Cmp(input.Amount) < 0 {
				return ErrInsufficientFunds
			}
			if !common.IsLKC(tx.TokenID) { //other token Fee
				if state.GetBalance(fromAddr).Cmp(tx.Fee) < 0 {
					return ErrInsufficientFunds
				}
			}
			aggInputAmount.Add(aggInputAmount, input.Amount)
			state.SubTokenBalance(fromAddr, tx.TokenID, input.Amount)
			if !common.IsLKC(tx.TokenID) {
				state.SubBalance(fromAddr, tx.Fee) //subBalance here, support only one accountinput
			}

			state.SetNonce(fromAddr, input.Nonce+1)
		default:
		}
	}
	accOutAmount := big.NewInt(0)
	for _, txout := range tx.Outputs {
		switch output := txout.(type) {
		case *AccountOutput:
			accOutAmount.Add(accOutAmount, output.Amount)
		default:
		}
	}

	neededGas := big.NewInt(0)
	if aggInputAmount.Sign() > 0 && aggInputAmount.Cmp(tx.Fee) > 0 {
		neededGas.SetUint64(CalNewAmountGas(aggInputAmount.Sub(aggInputAmount, tx.Fee), EverLiankeFee))
	}

	kind := tx.UTXOKind()
	if (kind & Uin) == Uin {
		if accOutAmount.Sign() > 0 {
			neededGas.Add(neededGas, big.NewInt(0).SetUint64(CalNewAmountGas(accOutAmount, EverLiankeFee)))
		}
		if (kind & Uout) == Uout {
			utxoGas := censor.GetUTXOGas()
			neededGas.Add(neededGas, big.NewInt(0).SetUint64(utxoGas))
		}
	}

	neededFee := neededGas.Mul(neededGas, big.NewInt(0).SetInt64(ParGasPrice))
	if tx.Fee.Cmp(neededFee) < 0 {
		log.Warn("checkFee insufficient", "txhash", tx.Hash(), "Fee", tx.Fee, "neededFee", neededFee)
		return ErrUtxoTxFeeTooLow
	}

	for _, ki := range keyImages {
		if !censor.Mempool().KeyImagePush(ki) {
			log.Error("checkState KeyImagePush fail, please check!!!", "key", ki, "txhash", tx.Hash())
		}
	}
	return nil
}

// IsOutputBelongToAccount check if output belong to account
func IsOutputBelongToAccount(acc *types.AccountKey, keyIndex map[types.PublicKey]uint64, otaddr types.Key,
	deriKeys []types.KeyDerivation, index uint64) (types.KeyDerivation, uint64, error) {
	for _, deriKey := range deriKeys {
		subaddrIdx, err := isOutputBelongToAccount(acc, keyIndex, otaddr, deriKey, index)
		if err == nil {
			return deriKey, subaddrIdx, nil
		}
	}
	return types.KeyDerivation{}, 0, ErrOutputNotBelongToAccount
}

func d2h(amount uint64) types.Key {
	key := ringct.Z
	i := 0
	for amount != 0 {
		key[i] = byte(amount & 0xFF)
		i++
		amount = amount / 256
	}
	return key
}

//NewAinTransaction return a UTXOTransaction for account input only
//1 generate random key
//2 compute one-time address
//3 construct UTXOTransaction, erase output money
//4 compute RangeBulletproof, utxo commitment, account commitment
//5 compute account sig
func NewAinTransaction(accSource *AccountSourceEntry, dests []DestEntry, tokenID common.Address, extra []byte) (*UTXOTransaction, *types.Key, error) {
	rSecKey, rPubKey := xcrypto.SkpkGen()
	var utxoDests []*UTXODestEntry
	for _, dest := range dests {
		if TypeUTXODest == dest.Type() {
			utxoDests = append(utxoDests, dest.(*UTXODestEntry))
		}
	}
	utxoOuts, mKeys, err := GenerateOneTimeAddress(rSecKey, utxoDests)
	if err != nil {
		return nil, nil, err
	}
	additionalKeys, err := GenerateAdditionalKeys(rSecKey, utxoDests)
	if err != nil {
		return nil, nil, err
	}
	utxoTrans, err := constructAinTrans(rPubKey, accSource, dests, utxoOuts, additionalKeys, tokenID, extra)
	if err != nil {
		return nil, nil, err
	}
	err = aInTransWithRctSig(utxoTrans, dests, mKeys)
	if err != nil {
		return nil, nil, err
	}
	return utxoTrans, &rSecKey, nil
}

func constructAinTrans(rPubKey types.Key, source *AccountSourceEntry, dests []DestEntry, utxoOuts []*UTXOOutput,
	additionalKeys []types.PublicKey, tokenID common.Address, extra []byte) (*UTXOTransaction, error) {
	utxoTrans := &UTXOTransaction{
		Outputs: make([]Output, len(dests)),
		RKey:    types.PublicKey(rPubKey),
		AddKeys: additionalKeys,
		TokenID: tokenID,
	}
	if extra != nil {
		utxoTrans.Extra = make([]byte, len(extra))
		copy(utxoTrans.Extra, extra[:])
	}
	var (
		inAmount  = big.NewInt(0)
		outAmount = big.NewInt(0)
	)
	input := &AccountInput{
		Amount: big.NewInt(0).Set(source.Amount),
		Nonce:  source.Nonce,
	}
	utxoTrans.Inputs = append(utxoTrans.Inputs, input)
	inAmount.Add(inAmount, source.Amount)
	n := 0
	for i, dest := range dests {
		if TypeUTXODest == dest.Type() {
			if n >= len(utxoOuts) {
				return nil, ErrUtxoOutSizeNotExpect
			}
			utxoTrans.Outputs[i] = &UTXOOutput{
				OTAddr: utxoOuts[n].OTAddr,
				Amount: big.NewInt(0),
				Remark: dest.(*UTXODestEntry).Remark,
			}
			n++
		} else {
			utxoTrans.Outputs[i] = &AccountOutput{
				To:     dest.(*AccountDestEntry).To,
				Amount: dest.GetAmount(),
				Data:   dest.(*AccountDestEntry).Data,
			}
		}
		outAmount.Add(outAmount, dest.GetAmount())
	}
	if inAmount.Cmp(outAmount) < 0 {
		return nil, ErrInMoneyLessThanOutMoney
	}
	utxoTrans.Fee = big.NewInt(0).Sub(inAmount, outAmount)
	return utxoTrans, nil
}

func aInTransWithRctSig(utxoTrans *UTXOTransaction, dests []DestEntry, mkeys types.KeyV) error {
	outAmounts := make([]types.Key, 0)
	for _, dest := range dests {
		if TypeUTXODest == dest.Type() {
			amountKey, err := BigInt2Hash(big.NewInt(0).Div(dest.GetAmount(), big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)))
			if err != nil {
				return err
			}
			outAmounts = append(outAmounts, amountKey)
		}
	}
	if len(outAmounts) != len(mkeys) {
		return ErrOutsAndMkeysNotMatch
	}
	n := 0
	for _, output := range utxoTrans.Outputs {
		if OutUTXO == output.Type() {
			utxoOut := output.(*UTXOOutput)
			if n >= len(mkeys) {
				return ErrOutsAndMkeysNotMatch
			}
			hash := lcrypto.Sha256(mkeys[n][:])
			for i := 0; i < 32; i++ {
				utxoOut.Remark[i] ^= hash[i]
			}
			n++
		}
	}
	sumOutCF := ringct.Z
	if len(outAmounts) > 0 {
		proof, commits, masks, err := ringct.ProveRangeBulletproof(outAmounts, mkeys)
		if err != nil {
			return ErrProveRangeBulletproof
		}
		proof.V = nil
		utxoTrans.RCTSig.P.Bulletproofs = append(utxoTrans.RCTSig.P.Bulletproofs, *proof)
		utxoTrans.RCTSig.OutPk = make(types.CtkeyV, len(outAmounts))
		utxoTrans.RCTSig.EcdhInfo = make([]types.EcdhTuple, len(outAmounts))
		for i := 0; i < len(outAmounts); i++ {
			sumOutCF = ringct.ScAdd(types.EcScalar(masks[i]), types.EcScalar(sumOutCF))
			utxoTrans.RCTSig.OutPk[i].Mask, _ = ringct.Scalarmult8(commits[i])
			utxoTrans.RCTSig.EcdhInfo[i].Mask = masks[i]
			utxoTrans.RCTSig.EcdhInfo[i].Amount = outAmounts[i]
			ok := ringct.EcdhEncode(&utxoTrans.RCTSig.EcdhInfo[i], mkeys[i], false)
			if !ok {
				return ErrEcdhEncode
			}
		}
	}
	for i, output := range utxoTrans.Outputs {
		if OutAc == output.Type() {
			amountKey, err := BigInt2Hash(big.NewInt(0).Div(output.(*AccountOutput).Amount, big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)))
			if err != nil {
				return err
			}
			utxoTrans.Outputs[i].(*AccountOutput).Commit = ringct.ScalarmultH(amountKey)
		}
	}
	if len(utxoTrans.Inputs) != 1 {
		return ErrAccountInputSizeNotExpect
	}
	if InAc != utxoTrans.Inputs[0].Type() {
		return ErrInputTypeNotExpect
	}
	amountKey, err := BigInt2Hash(big.NewInt(0).Div(utxoTrans.Inputs[0].(*AccountInput).Amount, big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)))
	if err != nil {
		return err
	}
	utxoTrans.Inputs[0].(*AccountInput).CF = sumOutCF
	utxoTrans.Inputs[0].(*AccountInput).Commit, _ = ringct.AddKeys2(sumOutCF, amountKey, ringct.H)
	return nil
}

func BigInt2Hash(amount *big.Int) (types.Key, error) {
	if amount.Sign() < 0 {
		return types.Key{}, ErrMoneyInvalid
	}
	var (
		i      = 0
		value  = big.NewInt(0).Set(amount)
		big0   = big.NewInt(0)
		big256 = big.NewInt(256)
		key    types.Key
	)
	//amount only support 8 bytes, 2^64
	for value.Sign() > 0 && i < 8 {
		key[i] = byte(big0.Mod(value, big256).Int64())
		value.Div(value, big256)
		i++
	}
	if value.Sign() > 0 && i == 8 {
		return types.Key{}, ErrMoneyInvalid
	}
	return key, nil
}

func Hash2BigInt(key types.Key) *big.Int {
	var (
		find   = false
		big256 = big.NewInt(256)
	)
	amount := big.NewInt(0)
	for i := 31; i >= 0; i-- {
		if !find && 0x00 == key[i] {
			continue
		}
		if !find {
			find = true
		}
		amount.Mul(amount, big256)
		amount.Add(amount, big.NewInt(int64(key[i])))
	}
	return amount
}

//NewUinTransaction return a UTXOTransaction for utxo input only
//1 generate random key
//2 compute input secret key and key_image
//3 compute one-time address
//4 construct UTXOTransaction, erase input and output money
//5 compute RangeBulletproof, utxo commitment, account commitment, ring signature
func NewUinTransaction(acc *types.AccountKey, keyIndex map[types.PublicKey]uint64, utxoSources []*UTXOSourceEntry,
	dests []DestEntry, tokenID common.Address, refundAddr common.Address, extra []byte) (*UTXOTransaction, []*UTXOInputEphemeral, types.KeyV, *types.Key, error) {
	rSecKey, rPubKey := xcrypto.SkpkGen()
	utxoInEphs, err := GenerateKeyImage(acc, keyIndex, utxoSources)
	if err != nil {
		return nil, nil, types.KeyV{}, nil, err
	}
	var utxoDests []*UTXODestEntry
	for _, dest := range dests {
		if TypeUTXODest == dest.Type() {
			utxoDests = append(utxoDests, dest.(*UTXODestEntry))
		}
	}
	utxoOuts, mKeys, err := GenerateOneTimeAddress(rSecKey, utxoDests)
	if err != nil {
		return nil, nil, types.KeyV{}, nil, err
	}
	additionalKeys, err := GenerateAllAdditionalKeys(rSecKey, dests)
	if err != nil {
		return nil, nil, types.KeyV{}, nil, err
	}
	utxoTrans, err := constructUinTrans(rPubKey, utxoSources, utxoInEphs, dests, utxoOuts, additionalKeys, mKeys, tokenID, refundAddr, extra)
	if err != nil {
		return nil, nil, types.KeyV{}, nil, err
	}
	return utxoTrans, utxoInEphs, mKeys, &rSecKey, nil
}

func constructUinTrans(rPubKey types.Key, sources []*UTXOSourceEntry, utxoIns []*UTXOInputEphemeral, dests []DestEntry, utxoOuts []*UTXOOutput,
	additionalKeys []types.PublicKey, mkeys types.KeyV, tokenID common.Address, refundAddr common.Address, extra []byte) (*UTXOTransaction, error) {
	utxoTrans := &UTXOTransaction{
		Inputs:  make([]Input, len(sources)),
		Outputs: make([]Output, len(dests)),
		RKey:    types.PublicKey(rPubKey),
		AddKeys: additionalKeys,
		TokenID: tokenID,
	}
	log.Debug("constructUinTrans", "len(utxoTrans.AddKeys)", len(utxoTrans.AddKeys))
	if extra != nil {
		utxoTrans.Extra = make([]byte, len(extra))
		copy(utxoTrans.Extra, extra[:])
	}
	var (
		inAmount  = big.NewInt(0)
		outAmount = big.NewInt(0)
	)
	for i := 0; i < len(sources); i++ {
		in := &UTXOInput{
			KeyImage: utxoIns[i].KeyImage,
		}
		in.KeyOffset = make([]uint64, len(sources[i].Ring))
		for j := 0; j < len(sources[i].Ring); j++ {
			in.KeyOffset[j] = sources[i].Ring[j].Index
		}
		in.KeyOffset = absoluteOffsetsToRelative(in.KeyOffset)
		utxoTrans.Inputs[i] = in
		inAmount.Add(inAmount, sources[i].Amount)
	}
	n := 0
	for i, dest := range dests {
		if TypeUTXODest == dest.Type() {
			if n >= len(utxoOuts) {
				return nil, ErrUtxoOutSizeNotExpect
			}
			utxoTrans.Outputs[i] = &UTXOOutput{
				OTAddr: utxoOuts[n].OTAddr,
				Amount: big.NewInt(0),
				Remark: dest.(*UTXODestEntry).Remark,
			}
			n++
		} else {
			utxoTrans.Outputs[i] = &AccountOutput{
				To:     dest.(*AccountDestEntry).To,
				Amount: dest.GetAmount(),
				Data:   dest.(*AccountDestEntry).Data,
			}
		}
		outAmount.Add(outAmount, dest.GetAmount())
	}
	n = 0
	for _, output := range utxoTrans.Outputs {
		if OutUTXO == output.Type() {
			utxoOut := output.(*UTXOOutput)
			if n >= len(mkeys) {
				return nil, ErrOutsAndMkeysNotMatch
			}
			hash := lcrypto.Sha256(mkeys[n][:])
			for i := 0; i < 32; i++ {
				utxoOut.Remark[i] ^= hash[i]
			}
			n++
		} else {
			accOutput := output.(*AccountOutput)
			amountKey, err := BigInt2Hash(big.NewInt(0).Div(accOutput.Amount, big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)))
			if err != nil {
				return nil, err
			}
			accOutput.Commit = ringct.ScalarmultH(amountKey)
		}
	}
	if inAmount.Cmp(outAmount) < 0 {
		return nil, ErrInMoneyLessThanOutMoney
	}
	utxoTrans.Fee = big.NewInt(0).Sub(inAmount, outAmount)
	return utxoTrans, nil
}

func UInTransWithRctSig(utxoTrans *UTXOTransaction, sources []*UTXOSourceEntry, utxoIns []*UTXOInputEphemeral,
	dests []DestEntry, mkeys types.KeyV) error {
	outAmounts := make([]types.Key, 0)
	for _, dest := range dests {
		if TypeUTXODest == dest.Type() {
			amountKey, err := BigInt2Hash(big.NewInt(0).Div(dest.GetAmount(), big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)))
			if err != nil {
				return err
			}
			outAmounts = append(outAmounts, amountKey)
		}
	}
	if len(outAmounts) != len(mkeys) {
		return ErrOutsAndMkeysNotMatch
	}
	sumOutCF := ringct.Z
	if len(outAmounts) > 0 {
		proof, commits, masks, err := ringct.ProveRangeBulletproof(outAmounts, mkeys)
		if err != nil {
			return ErrProveRangeBulletproof
		}
		proof.V = nil
		utxoTrans.RCTSig.P.Bulletproofs = append(utxoTrans.RCTSig.P.Bulletproofs, *proof)
		utxoTrans.RCTSig.OutPk = make(types.CtkeyV, len(outAmounts))
		utxoTrans.RCTSig.EcdhInfo = make([]types.EcdhTuple, len(outAmounts))
		for i := 0; i < len(outAmounts); i++ {
			sumOutCF = ringct.ScAdd(types.EcScalar(masks[i]), types.EcScalar(sumOutCF))
			utxoTrans.RCTSig.OutPk[i].Mask, _ = ringct.Scalarmult8(commits[i])
			utxoTrans.RCTSig.EcdhInfo[i].Mask = masks[i]
			utxoTrans.RCTSig.EcdhInfo[i].Amount = outAmounts[i]
			ok := ringct.EcdhEncode(&utxoTrans.RCTSig.EcdhInfo[i], mkeys[i], false)
			if !ok {
				return ErrEcdhEncode
			}
		}
	}
	var (
		inSKey    = make(types.CtkeyV, len(sources))
		rings     = make(types.CtkeyM, len(sources))
		indexs    = make([]uint32, len(sources))
		inAmounts = make([]types.Key, len(sources))
	)
	for i := 0; i < len(sources); i++ {
		ctKey := types.Ctkey{
			Dest: types.Key(utxoIns[i].SKey),
			Mask: sources[i].Mask,
		}
		inSKey[i] = ctKey
		indexs[i] = uint32(sources[i].RingIndex)
		rings[i] = make(types.CtkeyV, len(sources[i].Ring))
		for j := 0; j < len(sources[i].Ring); j++ {
			rings[i][j] = types.Ctkey{
				Dest: sources[i].Ring[j].OTAddr,
				Mask: sources[i].Ring[j].Commit,
			}
		}
		amountKey, err := BigInt2Hash(big.NewInt(0).Div(sources[i].Amount, big.NewInt(UTXO_COMMITMENT_CHANGE_RATE)))
		if err != nil {
			return err
		}
		inAmounts[i] = amountKey
	}
	utxoTrans.RCTSig.Type = uint8(types.RCTTypeBulletproof)
	utxoTrans.RCTSig.Message = utxoTrans.PrefixHash()
	utxoTrans.RCTSig.MixRing = rings
	utxoTrans.RCTSig.P.PseudoOuts = make(types.KeyV, len(sources))
	utxoTrans.RCTSig.P.MGs = make([]types.MgSig, len(sources))
	utxoTrans.RCTSig.P.Ss = make([]types.Signature, len(sources))
	ra := make(types.KeyV, len(sources))
	sumInCF := ringct.Z
	n := 0
	for n = 0; n < len(sources)-1; n++ {
		ra[n] = ringct.SkGen()
		sumInCF = ringct.ScAdd(types.EcScalar(ra[n]), types.EcScalar(sumInCF))
		utxoTrans.RCTSig.P.PseudoOuts[n], _ = ringct.AddKeys2(ra[n], inAmounts[n], ringct.H)
	}
	ra[n] = ringct.ScSub(types.EcScalar(sumOutCF), types.EcScalar(sumInCF))
	utxoTrans.RCTSig.P.PseudoOuts[n], _ = ringct.AddKeys2(ra[n], inAmounts[n], ringct.H)
	hash, err := ringct.GetPreMlsagHash(&utxoTrans.RCTSig)
	if err != nil {
		return err
	}
	isShortRing := false
	if len(sources) > 0 && len(sources[0].Ring) == SHORT_RING_MEMBER_NUM {
		isShortRing = true
		for i := 1; i < len(sources); i++ {
			if len(sources[i].Ring) != SHORT_RING_MEMBER_NUM {
				return ErrMixRingMemberNotSupport
			}
		}
	}
	for i := 0; i < len(sources); i++ {
		if isShortRing {
			pubs := []types.PublicKey{types.PublicKey(utxoIns[i].OTAddr)}
			ssig, err := xcrypto.GenerateRingSignature(types.Hash(hash), types.KeyImage(utxoIns[i].KeyImage), pubs, utxoIns[i].SKey, 0)
			if err != nil {
				return err
			}
			utxoTrans.RCTSig.P.Ss[i] = *ssig
		} else {
			mgSig, err := ringct.ProveRctMGSimple(hash, rings[i], inSKey[i], ra[i], utxoTrans.RCTSig.P.PseudoOuts[i], nil, nil, indexs[i])
			if err != nil {
				return err
			}
			utxoTrans.RCTSig.P.MGs[i] = *mgSig
		}
	}
	return nil
}

//GenerateAdditionalKeys return random keys for subaddr
func GenerateAdditionalKeys(seckey types.Key, dests []*UTXODestEntry) ([]types.PublicKey, error) {
	keys := make([]types.PublicKey, 0)
	for _, dest := range dests {
		pkey, err := xcrypto.ScalarmultKey(types.Key(dest.Addr.SpendPublicKey), seckey)
		if err != nil {
			return nil, err
		}
		keys = append(keys, types.PublicKey(pkey))
		log.Debug("GenerateAdditionalKeys", "key", fmt.Sprintf("%x", keys[len(keys)-1]))
	}
	return keys, nil
}

//GenerateAllAdditionalKeys return random keys for all dests
func GenerateAllAdditionalKeys(seckey types.Key, dests []DestEntry) ([]types.PublicKey, error) {
	var (
		pkey types.PublicKey
		keys = make([]types.PublicKey, 0)
	)
	for i, dest := range dests {
		if TypeUTXODest == dest.Type() {
			spubKey := dest.(*UTXODestEntry).Addr.SpendPublicKey
			key, err := xcrypto.ScalarmultKey(types.Key(spubKey), seckey)
			if err != nil {
				return nil, err
			}
			pkey = types.PublicKey(key)
		} else {
			to := dest.(*AccountDestEntry).To
			scalar, err := xcrypto.DerivationToScalar(types.KeyDerivation(seckey), i)
			if err != nil {
				return nil, err
			}
			data := make([]byte, types.COMMONLEN+common.AddressLength)
			copy(data[0:], scalar[:])
			copy(data[len(data):], to[:])
			key := lcrypto.Sha256(data)
			copy(pkey[:], key[:])
		}
		keys = append(keys, pkey)
	}
	return keys, nil
}
