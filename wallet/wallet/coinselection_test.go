package wallet

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	"github.com/lianxiangcloud/linkchain/libs/common"
	lkctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	lktypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/types"
)

var (
	coinWallet *Wallet
	coinTests  = make(map[string][]CoinTest)
)

type CoinTest struct {
	val           interface{}
	output, error string
}

func runCoinTests(t *testing.T, id string, f func(val interface{}) ([]byte, error)) {
	if tests, exist := coinTests[id]; exist {
		for i, test := range tests {
			output, err := f(test.val)
			if err != nil && test.error == "" {
				t.Errorf("test %s-%d: unexpected error: %v\nvalue %#v\ntype %T",
					id, i, err, test.val, test.val)
				continue
			}
			if test.error != "" && fmt.Sprint(err) != test.error {
				t.Errorf("test %s-%d: error mismatch\ngot   %v\nwant  %v\nvalue %#v\ntype  %T",
					id, i, err, test.error, test.val, test.val)
				continue
			}
			b, err := hex.DecodeString(strings.Replace(test.output, " ", "", -1))
			if err != nil {
				panic(fmt.Sprintf("invalid hex string: %q", test.output))
			}
			if err == nil && !bytes.Equal(output, b) {
				t.Errorf("test %s-%d: output mismatch:\ngot   %X\nwant  %s\nvalue %#v\ntype  %T",
					id, i, output, test.output, test.val, test.val)
			}
		}
	}
}

func init() {
	keyJSON := `
		{
			"address":"54fb1c7d0f011dd63b08f85ed7b518ab82028100",
			"crypto":{
					"cipher":"aes-128-ctr",
					"ciphertext":"e77ec15da9bdec5488ce40b07a860fb5383dffce6950defeb80f6fcad4916b3a",
					"cipherparams":{
							"iv":"5df504a561d39675b0f9ebcbafe5098c"
					},
					"kdf":"scrypt",
					"kdfparams":{
							"dklen":32,
							"n":262144,
							"p":1,
							"r":8,
							"salt":"908cd3b189fc8ceba599382cf28c772b735fb598c7dbbc59ef0772d2b851f57f"
					},
					"mac":"9bb92ffd436f5248b73a641a26ae73c0a7d673bb700064f388b2be0f35fedabd"
			},
			"id":"2e15f180-b4f1-4d9c-b401-59eeeab36c87",
			"version":3
		}
	`
	sKey, err := KeyFromAccount([]byte(keyJSON), "1234")
	if err != nil {
		panic(err)
	}
	mockAccount, err := RecoveryKeyToAccount(sKey)
	if err != nil {
		panic(err)
	}
	am, err := makeAccountManager("/tmp/test_data/keystore/")
	if err != nil {
		panic(err)
	}
	walletdb := dbm.NewDB("0", "leveldb", "/tmp/walletdb/", 1)
	linkAccount := &LinkAccount{
		Logger:    log.Root(),
		account:   mockAccount,
		walletDB:  walletdb,
		Transfers: make([]*types.UTXOOutputDetail, 0),
		txKeys:    make(map[common.Hash]lktypes.Key, 0),
	}
	coinWallet = &Wallet{
		Logger:      log.Root(),
		currAccount: linkAccount,
		accManager:  am,
		walletDB:    walletdb,
		utxoGas:     new(big.Int).Mul(new(big.Int).SetInt64(50), new(big.Int).SetInt64(1e18)),
	}
	mockAccount.EthAddress = common.HexToAddress("0x54fb1c7d0f011dd63b08f85ed7b518ab82028100")
}

func makeAccountManager(keydir string) (*accounts.Manager, error) {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP

	if err := os.MkdirAll(keydir, 0700); err != nil {
		return nil, err
	}
	backends := []accounts.Backend{
		keystore.NewKeyStore(keydir, scryptN, scryptP),
	}
	return accounts.NewManager(backends...), nil
}

