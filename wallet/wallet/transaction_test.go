package wallet

import (
	"bytes"
	"encoding/hex"
	"time"

	//"encoding/json"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/ringct"
	lktypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/xcrypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/wallet/config"
	"github.com/lianxiangcloud/linkchain/wallet/daemon"
)

var (
	mockWallet *Wallet
	transTests = make(map[string][]TransTest)
)

type TransTest struct {
	val           interface{}
	output, error string
}

func runTransTests(t *testing.T, id string, f func(val interface{}) ([]byte, error)) {
	if tests, exist := transTests[id]; exist {
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
	mockWallet = &Wallet{
		Logger:      log.Root(),
		currAccount: linkAccount,
		accManager:  am,
		walletDB:    walletdb,
		utxoGas:     new(big.Int).Mul(new(big.Int).SetUint64(defaultUTXOGas), new(big.Int).SetInt64(1e11)),
	}
	mockAccount.EthAddress = common.HexToAddress("0x54fb1c7d0f011dd63b08f85ed7b518ab82028100")
	cfg := config.DefaultDaemonConfig()
	daemon.InitDaemonClient(cfg)
	//0x54fb1c7d0f011dd63b08f85ed7b518ab82028100 -> B8VqGZiPtfJHuXfxBvqrRf43Gk4dP82i1egq6RgfARKVFTeL2eMtt2Yfc71PqwYH2nVe99sqN1WJY1S7cMy1CYVuCN9qJJr 1e21
	rawTx := "f904caf856cf4852c16c3232f84d8089367b2d3f4823940000a0f3cc70f5ad1c19b76e91e4d78e6f56feb865c776b07c6f81f14189eb27d6db0ea0fabfbd41bce85c2328d010d480417fd9c6af54d1fb976af9f883ecd2290836a2f84c5842699137a517f843a0ccf9faa6bed95bb04bd65cb09cc3f6379fc20f55d6ae21588b459e08f98e4c9880a096dba525c886f200200a6b643806fd218e34ef7104e49fb8eb48d032c4782d13940000000000000000000000000000000000000000a0cf3cd909f93394248d958c8cad410202792ff1ee035bd87c01bb1d56ac128be4e1a07a3cdc06a9c53dcc422ffd96f61c5c80b5b6bec583bbb5af1bb6719aa843ba56884563918244f4000080f84582e3e6a02fc5c2c5062633e8586c9ff2ea4448f461676cc8e20aad64148ce08730621a1ca07c01beeb4375c932f31def7083166cf564e5ae5798f473b75b3b81b5f9488a16f90378f8b080c0f865f863a0af002b6141dde607e26160f6098295f6469ab6e8b4600f3add8a591eec4e0902a07ff9701977502ccd45d15a093d0e749b846e0d3dcf48e8facc90851bf41d9a00a00000000000000000000000000000000000000000000000000000000000000000f844f842a00000000000000000000000000000000000000000000000000000000000000000a0652e3723ddbc1c590d1ba7cdefc42007fdfe49ab5d7aa822e766aa24b2e8137580f902c3c0f902bcf902b9a0ccdd50a8738940279c4a9f84537c02fdad3a41881fc7a0c13d2c932f89dc97d7a09975115d03a01a2789379a446b37349da1a3a93853ce4b806dc036889a30a0d1a0920a6f6b603370bfcb8826fe6cc3a7d2cf50fd021d28f72282ffa5b81991adc8a0ed2ed9c6ae32085096f131290b3d38811656461005a9a09ca66cd6d2d025f3f0a0180bf0ac2bc037225f81a3d3fff9847aa05b75d6d54af47a6bb120d6ddd34202a0dc898ada541d915e8b246b3cfc23ad2c80ac17c86e8c593d15ca265266b6d300f8c6a037881d637570ea6b5b8f2690e35bca69be15e3a027d5454145233c6fc78d3cfca0335410631300946a18fd68e17c09639b32b23a1965df505adf2d907ca145a350a00281db08c4e5e404c439ed8d14a850152a4011142063263e8c8cbf7271567581a0a34c2b3bc70dcbf2c9ed7376de1c6a694d66d0f4a711bc71f31e74f77fe82eaea075e792b54ff4744d646c5e68391604cf470016f6747fb624738bd5c0ee509bdea0c48e20452b9f04b8c814f237b12d0aa4e350116fe9579f46eb9196fd50b90bf6f8c6a097d802447a4f8ef413ec4fce3a62afae7ab01ee932de751c2366a3114b395399a0802d1b642db0c480cae95a91b4ca5444f36231fff9ffbc110c9b7242fa5ec91ea0fb99e8724dc2da76d078eaf5daf290b63edb21d7bf06ab8569bedbb9ca3a4ce1a06f6346f0afbc928a91977da7170dca8841e205c392c5c497411ea3714296ee58a00a9a21460bc8523b672190f5fea38fe461af19397d42adfa738a71310f94ae6ba04e0f095abcf570b28853f1e5b630f882ba4c68c2ac117f61be5f132eeb9bce10a05eb932241a406a688dfec3a810e441efb23eeb46f2ac2ddf1202515a76fe950ca002bf8fe113016c5c20144ad0f84642498b7ef61c8e0805119fd0f77099413b02a0652a5d380582b49fe4f0ac43f1738bba2296b917f807293a7c4e07fd50d06d0bc0c0c0"
	bz, err := hex.DecodeString(rawTx)
	if err != nil {
		panic(err)
	}
	types.RegisterUTXOTxData()
	var utxoTx types.UTXOTransaction
	err = ser.DecodeBytes(bz, &utxoTx)
	if err != nil {
		panic(err)
	}
	mockWallet.SubmitUTXOTransaction(&utxoTx)
	ecdh := &lktypes.EcdhTuple{
		Mask:   utxoTx.RCTSig.RctSigBase.EcdhInfo[0].Mask,
		Amount: utxoTx.RCTSig.RctSigBase.EcdhInfo[0].Amount,
	}
	//wait block commit
	time.Sleep(3 * time.Second)
	derivationKey, err := xcrypto.GenerateKeyDerivation(utxoTx.RKey, mockWallet.currAccount.account.GetKeys().ViewSKey)
	if err != nil {
		panic(err)
	}
	scalar, err := xcrypto.DerivationToScalar(derivationKey, 0)
	if err != nil {
		panic(err)
	}
	ok := xcrypto.EcdhDecode(ecdh, lktypes.Key(scalar), false)
	if !ok {
		panic("xcrypto.EcdhDecode fail")
	}
	mockWallet.currAccount.Transfers = append(mockWallet.currAccount.Transfers, &types.UTXOOutputDetail{
		OutIndex:     0,
		GlobalIndex:  0,
		Spent:        false,
		Frozen:       false,
		RKey:         utxoTx.RKey,
		Mask:         ecdh.Mask,
		Amount:       big.NewInt(0).Mul(types.Hash2BigInt(ecdh.Amount), big.NewInt(1e10)),
		SubAddrIndex: 0,
		TokenID:      common.EmptyAddress,
	})
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

func TestCheckDest(t *testing.T) {
	type stest struct {
		dests   []types.DestEntry
		mode    InputMode
		tokenID common.Address
	}
	transTests["TestCheckDest"] = []TransTest{
		{
			val: stest{
				dests:   []types.DestEntry{},
				mode:    UTXOInputMode,
				tokenID: common.EmptyAddress,
			},
			output: "",
			error:  "output empty",
		},
		{
			val: stest{
				dests: []types.DestEntry{
					&types.UTXODestEntry{
						Addr:   lktypes.AccountAddress{},
						Amount: big.NewInt(1e11),
					},
					&types.UTXODestEntry{
						Addr:   lktypes.AccountAddress{},
						Amount: big.NewInt(1e11),
					},
				},
				mode:    UTXOInputMode,
				tokenID: common.EmptyAddress,
			},
			output: fmt.Sprintf("%x", big.NewInt(0).Add(big.NewInt(2*1e11), mockWallet.estimateUtxoTxFee()).Bytes()),
			error:  "",
		},
		{
			val: stest{
				dests: []types.DestEntry{
					&types.UTXODestEntry{
						Addr:   lktypes.AccountAddress{},
						Amount: big.NewInt(1e11),
					},
					&types.UTXODestEntry{
						Addr:   lktypes.AccountAddress{},
						Amount: big.NewInt(1e11),
					},
				},
				mode:    AccountInputMode,
				tokenID: common.EmptyAddress,
			},
			output: fmt.Sprintf("%x", big.NewInt(0).Add(big.NewInt(2*1e11), mockWallet.estimateTxFee(big.NewInt(2*1e11))).Bytes()),
			error:  "",
		},
	}
	runTransTests(t, "TestCheckDest", func(val interface{}) ([]byte, error) {
		st := val.(stest)
		amount, _, err := mockWallet.checkDest(st.dests, st.tokenID, st.mode)
		return amount.Bytes(), err
	})
}

func TestSortableSubaddr(t *testing.T) {
	type sstest struct {
		subaddr []uint64
		balance map[uint64]*big.Int
	}
	transTests["TestSortableSubaddr"] = []TransTest{
		{
			val: sstest{
				subaddr: []uint64{0, 1},
				balance: map[uint64]*big.Int{
					0: big.NewInt(100),
					1: big.NewInt(200),
				},
			},
			output: "0100",
			error:  "",
		},
		{
			val: sstest{
				subaddr: []uint64{0, 1},
				balance: map[uint64]*big.Int{
					0: big.NewInt(200),
					1: big.NewInt(100),
				},
			},
			output: "0001",
			error:  "",
		},
	}
	runTransTests(t, "TestSortableSubaddr", func(val interface{}) ([]byte, error) {
		sstest := val.(sstest)
		ss := initSortableSubaddr(sstest.subaddr, sstest.balance)
		sort.Sort(ss)
		subaddr := make([]byte, 0)
		for i := 0; i < len(ss); i++ {
			subaddr = append(subaddr, byte(ss[i].Subaddr))
		}
		return subaddr, nil
	})
}

func TestDecodeAmount(t *testing.T) {
	type stest struct {
		amount string
		rkey   string
		index  int
	}
	transTests["TestDecodeAmount"] = []TransTest{
		{
			val: stest{
				amount: "70c5be1d33f10bb316f98b2e959c4bf66b27df9e2f57528fbf0cd057416a500d",
				rkey:   "ef6ca937902bf64293bab6fd01f7094c359f6d482f3d1f9db6565e3ca9e109e4",
				index:  0,
			},
			output: "0000a0dec5adc935360000000000000000000000000000000000000000000000", //1000000000000000000000
			error:  "",
		},
	}
	runTransTests(t, "TestDecodeAmount", func(val interface{}) ([]byte, error) {
		st := val.(stest)
		amount, err := hex.DecodeString(st.amount)
		if err != nil {
			return nil, err
		}
		var encAmount lktypes.Key
		copy(encAmount[:], amount)
		ecdh := &lktypes.EcdhTuple{
			Amount: encAmount,
		}
		rKey, err := hex.DecodeString(st.rkey)
		if err != nil {
			return nil, err
		}
		var rPubKey lktypes.PublicKey
		copy(rPubKey[:], rKey)
		derivationKey, err := xcrypto.GenerateKeyDerivation(rPubKey, mockWallet.currAccount.account.GetKeys().ViewSKey)
		if err != nil {
			return nil, err
		}
		scalar, err := xcrypto.DerivationToScalar(derivationKey, st.index)
		if err != nil {
			return nil, err
		}
		ok := xcrypto.EcdhDecode(ecdh, lktypes.Key(scalar), false)
		if !ok {
			return nil, err
		}
		return []byte(ecdh.Amount[:]), nil
	})
}

func TestOneTimeAddress(t *testing.T) {
	transTests["TestOneTimeAddress"] = []TransTest{
		{
			val:    "B8VqGZiPtfJHuXfxBvqrRf43Gk4dP82i1egq6RgfARKVFTeL2eMtt2Yfc71PqwYH2nVe99sqN1WJY1S7cMy1CYVuCN9qJJr",
			output: "8f138423c4219965128858a11aca4a122b82d47c15bca6e1506a22a1c14b3856",
			error:  "",
		},
	}
	runTransTests(t, "TestOneTimeAddress", func(val interface{}) ([]byte, error) {
		str := val.(string)
		toAddr, err := StrToAddress(str)
		if err != nil {
			return nil, err
		}
		rSecKey, rPubKey := xcrypto.SkpkGen()
		derivationKey, err := xcrypto.GenerateKeyDerivation(toAddr.ViewPublicKey, lktypes.SecretKey(rSecKey))
		if err != nil {
			return nil, err
		}
		index := 0
		otAddr, err := xcrypto.DerivePublicKey(derivationKey, index, toAddr.SpendPublicKey)
		if err != nil {
			return nil, err
		}
		derivationKey2, err := xcrypto.GenerateKeyDerivation(lktypes.PublicKey(rPubKey), mockWallet.currAccount.account.GetKeys().ViewSKey)
		if err != nil {
			return nil, err
		}
		spendPubKey, err := xcrypto.DeriveSubaddressPublicKey(otAddr, derivationKey2, index)
		if err != nil {
			return nil, err
		}
		return []byte(spendPubKey[:]), nil
	})
}

func TestGenerateKeyImage(t *testing.T) {
	type stest struct {
		rkey   string
		otaddr string
		index  int
	}
	transTests["TestGenerateKeyImage"] = []TransTest{
		{
			val: stest{
				rkey:   "ef6ca937902bf64293bab6fd01f7094c359f6d482f3d1f9db6565e3ca9e109e4",
				otaddr: "79af500fb22e6af1120964737cf2661a1088e21868d5d594fbc72a4cce8a5a35",
				index:  0,
			},
			output: "f25180b6bafb4b0d0bb0225f5dc8d51cf8bcde90a3f716d701d7c14b14dcbb65",
			error:  "",
		},
	}
	runTransTests(t, "TestGenerateKeyImage", func(val interface{}) ([]byte, error) {
		st := val.(stest)
		rkey, _ := hex.DecodeString(st.rkey)
		var rPubKey lktypes.PublicKey
		copy(rPubKey[:], rkey)
		otaddr, _ := hex.DecodeString(st.otaddr)
		var otAddr lktypes.PublicKey
		copy(otAddr[:], otaddr)
		derivationKey, err := xcrypto.GenerateKeyDerivation(rPubKey, mockWallet.currAccount.account.GetKeys().ViewSKey)
		if err != nil {
			return nil, err
		}
		secretKey, err := xcrypto.DeriveSecretKey(derivationKey, st.index, mockWallet.currAccount.account.GetKeys().SpendSKey)
		if err != nil {
			return nil, err
		}
		keyImage, err := xcrypto.GenerateKeyImage(otAddr, secretKey)
		return []byte(keyImage[:]), err
	})
}

func TestOutputs(t *testing.T) {
	type stest struct {
		words  string
		rkey   string
		otaddr string
		amount string
		index  int
	}
	transTests["TestOutputs"] = []TransTest{
		{
			val: stest{
				words:  "pruned leopard wrong isolated theatrics gotten zoom gambit each ponies hotel ditch ocean somewhere betting irony iris purged lettuce ability peaches issued cavernous cupcake somewhere",
				rkey:   "fe3805342b2e88261ac9d6ffef176810084b60d8fc7c33fc786616b55366fc65",
				otaddr: "537a30033d10f52345da5bd390abebb024a849e044b822d2596bb307538380cf",
				amount: "c20d9e6aba3c3632",
				index:  0,
			},
			output: "dcf5dd4bce3f0000000000000000000000000000000000000000000000000000", //70155268650460
			error:  "",
		},
		{
			val: stest{
				words:  "hijack puffin unhappy until acoustic byline vivid deftly cunning giddy enigma haunted beware oncoming eleven hairy butter syllabus were bomb ferry meant insult joining acoustic",
				rkey:   "fe3805342b2e88261ac9d6ffef176810084b60d8fc7c33fc786616b55366fc65",
				otaddr: "219cab7b5104408068fc9b63683670de739e8dcd5e8c0c58ed8bcfb53e2c99a5",
				amount: "67e87c615ec55732",
				index:  1,
			},
			output: "8096980000000000000000000000000000000000000000000000000000000000", //10000000
			error:  "",
		},
	}
	runTransTests(t, "TestOutputs", func(val interface{}) ([]byte, error) {
		st := val.(stest)
		acc, err := WordsToAccount(st.words)
		if err != nil {
			return nil, err
		}
		rkey, _ := hex.DecodeString(st.rkey)
		var rPubKey lktypes.PublicKey
		copy(rPubKey[:], rkey)
		otaddr, _ := hex.DecodeString(st.otaddr)
		var otAddr lktypes.PublicKey
		copy(otAddr[:], otaddr)
		derivationKey, err := xcrypto.GenerateKeyDerivation(rPubKey, acc.GetKeys().ViewSKey)
		if err != nil {
			return nil, err
		}
		spendPubKey, err := xcrypto.DeriveSubaddressPublicKey(otAddr, derivationKey, st.index)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(spendPubKey[:], acc.GetKeys().Addr.SpendPublicKey[:]) {
			return nil, fmt.Errorf("spend public key not equal")
		}
		encodeAmount, err := hex.DecodeString(st.amount)
		if err != nil {
			return nil, err
		}
		var amount lktypes.Key
		copy(amount[0:], encodeAmount)
		ecdh := &lktypes.EcdhTuple{
			Mask:   ringct.Z,
			Amount: amount,
		}
		scalar, err := xcrypto.DerivationToScalar(derivationKey, st.index)
		if err != nil {
			return nil, err
		}
		ok := xcrypto.EcdhDecode(ecdh, lktypes.Key(scalar), true)
		if !ok {
			return nil, fmt.Errorf("EcdhDecode fail")
		}
		return []byte(ecdh.Amount[:]), nil
	})
}

func TestSubmitUTXOTransactions(t *testing.T) {
	transTests["TestSubmitUTXOTransactions"] = []TransTest{
		{
			val:    "f9063deb10c698b1d37308e3c180a0fa2042f28e79a2f279c0ecf68567be051dd3d329bb1987747b3f088b0d868740f8985842699137a517f843a08435fd1d0a9f209e5b7e1d322b7687bc2c8939077b6cc10cc1c7957bc1dd11d580a05384cb7608ecef955c680a4975701b907db0c37c401cc5eb86e6b0480cb24f415842699137a517f843a0445b80783b3d3bf0602782a67f972ef3d62adb851c031a518628d32c4a0fda3f80a094d516d752efbbc103a41398258638ca8f1f499318910d57fbe1bd7feb0da3fb940000000000000000000000000000000000000000a04f1ea6d09b8743cfbc9b512035ac3041874b06966e119753d32faedc2bb67443f842a044f5a10838dc58d270ccb02c95b0779faac1071ce59d37339726ed6e4b3da4f2a026a532290639fd11c255786ee94a429597088df9b5879aa4792edfa29d1e0a7d87b1a2bc2ec5000080c3808080f904edf9015903c0f8caf863a019defdf2f5aa49d94058838ee16e5e5b78edf1544f75237a59fd25089360eb08a0cf3aab8ac8fc70f4d6354fa5a084c3e56da5449662af3e0a0558b50989be0203a00000000000000000000000000000000000000000000000000000000000000000f863a0ee0963a616f3d0883b3ae90796c154420d5330fa40a94090e54b7e49726e6408a0f664057e253da1bd300d7e587201e3dd313ec3d9b1d87b16b17fdb5809e94406a00000000000000000000000000000000000000000000000000000000000000000f888f842a00000000000000000000000000000000000000000000000000000000000000000a0b0ac54da56b8d0bc4b12bbc9b0c4b2aec11658872ff16fb9f30698571dd53bf6f842a00000000000000000000000000000000000000000000000000000000000000000a0d4082278747e739eb58f3b4abd9625b9caada94fa7ed8ae2cd664fc42a0d5d1c80f9038ec0f902fef902fba087509cf003efe1df888e37358fa8843d861372c12c72e48928a5822eb35f9a10a082eff229d74b0730ba4a78ed1a77b757aa7b5fb9ea576515ca94673edfe14b37a00e98eccc0a952749c324ebcc2f6ba5b3896ad222257300536038ee029f764730a0afd56c91cac5963bdb5eae06e0260ca5e8497cba84536685f099fc83469ef857a0ad2b1ff7f8f20ba8aafc4fda690443db3e414c7c2389372d8b9e0ebb82bf9802a08fe7409113f91f16c85fe681116b2664890c6a67475841f2cd1f09bf5523bf0ff8e7a0547eb9da2f599b1dfb81571b3b123e7e6ebaf64ac89002235666de9eb55b56b9a0a0298308cb679ecbb1faec4c41d4b604a63721dfac8b50d2597fea3e579aa3ada0e75df745130dfc7ba7ad31e0207ef5fc121268a8b54075572aeb0e5b7bc524f5a0eabaefe5a86cf37283119eaae223944c458a0b90f87c1c7db45f700e4db723a9a054c5d49d76d80a7383a313ca83cffed0cb951eb6da9e06a574628e1eebc2d6aea0a5f947f10fafbb022d1a331ce80cde4549c1b34f6503c924f6e2dbd1dcdece7ca0f7ccd35758758cf771a6b552d1ffb499c592d08c4f3fce3dd39585be6589d009f8e7a0c5d149d18f68c680b0b3856912ecccb360a505efdae547de4414910b10a518c5a00d9276c93fb5c74433a5d70b7aba097a51dd7cd77cc1654655ad94e8df434a8fa0369f342e402b47bc4689f72ca8a184b219dbea7bc6972850abd6c53452bda934a06badd7c7162ac9af5fa9fe46095570659ffe7beb56e5ddc8b21e8b05f6dd1b72a0967a0c41db92f830a0b854cb30de35053d06a8933d46a2591521c7df612c510ea04709de973e162f3506682fb2508001d047c8dac4702a4ab625c3e0d5b4eca49ea0b8b9672660072ce44a0483525eb10c3bd74b8b80b1a0af68ffee609357fe65ada054419922d73c269466ec910ce89958caf85aab6fa120f554fd6f3d4c7e0e8c09a07e4dda499d53a702e2c4f54a1d412bb9eccefcb1fb7475c1dc9c29085a82de09a0ca656921d627e647215c176cea4a647a79dfbb2b2f801b6a9a89b2b489b19003e3e2c0a00000000000000000000000000000000000000000000000000000000000000000e1a0aaee6a4ffca6a09ab9a8c2179e4ddd1857178f86b967050ead8612b7b3c74c34f844f842a05bf6694964ba9ac0f99900f96336d7ee541580c51a6715501882766c5257c908a035c20e9e16e130a08fb04e218bd8da9dd14519ec8f14f82839c3f4eb3ba4d30b",
			output: "791ef44b47090bfa1f6c81a3629d5a6dd92b3a9a57937520dc5d63154ed5a85b",
			error:  "",
		},
	}
	runTransTests(t, "TestSubmitUTXOTransactions_NOTEXEC", func(val interface{}) ([]byte, error) {
		strTx := val.(string)
		rawTx, err := hex.DecodeString(strTx)
		if err != nil {
			return nil, err
		}
		var utxoTx types.UTXOTransaction
		err = ser.DecodeBytes(rawTx, &utxoTx)
		if err != nil {
			return nil, err
		}
		utxoTxes := []*types.UTXOTransaction{&utxoTx}
		hashes, err := mockWallet.SubmitUTXOTransactions(utxoTxes)
		if err != nil {
			return nil, err
		}
		return []byte(hashes[0][:]), nil
	})
}

func TestCreateAinTransaction(t *testing.T) {
	type stest struct {
		from   string
		passwd string
		nonce  uint64
		amount []int64
		to     []string
	}
	transTests["TestCreateAinTransaction"] = []TransTest{
		{
			val: stest{
				from:   "0x54fb1c7d0f011dd63b08f85ed7b518ab82028100",
				passwd: "1234",
				nonce:  0,
				amount: []int64{
					1e18,
				},
				to: []string{
					"0x7b6837189a3464d3c696069b2b42a9ae8e17dda1",
				},
			},
			output: "00",
			error:  "",
		},
		{
			val: stest{
				from:   "0x54fb1c7d0f011dd63b08f85ed7b518ab82028100",
				passwd: "1234",
				nonce:  0,
				amount: []int64{
					1e18,
					1e18,
				},
				to: []string{
					"0x7b6837189a3464d3c696069b2b42a9ae8e17dda1",
					"0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5",
				},
			},
			output: "00",
			error:  "",
		},
		{
			val: stest{
				from:   "0x54fb1c7d0f011dd63b08f85ed7b518ab82028100",
				passwd: "1234",
				nonce:  0,
				amount: []int64{
					1e18,
				},
				to: []string{
					"EUApefPm27VPhJKFW8v6KZG75CgsLcSfH53AYN6faQqjMvZPCMhVHDcKW3h2fMjwHNMNqcLnE8NJeRVm9fSUXeDxAYiCGK8",
				},
			},
			output: "00",
			error:  "",
		},
		{
			val: stest{
				from:   "0x54fb1c7d0f011dd63b08f85ed7b518ab82028100",
				passwd: "1234",
				nonce:  0,
				amount: []int64{
					1e18,
					1e18,
				},
				to: []string{
					"EUApefPm27VPhJKFW8v6KZG75CgsLcSfH53AYN6faQqjMvZPCMhVHDcKW3h2fMjwHNMNqcLnE8NJeRVm9fSUXeDxAYiCGK8",
					"ERXDVwRpPAZSWzNTYPVYUrW1CfCbrndK6LPUapBvKUD9GFSDT2uamAYGZ7j7ZUduJtWYPMvboXcMrS8MZYFg9HFT6pXVeUf",
				},
			},
			output: "00",
			error:  "",
		},
		{
			val: stest{
				from:   "0x54fb1c7d0f011dd63b08f85ed7b518ab82028100",
				passwd: "1234",
				nonce:  0,
				amount: []int64{
					1e18,
					1e18,
					1e18,
					1e18,
				},
				to: []string{
					"0x7b6837189a3464d3c696069b2b42a9ae8e17dda1",
					"0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5",
					"EUApefPm27VPhJKFW8v6KZG75CgsLcSfH53AYN6faQqjMvZPCMhVHDcKW3h2fMjwHNMNqcLnE8NJeRVm9fSUXeDxAYiCGK8",
					"ERXDVwRpPAZSWzNTYPVYUrW1CfCbrndK6LPUapBvKUD9GFSDT2uamAYGZ7j7ZUduJtWYPMvboXcMrS8MZYFg9HFT6pXVeUf",
				},
			},
			output: "00",
			error:  "",
		},
	}
	runTransTests(t, "TestCreateAinTransaction_NOTEXEC", func(val interface{}) ([]byte, error) {
		st := val.(stest)
		from := common.HexToAddress(st.from)
		var dests []types.DestEntry
		for i, addr := range st.to {
			if len(addr) == 42 || len(addr) == 40 {
				ethAddr := common.HexToAddress(addr)
				dest := &types.AccountDestEntry{
					To:     ethAddr,
					Amount: big.NewInt(st.amount[i]),
				}
				dests = append(dests, dest)
			} else {
				moneroAddr, err := StrToAddress(addr)
				if err != nil {
					return nil, err
				}
				dest := &types.UTXODestEntry{
					Addr:   *moneroAddr,
					Amount: big.NewInt(st.amount[i]),
				}
				dests = append(dests, dest)
			}
		}
		signedTx, err := mockWallet.CreateAinTransaction(from, st.passwd, st.nonce, dests, common.EmptyAddress, nil)
		if err != nil {
			return nil, err
		}
		bz, err := ser.EncodeToBytes(signedTx)
		if err != nil {
			return nil, err
		}
		fmt.Printf("tx_hex: %x\n", bz)
		return []byte{0}, nil
	})
}

func TestCreateUinTransaction(t *testing.T) {
	type stest struct {
		from   string
		passwd string
		amount []int64
		to     []string
	}
	transTests["TestCreateUinTransaction"] = []TransTest{
		{
			val: stest{
				from:   "0x54fb1c7d0f011dd63b08f85ed7b518ab82028100",
				passwd: "1234",
				amount: []int64{
					1e18,
				},
				to: []string{
					"0x7b6837189a3464d3c696069b2b42a9ae8e17dda1",
				},
			},
			output: "00",
			error:  "",
		},
		{
			val: stest{
				from:   "0x54fb1c7d0f011dd63b08f85ed7b518ab82028100",
				passwd: "1234",
				amount: []int64{
					1e18,
					1e18,
				},
				to: []string{
					"0x7b6837189a3464d3c696069b2b42a9ae8e17dda1",
					"0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5",
				},
			},
			output: "00",
			error:  "",
		},
		{
			val: stest{
				from:   "0x54fb1c7d0f011dd63b08f85ed7b518ab82028100",
				passwd: "1234",
				amount: []int64{
					1e18,
				},
				to: []string{
					"EUApefPm27VPhJKFW8v6KZG75CgsLcSfH53AYN6faQqjMvZPCMhVHDcKW3h2fMjwHNMNqcLnE8NJeRVm9fSUXeDxAYiCGK8",
				},
			},
			output: "00",
			error:  "",
		},
		{
			val: stest{
				from:   "0x54fb1c7d0f011dd63b08f85ed7b518ab82028100",
				passwd: "1234",
				amount: []int64{
					1e18,
					1e18,
				},
				to: []string{
					"EUApefPm27VPhJKFW8v6KZG75CgsLcSfH53AYN6faQqjMvZPCMhVHDcKW3h2fMjwHNMNqcLnE8NJeRVm9fSUXeDxAYiCGK8",
					"ERXDVwRpPAZSWzNTYPVYUrW1CfCbrndK6LPUapBvKUD9GFSDT2uamAYGZ7j7ZUduJtWYPMvboXcMrS8MZYFg9HFT6pXVeUf",
				},
			},
			output: "00",
			error:  "",
		},
		{
			val: stest{
				from:   "0x54fb1c7d0f011dd63b08f85ed7b518ab82028100",
				passwd: "1234",
				amount: []int64{
					1e18,
					1e18,
					1e18,
					1e18,
				},
				to: []string{
					"0x7b6837189a3464d3c696069b2b42a9ae8e17dda1",
					"0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5",
					"EUApefPm27VPhJKFW8v6KZG75CgsLcSfH53AYN6faQqjMvZPCMhVHDcKW3h2fMjwHNMNqcLnE8NJeRVm9fSUXeDxAYiCGK8",
					"ERXDVwRpPAZSWzNTYPVYUrW1CfCbrndK6LPUapBvKUD9GFSDT2uamAYGZ7j7ZUduJtWYPMvboXcMrS8MZYFg9HFT6pXVeUf",
				},
			},
			output: "00",
			error:  "",
		},
	}
	runTransTests(t, "TestCreateUinTransaction_NOTEXEC", func(val interface{}) ([]byte, error) {
		st := val.(stest)
		from := common.HexToAddress(st.from)
		var dests []types.DestEntry
		for i, addr := range st.to {
			if len(addr) == 42 || len(addr) == 40 {
				ethAddr := common.HexToAddress(addr)
				dest := &types.AccountDestEntry{
					To:     ethAddr,
					Amount: big.NewInt(st.amount[i]),
				}
				dests = append(dests, dest)
			} else {
				moneroAddr, err := StrToAddress(addr)
				if err != nil {
					return nil, err
				}
				dest := &types.UTXODestEntry{
					Addr:   *moneroAddr,
					Amount: big.NewInt(st.amount[i]),
				}
				dests = append(dests, dest)
			}
		}
		subaddrs := []uint64{0}
		txes, err := mockWallet.CreateUinTransaction(from, st.passwd, subaddrs, dests, common.EmptyAddress, from, nil)
		if err != nil {
			return nil, err
		}
		bz, err := ser.EncodeToBytes(txes[0])
		if err != nil {
			return nil, err
		}
		fmt.Printf("tx_hex: %x\n", bz)
		return []byte{0}, nil
	})
}

func TestSubaddrReceive(t *testing.T) {
	type stest struct {
		addr   string
		subIdx int
	}
	transTests["TestSubaddrReceive"] = []TransTest{
		{
			val: stest{
				addr:   "EUApefPm27VPhJKFW8v6KZG75CgsLcSfH53AYN6faQqjMvZPCMhVHDcKW3h2fMjwHNMNqcLnE8NJeRVm9fSUXeDxAYiCGK8",
				subIdx: 1,
			},
			output: "85c40c61b6925c87aef07f0e7dcc405a4e1a03782df958182336a5055a2aae7d",
			error:  "",
		},
	}
	runTransTests(t, "TestSubaddrReceive", func(val interface{}) ([]byte, error) {
		st := val.(stest)
		addr, err := StrToAddress(st.addr)
		if err != nil {
			return nil, err
		}
		rSecKey, _ := xcrypto.SkpkGen()
		additionalKey, _ := xcrypto.ScalarmultKey(lktypes.Key(addr.SpendPublicKey), rSecKey)
		derivationKey, err := xcrypto.GenerateKeyDerivation(addr.ViewPublicKey, lktypes.SecretKey(rSecKey))
		if err != nil {
			return nil, err
		}
		derivationKey1, err := xcrypto.GenerateKeyDerivation(lktypes.PublicKey(additionalKey), mockWallet.currAccount.account.GetKeys().ViewSKey)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(derivationKey[:], derivationKey1[:]) {
			return nil, fmt.Errorf("devivation key not equal")
		}
		index := 0
		otAddr, err := xcrypto.DerivePublicKey(derivationKey, index, addr.SpendPublicKey)
		if err != nil {
			return nil, err
		}
		spendPubKey, err := xcrypto.DeriveSubaddressPublicKey(lktypes.PublicKey(otAddr), derivationKey1, index)
		if err != nil {
			return nil, err
		}
		return []byte(spendPubKey[:]), nil
	})
}

func TestSubaddrSpend(t *testing.T) {
	type stest struct {
		rawTx  string
		subIdx int
		outIdx int
	}
	transTests["TestSubaddrSpend"] = []TransTest{
		{
			val: stest{
				rawTx:  "f906c3eb10c698b1d37308e3c180a0f25180b6bafb4b0d0bb0225f5dc8d51cf8bcde90a3f716d701d7c14b14dcbb65f8985842699137a517f843a0d1333d2364cf76cf0821121cd72a1b8d4686db61bab9f60fedd97d355f55473c80a039aacec821acc6dd7d02f1b34e79262ed6fa97e718b3bcc1817f2dfc29ee0ecc5842699137a517f843a02c70ed779355bffa2d287a6544b7a1a93efb3f3281ec024bef676d41eeb8b28e80a088cbc62f443397d96c0cd97b5971751ea07d9a1de3d8d5eb2bc3ed7c5ba504fb940000000000000000000000000000000000000000a0fdb9808761e8c80d24e2867acfa7d6e5ebe257c90e01e98893b857a9da76f559f842a062f8e2484ba144ff15d3a117bc76414f004ac7c60fcbe7cf506b7608873513c4a069d400c054000067a95130b9aa72adca3a2b98de77ad8338bc6be5616aafd6718609184e72a00080f84582e3e5a0023cac2b4ef0bbf46e3c6ab421f3245175ba2fecf0c0bc899626b8ad271b6033a02100b4085f1f60da06845d0e93d4e2e488cc321b23f3ffa2a1ec4e10f38db0dcf90531f9015903c0f8caf863a0748bfec8c57d47857f40ec855583646b24001c55fb634d2a2cb4ea3006562801a0e1715082e92776f7149bf01d3e4ba431d27f2a9a45a9ae925cf4c89a487f9d0ca00000000000000000000000000000000000000000000000000000000000000000f863a06f7c5a421f3a4b7b4dd29ac1d116553a46ac53666ffa49a65c7902b5f6b8900ea023c16f0256fdaaf81eaf6af3973a79c182c6a1620a3ef478009a6b52d9044d0ea00000000000000000000000000000000000000000000000000000000000000000f888f842a00000000000000000000000000000000000000000000000000000000000000000a07fe32d4fc37952f6eb1d92ce7297a9e0dc6e0a8bc8621dc11b20cfbb82521fa9f842a00000000000000000000000000000000000000000000000000000000000000000a0fa878714a896760d683c558ef67ec01b67a36b4b233dcfc30feab570a069b0bf80f903d2c0f90342f9033fa0d3a5245ae62775bc697d0d311ad523da7673aec21ccd9ca678d75734085ece81a00d7386831e4f0cf59b17a16c5ce06a72a01b6ee63a17e559bc648d764ba2ac0ca0cc63cf926fd7802b840ad2e022da74fe3b4e94c5741b3e3a3cb5f7d7786245eba01e3bef521f3d0deb4b4c145643ef04bf28a9a68989c6c1129c66dbaddb5c2e9fa0f717042714874a26129d6b0178d021222a1365ecd735ab9a98781cce5570be0ca02d9e65746418f1e2cbcdf9059e2b912e76a4366d7e36402c490157cf5cccf302f90108a012899b0365438434d32a95d6f98b1a36b859cbb0a1f825838c34e8b7f1cf46dda0cc7e8e5d99d492bee36d7390879c108df5469a7efb4e48a5378d9758e69f9934a0dafbc4105b6c140efbdd15bff9ba84f2ebb741aa0fb56ddbd9b0823f719f04dba03ce23b2d528e458c595c24246d58dd34fa0a5dcf12f68280eaea1b461c75c51da0dc0d56f766903202d0bfa28a176929b62ae9658e158409cf41774e427c801af6a0b6a0078584625aded3271a420c6b970a49d111ba1fd701802e4ffb25407fd9fda07912a55f815fb7c595fc6a6d6b6c82636bc7983f71d3cfcd2fd88a0b6693fc5aa0edcff7dabc2ff801aa2cb7617b33c318a3fdd9014ac3209abb3162b87462039bf90108a073b9f681151b3e707e965c1f1f64ec94d4c1ee8e2fa0c268fa5b60d8e616c244a0dc15c58cd61d46fc191677b9fd173f0296295cfdc1b6f3a700e1b950e014c747a045423023b27357c6e278fdf203d63f89df94fdae3f06a87d192d08beaf53d9b2a064a2c0f8bb5c3427ab65821819e8d726a0acebda853f34ead9ca81874fcc0d83a0b75a4a8e61fd490debdb92fe7428701db55d85be44b783007f59ed0f2d377457a085c8957c6b2a3b58484a38287fdcdd536865a47657c33a306ef258dceedfd214a082ead109163f37b1712d56988984ca3172fc8863ac0f5b0e16c2f0ada8f63b49a04169e0a7aae47cbec2612a9d1810dae0adc061757a78614fe60a33a0412628e5a04e010e2ff2c42e5ea945dd2d7819b6aee2038312facb62a5b9e4019ed775820ea045e10a0d50c89650b13972d62b70c3ee06e8a80ac0f6054e107ae0ceb39d0102a0f94c221e2eb86513ed6474b07db204ee889741ae2a8073d4107af025afd4d501e3e2c0a00000000000000000000000000000000000000000000000000000000000000000e1a03e46202d2ff85a2f75bb1980127cdf087bc7502c27b11044b22f11ecd236d9e8f844f842a05e6764a4fdb467fd8accdcf98f519e74acabed8315a6e4e82fa7376d883d7306a08db8e517d52906c3dadf2367dcb5dcc07cdb976a4e9805fa8a345921bb8cab0b",
				subIdx: 2,
				outIdx: 0,
			},
			output: "0000a0dec5adc935060000000000000000000000000000000000000000000000",
			error:  "",
		},
	}
	runTransTests(t, "TestSubaddrSpend", func(val interface{}) ([]byte, error) {
		st := val.(stest)
		tx, err := hex.DecodeString(st.rawTx)
		if err != nil {
			return nil, err
		}
		var utxoTrans types.UTXOTransaction
		err = ser.DecodeBytes(tx, &utxoTrans)
		if err != nil {
			return nil, err
		}
		err = mockWallet.currAccount.account.CreateSubAccountN(st.subIdx + 1)
		if err != nil {
			return nil, err
		}
		addKey := utxoTrans.AddKeys[0]
		derivationKey, err := xcrypto.GenerateKeyDerivation(addKey, mockWallet.currAccount.account.GetKeys().ViewSKey)
		otaddr := utxoTrans.Outputs[st.outIdx].(*types.UTXOOutput).OTAddr
		derivationKeys := []lktypes.KeyDerivation{derivationKey}
		_, subaddrIndex, err := types.IsOutputBelongToAccount(mockWallet.currAccount.account.GetKeys(), mockWallet.currAccount.account.KeyIndex, otaddr, derivationKeys, uint64(st.outIdx))
		if err != nil {
			return nil, err
		}
		if subaddrIndex != uint64(st.subIdx) {
			return nil, fmt.Errorf("subaddrIndex not expect, got: %d want: %d", subaddrIndex, st.subIdx)
		}
		secretKey, err := xcrypto.DeriveSecretKey(derivationKey, st.outIdx, mockWallet.currAccount.account.GetKeys().SpendSKey)
		if err != nil {
			return nil, err
		}
		subaddrSk := xcrypto.GetSubaddressSecretKey(mockWallet.currAccount.account.GetKeys().ViewSKey, uint32(subaddrIndex))
		sk1 := xcrypto.SecretAdd(secretKey, subaddrSk)
		_, err = xcrypto.GenerateKeyImage(lktypes.PublicKey(otaddr), sk1)
		if err != nil {
			return nil, err
		}
		ecdh := &lktypes.EcdhTuple{
			Mask:   utxoTrans.RCTSig.RctSigBase.EcdhInfo[st.outIdx].Mask,
			Amount: utxoTrans.RCTSig.RctSigBase.EcdhInfo[st.outIdx].Amount,
		}
		scalar, err := xcrypto.DerivationToScalar(derivationKey, st.outIdx)
		if err != nil {
			return nil, err
		}
		ok := xcrypto.EcdhDecode(ecdh, lktypes.Key(scalar), false)
		if !ok {
			return nil, fmt.Errorf("EcdhDecode fail")
		}
		return []byte(ecdh.Amount[:]), nil
	})
}

func TestRemark(t *testing.T) {
	type stest struct {
		rawTx  string
		subIdx int
		outIdx int
	}
	transTests["TestRemark"] = []TransTest{
		{
			val: stest{
				rawTx:  "f9050cf856cf4852c16c3232f84d8089367b2d3f4823940000a03e64d36c39e37447a55ce05251758ada8e94f81fe7e24fccff0647f7d717bc0ba0404001d693c78c8df81d3bfa4685bf3487ff4c0658ea16678f47905a52ec8990f84c5842699137a517f843a00283238bf5a719e65a0bca73a24b88775f199f3ab60721e3d04fa0a721de758280a0b3007102d8329eba56932be3125c63be6874b2af2222a9fae7df901bb0b8706f940000000000000000000000000000000000000000a0fca543b063c9f732e2225acb813ae6c0e93428684cc801ea4815734e1e65f566e1a0e152c6c7709cbc1346ebf040654f6aef39d5bf7bdc0ffad6272200db47c77886884563918244f4000080f84582e3e6a0fa08452d3ef9785a0ad1b25c13aaa811f729ecd01d266deda07399f17c2d7721a0374965385867b19e40eebcd75321ef2d5e083bfd1bc659aabdbe0b8a88f2f3c5f903baf8b080c0f865f863a08a0133537989e17900c6766fb6d17314a0e8cbeb2d331c5694062c769be42c0ba0fce26074aecd0d2b91bf2287adc01bb148af1841dc875d3b3759147913bea203a00000000000000000000000000000000000000000000000000000000000000000f844f842a00000000000000000000000000000000000000000000000000000000000000000a01fd09e66facd3a9509c2e26f5e44e8344c4ef67302fc16513b9dc546f0b1cc2e80f90305c0f902fef902fba03ce9d4be613bc3db0c82338442385d4f60cdc98f29e5927128f481f09a3fdb32a06f2f709398e6f9a2469b82d179ee44881072c0d96e7b6016cbe7a4c6da8fae7fa09ad888d8951b30580684c7b789c5ab868ede3b718eb3308b61b7665cc89075e1a0d548968b0ec73b3c0590938c18b1dde54dc2a09c36aad578411f0e329d465a76a018e2a2d904d138733fb1e0b25aad588fb53f53bc268c18c362d2938414f58f0ea01573cf7cecc516db0b680353b36d62ba3f07cedba5c751b5fd1c0dd224e15e03f8e7a089a4fb3646b9c5157700fef52cca8cd632ac78a3037287169cb8b738f7eedb7fa01cfd17bc1f9816a69aa09a99531c91e7ee69a90151467754ace8bcc4352fe0cda01087c26919191b63aebf08f2ca9741ab3fc249ca560dfcb7c9eb8a16bee6b312a02537995a24d7550782f4d73a3c7f65e7e64e39da946aee7a52a3b8e40f3949cda00991f9fc7f5f0c05db98331ab0a68adb64fe88069ba706fae9ee0e5e19d55832a02933b021e0a78f01a3991c547023bc18669616a6cecf289c8e8a763ecbc13af1a0edd3a5e0423ccedc9e61614427762b6ea3518aab709bba51cf22510befff7ab0f8e7a0e50c5469587ed1d98ddb20f1eb6c26b8675506920fecaf139400cab9d8c45ca9a0b27582f4148dacbb04f11d49150037fe90ffb07c0722363cb9c94a6d9ae61ff8a0ff0214c74ebdfdcceb2cab70748bb08d9cbebb67de974ec4c326205b9ef3a9dba015148f3dee1b5c464dec29302cf8247c3c953da2a49f69e09b5b18f69649c9ada08e78d9dd73113babaf3bdf5268afd0a57a689d0ba75f829adb8074ab1e37d789a0a850d05124ce8848ff19b61bfa0b688378f33a9ca4f0abac1df238af0591ce60a04337d79a4a6e9c07f0ae992f10976ad5b5e4a7229299ffc66a5924cfe8c0c972a0a0ad3119ad934c6be6139300ba7a64d832a0d57289d361ae94bebe16f9f61e0ca057a16161f5c97ad0c9c868c11b18f9be6c70107d60a2503c9ae8414ffdc3c005a0844772ef2644c3a23a2b78f2afd5059729a92f1eabe082846c8fcc7b08b52002c0c0c0",
				subIdx: 0,
				outIdx: 0,
			},
			output: "746869732069732061207072696d617279206164647265737320746573740000", //"this is a primary address test"
			error:  "",
		},
		{
			val: stest{
				rawTx:  "f9050cf856cf4852c16c3232f84d8089367b2d3f4823940000a00783257eb16af39869b05a41f65fcf540e9d21d2b451c243fa47663991539104a035d855bf3df388e78ab2e02bbcae4be5ce26a06b4ed29392e576665f9da30f92f84c5842699137a517f843a0cff156b9dfbd63284c45d5b0b2743de93436fe2e84d878c2c5640b029249573b80a053beb5b87876269046865a996024546f38a800c135f3bf379be441ccc75778c8940000000000000000000000000000000000000000a0039f301ffae3f59e82aa27eb43e7da4f95542e467d072c7e6602db9b13c27bcce1a052d44b411a426ddf92a0e4a10384e8b54dce05e304935aca6291db9f4ea0cc14884563918244f4000080f84582e3e5a0a0de3ba15db123cccc539f55775eefc2bd969e0209d72fc28aac6bccc3e00ce1a03b3b1a5ef1212e372389b2c2e28784d099b310d9f1973951524b59bb0d914ecef903baf8b080c0f865f863a0126a7ceecde46b2885fce88989ee8b65576a9584cad89c88e92ca80559ab090da0fa8940f8f9120c0076e03871b44e490a089ef0162f3e8fc580bcb90617c9fb0ca00000000000000000000000000000000000000000000000000000000000000000f844f842a00000000000000000000000000000000000000000000000000000000000000000a039ac102e1f3171a209a4748ed1d6f8033e44f9f7d9e136eee531a2c4ad483a4280f90305c0f902fef902fba0ace7b0d495b2e434306d401e96b0cb24a2e39067f9bdc014a7291557d54f2ebfa02b8c8f0ad6c6daf5cedd338fd0487efb92c747c712856e5b2c365cc1100d9d47a0e624dab15a7668dc129b5dfc25f1f4358d587684f99b79c0c13ed5815a2a596fa0bbfede0e497a5da0d15d8b1fcbb0e83cb92bb43b34c40fd8b730e8dc062c7cb0a0ac4ec2afadad922f65d4ea758026a93f8b7b5f5e38d58a269d7a58818daaca0fa095accc850a7b799b2c583dfcdd7fdd86d1242e9523309d82dd76f2096fb6820ff8e7a09e6ed2c69b7260d76994e64a8bf0812a5c3f4f08f9b86cdf10da529226b8261ba0a20e518fff0fc6a5fca6472b16b9a0978208074cb0a7a6e861860ea91f653841a0699c581c848212c4e90ca21c201f89270b36d6d7444952ed342a953c2f8ffddfa09576c48c9ad0aab9d282894ea31f8e98c360e303c52e18786e312b4c4f9aff63a0a5fe3ca722f3e5c3b31f4ed1e793ec2b1d5b0c57c60e61b9ac3cb0d046471774a02caf9652d0bee9687657d2ac7b4e6552c627963df64eb4fbe24dc01b1484949aa02827cb3e7942e092539f15cb37ab5dd365aa25b2fcf446925b546719d164147ff8e7a093814fd34a68b43ee69febae814e5378575e6db649befd4b92ca74049e5e7310a0a67f9283fe8946bc199f92b48f60ec6543ba78c71239afb230cfe7a96280591ca042b775506aedfd33d80c1fb139a875050d793d6cc46057e21c3f17a3e8423df7a0a02109e3ba4aa00fef094b0eb6d9a99758757db6dc6bf3719f8bcc753fa711bea0e8a63c1593f10e5b881284c97cbfee1e3b0e3feaaf222f2cc8fced2ef2618644a029d4ce542b99a46fe2c84a13befe6f7f545a36c93a3b268b4f1511b126cae5a0a0e585361848c96a9d302dd5d5fe3a079a50f506d1164c397627fad32140b506d3a0ca6a9895d2584e4e28a36d210dce78e4f49575e0b8b77a86d52beafba23cd007a0ff3d497926fd1e31006568dae1a3b2e5a760cf2c2e6bc4d0d4978e11d98c8902a009c53c52102f1804331aa9a57adb283eab374177ee8e74e519207107d7f8e301c0c0c0",
				subIdx: 2,
				outIdx: 0,
			},
			output: "7468697320697320612073756261646472657373207465737400000000000000", //"this is a subaddress test"
			error:  "",
		},
	}
	runTransTests(t, "TestRemark", func(val interface{}) ([]byte, error) {
		st := val.(stest)
		tx, err := hex.DecodeString(st.rawTx)
		if err != nil {
			return nil, err
		}
		var utxoTrans types.UTXOTransaction
		err = ser.DecodeBytes(tx, &utxoTrans)
		if err != nil {
			return nil, err
		}
		err = mockWallet.currAccount.account.CreateSubAccountN(st.subIdx + 1)
		if err != nil {
			return nil, err
		}
		derivationKey, err := xcrypto.GenerateKeyDerivation(utxoTrans.RKey, mockWallet.currAccount.account.GetKeys().ViewSKey)
		if err != nil {
			return nil, err
		}
		derivationKeys := []lktypes.KeyDerivation{derivationKey}
		if len(utxoTrans.AddKeys) > 0 {
			for _, addKey := range utxoTrans.AddKeys {
				derivationKey, err = xcrypto.GenerateKeyDerivation(addKey, mockWallet.currAccount.account.GetKeys().ViewSKey)
				if err != nil {
					return nil, err
				}
				derivationKeys = append(derivationKeys, derivationKey)
			}
		}
		otaddr := utxoTrans.Outputs[st.outIdx].(*types.UTXOOutput).OTAddr
		remark := utxoTrans.Outputs[st.outIdx].(*types.UTXOOutput).Remark
		realDeriKey, subaddrIndex, err := types.IsOutputBelongToAccount(mockWallet.currAccount.account.GetKeys(), mockWallet.currAccount.account.KeyIndex, otaddr, derivationKeys, uint64(st.outIdx))
		if err != nil {
			return nil, err
		}
		if subaddrIndex != uint64(st.subIdx) {
			return nil, fmt.Errorf("subaddrIndex not expect, got: %d want: %d", subaddrIndex, st.subIdx)
		}
		scalar, err := xcrypto.DerivationToScalar(realDeriKey, st.outIdx)
		if err != nil {
			return nil, err
		}
		hash := xcrypto.FastHash(scalar[:])
		for i := 0; i < 32; i++ {
			remark[i] ^= hash[i]
		}
		return []byte(remark[:]), nil
	})
}

func TestBigVerifyProof(t *testing.T) {
	transTests["TestBigVerifyProof"] = []TransTest{
		{
			val:    big.NewInt(1e18),
			output: "00",
			error:  "",
		},
		{
			val:    big.NewInt(0).SetUint64(1<<64 - 1),
			output: "00",
			error:  "",
		},
		{
			val:    big.NewInt(0).Add(big.NewInt(0).SetUint64(1<<64-1), big.NewInt(1)),
			output: "00",
			error:  "",
		},
		{
			val: big.NewInt(0).Sub(big.NewInt(0).Mul(big.NewInt(0).Add(big.NewInt(0).SetUint64(1<<64-1), big.NewInt(1)),
				big.NewInt(0).Add(big.NewInt(0).SetUint64(1<<64-1), big.NewInt(1))), big.NewInt(1)),
			output: "00",
			error:  "",
		},
		{
			val: big.NewInt(0).Mul(big.NewInt(0).Add(big.NewInt(0).SetUint64(1<<64-1), big.NewInt(1)),
				big.NewInt(0).Add(big.NewInt(0).SetUint64(1<<64-1), big.NewInt(1))),
			output: "",
			error:  "verify fail",
		},
		{
			val:    big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(1e10)),
			output: "00",
			error:  "",
		},
	}
	runTransTests(t, "TestBigVerifyProof", func(val interface{}) ([]byte, error) {
		amount := val.(*big.Int)
		amountKey, err := BigInt2Hash(amount)
		if err != nil {
			return nil, err
		}
		outAmounts := make([]lktypes.Key, 0)
		mKeys := make([]lktypes.Key, 0)
		N := 8
		for i := 0; i < N; i++ {
			outAmounts = append(outAmounts, amountKey)
			mKeys = append(mKeys, ringct.I)
		}
		proof, _, _, err := ringct.ProveRangeBulletproof128(outAmounts, mKeys)
		if err != nil {
			return nil, err
		}
		ok, err := ringct.VerBulletproof128(proof)
		if err != nil || !ok {
			return nil, fmt.Errorf("verify fail")
		}
		return []byte{0}, nil
	})
}

func BigInt2Hash(amount *big.Int) (lktypes.Key, error) {
	if amount.Sign() < 0 {
		return lktypes.Key{}, fmt.Errorf("amount invalid")
	}
	var (
		i      = 0
		value  = big.NewInt(0).Set(amount)
		big0   = big.NewInt(0)
		big256 = big.NewInt(256)
		key    lktypes.Key
	)
	//amount only support 16 bytes
	for value.Sign() > 0 && i < 32 {
		key[i] = byte(big0.Mod(value, big256).Int64())
		value.Div(value, big256)
		i++
	}
	if value.Sign() > 0 && i == 32 {
		return lktypes.Key{}, fmt.Errorf("amount invalid")
	}
	return key, nil
}

func TestOneRingMember(t *testing.T) {
	type stest struct {
		Secretkey string
		Publickey string
		Keyimage  string
		Hash      string
	}
	transTests["TestOneRingMember"] = []TransTest{
		{
			val: stest{
				Secretkey: "6f0e57a8cda67088f850cfd5d9fffeb9afc4b8fee8bca7fea7503fe6f6ea7d09",
				Publickey: "3f64263b783282baf63cf33a008e41dacc734a06e43b29dd5fd15b6aed1c0f78",
				Keyimage:  "03896b4070b318f5555494b1a2816029622f41b0ba72cc571b614532d74324b0",
				Hash:      "1a3b597ce825eddc60a6441ee926503202ab592158542b72f1f8765d89e8ae00",
			},
			output: "00",
			error:  "",
		},
		{
			val: stest{
				Secretkey: "bf425dc7d1826e6d57b03db518a8dba56fd16d95886ea927115ece96d80af107", //fake sk
				Publickey: "3f64263b783282baf63cf33a008e41dacc734a06e43b29dd5fd15b6aed1c0f78",
				Keyimage:  "e30b44960ff3597744003cefd0de4bc7b16dbd6e402fa653dde103e4069c57a1", //fake key image
				Hash:      "1a3b597ce825eddc60a6441ee926503202ab592158542b72f1f8765d89e8ae00",
			},
			output: "",
			error:  "check fail",
		},
	}
	runTransTests(t, "TestOneRingMember", func(val interface{}) ([]byte, error) {
		st := val.(stest)
		bz, err := hex.DecodeString(st.Secretkey)
		if err != nil {
			return nil, err
		}
		var sk lktypes.Key
		copy(sk[:], bz)
		bz, err = hex.DecodeString(st.Publickey)
		if err != nil {
			return nil, err
		}
		var pk lktypes.Key
		copy(pk[:], bz)
		bz, err = hex.DecodeString(st.Keyimage)
		if err != nil {
			return nil, err
		}
		var keyImage lktypes.Key
		copy(keyImage[:], bz)
		bz, err = hex.DecodeString(st.Hash)
		if err != nil {
			return nil, err
		}
		var hash lktypes.Key
		copy(hash[:], bz)
		pubs := []lktypes.PublicKey{lktypes.PublicKey(pk)}
		sig, err := xcrypto.GenerateRingSignature(lktypes.Hash(hash), lktypes.KeyImage(keyImage), pubs, lktypes.SecretKey(sk), 0)
		if err != nil {
			return nil, err
		}
		if !xcrypto.CheckRingSignature(lktypes.Hash(hash), lktypes.KeyImage(keyImage), pubs, sig) {
			return nil, fmt.Errorf("check fail")
		}
		return []byte{0}, nil
	})
}