func TestDescUTXOPoolByAmount(t *testing.T) {
	utxoPool := make([]*UTXOItem, 0)
	for i := 0; i < 10; i++ {
		utxoPool = append(utxoPool, &UTXOItem{
			subaddr:  uint64(i),
			localIdx: uint64(i),
			height:   uint64(i),
			amount:   big.NewInt(rand.Int63n(1000)),
		})
	}
	fmt.Printf("origin utxoPool: %v\n", utxoPool)
	descUTXOPoolByAmount(utxoPool)
	fmt.Printf("sorted utxoPool: %v\n", utxoPool)
}

func TestKnuthDurstenfeldShuffle(t *testing.T) {
	utxoPool := make([]*UTXOItem, 0)
	for i := 0; i < 10; i++ {
		utxoPool = append(utxoPool, &UTXOItem{
			subaddr:  uint64(i),
			localIdx: uint64(i),
			height:   uint64(i),
			amount:   big.NewInt(rand.Int63n(1000)),
		})
	}
	fmt.Printf("origin utxoPool: %v\n", utxoPool)
	knuthDurstenfeldShuffle(utxoPool)
	fmt.Printf("sorted utxoPool: %v\n", utxoPool)
}

func TestSelectDFS(t *testing.T) {
	utxoPool := make([]*UTXOItem, 0)
	for i := 0; i < 10; i++ {
		utxoPool = append(utxoPool, &UTXOItem{
			subaddr:  uint64(i),
			localIdx: uint64(i),
			height:   uint64(i),
			amount:   big.NewInt(10),
		})
	}
	totalAmount := big.NewInt(0)
	for _, utxo := range utxoPool {
		totalAmount.Add(totalAmount, utxo.amount)
	}
	target := big.NewInt(0).Div(totalAmount, big.NewInt(2))
	selectDFS(utxoPool, target)
}

func TestSelectSRD(t *testing.T) {
	utxoPool := make([]*UTXOItem, 0)
	for i := 0; i < 10; i++ {
		utxoPool = append(utxoPool, &UTXOItem{
			subaddr:  uint64(i),
			localIdx: uint64(i),
			height:   uint64(i),
			amount:   big.NewInt(rand.Int63n(1000)),
		})
	}
	totalAmount := big.NewInt(0)
	for _, utxo := range utxoPool {
		totalAmount.Add(totalAmount, utxo.amount)
	}
	target := big.NewInt(0).Div(totalAmount, big.NewInt(2))
	selectSRD(utxoPool, target)
}

func TestCoinSelection(t *testing.T) {
	utxoPool := make([]*UTXOItem, 0)
	for i := 0; i < 10; i++ {
		utxoPool = append(utxoPool, &UTXOItem{
			subaddr:  uint64(i),
			localIdx: uint64(i),
			height:   uint64(i),
			amount:   big.NewInt(10),
		})
	}
	utxoPool = append(utxoPool, &UTXOItem{
		subaddr:  uint64(10),
		localIdx: uint64(10),
		height:   uint64(10),
		amount:   big.NewInt(200),
	})
	totalAmount := big.NewInt(0)
	for _, utxo := range utxoPool {
		totalAmount.Add(totalAmount, utxo.amount)
	}
	target := big.NewInt(0).Div(totalAmount, big.NewInt(2))
	target = big.NewInt(100)
	selectedUtxos, selectedAmount, err := coinSelection(utxoPool, target)
	if err != nil {
		panic(err)
	}
	fmt.Printf("selectedUtxos: %v selectedAmount: %d\n", selectedUtxos, selectedAmount)
}

func TestPayDests(t *testing.T) {
	utxoPool := make([]*UTXOItem, 0)
	for i := 0; i < 10; i++ {
		utxoPool = append(utxoPool, &UTXOItem{
			subaddr:  uint64(i),
			localIdx: uint64(i),
			height:   uint64(i),
			amount:   big.NewInt(rand.Int63n(1000)),
		})
	}
	totalAmount := big.NewInt(0)
	for _, utxo := range utxoPool {
		totalAmount.Add(totalAmount, utxo.amount)
	}
	target := big.NewInt(0).Div(totalAmount, big.NewInt(2))
	selectedUtxos, _, err := coinSelection(utxoPool, target)
	if err != nil {
		panic(err)
	}
	dests := make([]types.DestEntry, 0)
	dests = append(dests, &types.AccountDestEntry{
		To:     common.Address{1},
		Amount: big.NewInt(rand.Int63n(target.Int64())),
	})
	dests = append(dests, &types.UTXODestEntry{
		Amount: big.NewInt(0).Sub(target, dests[0].GetAmount()),
		Addr:   lkctypes.AccountAddress{},
	})
	dests, err = payDests(selectedUtxos, dests)
	if err != nil {
		panic(err)
	}
	for _, dest := range dests {
		fmt.Printf("type: %s amount: %d\n", dest.Type(), dest.GetAmount())
	}
}

func TestDirectSelection(t *testing.T) {
	utxoPool := make([]*UTXOItem, 0)
	totalAmount := big.NewInt(0)
	for i := 0; i < 10; i++ {
		amount := big.NewInt(rand.Int63n(1000))
		utxoPool = append(utxoPool, &UTXOItem{
			subaddr:  uint64(i),
			localIdx: uint64(i),
			height:   uint64(i),
			amount:   big.NewInt(0).Mul(amount, big.NewInt(1e18)),
		})
		totalAmount.Add(totalAmount, amount)
	}
	target := big.NewInt(0).Div(totalAmount, big.NewInt(2))
	dests := make([]types.DestEntry, 0)
	accAmount := big.NewInt(rand.Int63n(target.Int64()))
	dests = append(dests, &types.AccountDestEntry{
		To:     common.Address{1},
		Amount: big.NewInt(0).Mul(accAmount, big.NewInt(1e18)),
	})
	dests = append(dests, &types.UTXODestEntry{
		Amount: big.NewInt(0).Mul(big.NewInt(0).Sub(target, accAmount), big.NewInt(1e18)),
		Addr:   lkctypes.AccountAddress{},
	})
	inputSum := big.NewInt(0)
	outputSum := big.NewInt(0)
	for _, utxo := range utxoPool {
		inputSum.Add(inputSum, utxo.amount)
	}
	for _, dest := range dests {
		outputSum.Add(outputSum, dest.GetAmount())
	}
	fmt.Printf("input amount: %s output amount: %s\n", inputSum.String(), outputSum.String())
	packets, err := coinWallet.directSelection(utxoPool, dests, 0, common.EmptyAddress)
	if err != nil {
		panic(err)
	}
	for _, packet := range packets {
		fmt.Printf("inputs: %v\n", packet.Inputs)
		for _, dest := range packet.Outputs {
			fmt.Printf("type: %s amount: %d\n", dest.Type(), dest.GetAmount())
		}
	}
}

func TestMergeDests(t *testing.T) {
	dests := make([]types.DestEntry, 0)
	for i := 0; i < 20; i++ {
		j := byte(i / 5)
		dests = append(dests, &types.UTXODestEntry{
			Amount: big.NewInt(rand.Int63n(1000)),
			Addr: lkctypes.AccountAddress{
				SpendPublicKey: lkctypes.PublicKey{j},
			},
		})
	}
	for i := 20; i < 32; i++ {
		j := byte(i)
		dests = append(dests, &types.UTXODestEntry{
			Amount: big.NewInt(rand.Int63n(1000)),
			Addr: lkctypes.AccountAddress{
				SpendPublicKey: lkctypes.PublicKey{j},
			},
		})
	}
	dests = append(dests, &types.AccountDestEntry{
		To:     common.Address{1},
		Amount: big.NewInt(rand.Int63n(1000)),
	})
	dests, err := mergeDests(dests)
	if err != nil {
		panic(err)
	}
	for _, dest := range dests {
		if utxodest, ok := dest.(*types.UTXODestEntry); ok {
			fmt.Printf("merged type: %s amount: %d spendKey: %x\n", dest.Type(), dest.GetAmount(), utxodest.Addr.SpendPublicKey[:])
		} else {
			fmt.Printf("merged type: %s amount: %d\n", dest.Type(), dest.GetAmount())
		}
	}
}
