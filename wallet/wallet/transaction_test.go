package wallet

import (
	"bytes"
	"encoding/hex"

	//"encoding/json"

	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/ringct"
	lktypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/xcrypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
	wtypes "github.com/lianxiangcloud/linkchain/wallet/types"
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
		Logger:    log.TestingLogger(),
		account:   mockAccount,
		walletDB:  walletdb,
		Transfers: make([]*types.UTXOOutputDetail, 0),
		txKeys:    make(map[common.Hash]lktypes.Key, 0),
	}
	mockWallet = &Wallet{
		Logger:      log.TestingLogger(),
		currAccount: linkAccount,
		accManager:  am,
		walletDB:    walletdb,
		utxoGas:     new(big.Int).Mul(new(big.Int).SetUint64(defaultUTXOGas), new(big.Int).SetInt64(1e11)),
	}
	mockAccount.EthAddress = common.HexToAddress("0x54fb1c7d0f011dd63b08f85ed7b518ab82028100")
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
	ecdh := &lktypes.EcdhTuple{
		Mask:   utxoTx.RCTSig.RctSigBase.EcdhInfo[0].Mask,
		Amount: utxoTx.RCTSig.RctSigBase.EcdhInfo[0].Amount,
	}
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
			val:    "byF93XiVh8tP7CVsDS1Jt91sgCkhWzRBrqQ1UaygKuYE4pXM8HxnLMEXz2H9PdjFzqX7ozBJ6i2exvJdsMoKsU9zoMTG9V",
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
	raw := "f9063deb10c698b1d37308e3c180a0fa2042f28e79a2f279c0ecf68567be051dd3d329bb1987747b3f088b0d868740f8985842699137a517f843a08435fd1d0a9f209e5b7e1d322b7687bc2c8939077b6cc10cc1c7957bc1dd11d580a05384cb7608ecef955c680a4975701b907db0c37c401cc5eb86e6b0480cb24f415842699137a517f843a0445b80783b3d3bf0602782a67f972ef3d62adb851c031a518628d32c4a0fda3f80a094d516d752efbbc103a41398258638ca8f1f499318910d57fbe1bd7feb0da3fb940000000000000000000000000000000000000000a04f1ea6d09b8743cfbc9b512035ac3041874b06966e119753d32faedc2bb67443f842a044f5a10838dc58d270ccb02c95b0779faac1071ce59d37339726ed6e4b3da4f2a026a532290639fd11c255786ee94a429597088df9b5879aa4792edfa29d1e0a7d87b1a2bc2ec5000080c3808080f904edf9015903c0f8caf863a019defdf2f5aa49d94058838ee16e5e5b78edf1544f75237a59fd25089360eb08a0cf3aab8ac8fc70f4d6354fa5a084c3e56da5449662af3e0a0558b50989be0203a00000000000000000000000000000000000000000000000000000000000000000f863a0ee0963a616f3d0883b3ae90796c154420d5330fa40a94090e54b7e49726e6408a0f664057e253da1bd300d7e587201e3dd313ec3d9b1d87b16b17fdb5809e94406a00000000000000000000000000000000000000000000000000000000000000000f888f842a00000000000000000000000000000000000000000000000000000000000000000a0b0ac54da56b8d0bc4b12bbc9b0c4b2aec11658872ff16fb9f30698571dd53bf6f842a00000000000000000000000000000000000000000000000000000000000000000a0d4082278747e739eb58f3b4abd9625b9caada94fa7ed8ae2cd664fc42a0d5d1c80f9038ec0f902fef902fba087509cf003efe1df888e37358fa8843d861372c12c72e48928a5822eb35f9a10a082eff229d74b0730ba4a78ed1a77b757aa7b5fb9ea576515ca94673edfe14b37a00e98eccc0a952749c324ebcc2f6ba5b3896ad222257300536038ee029f764730a0afd56c91cac5963bdb5eae06e0260ca5e8497cba84536685f099fc83469ef857a0ad2b1ff7f8f20ba8aafc4fda690443db3e414c7c2389372d8b9e0ebb82bf9802a08fe7409113f91f16c85fe681116b2664890c6a67475841f2cd1f09bf5523bf0ff8e7a0547eb9da2f599b1dfb81571b3b123e7e6ebaf64ac89002235666de9eb55b56b9a0a0298308cb679ecbb1faec4c41d4b604a63721dfac8b50d2597fea3e579aa3ada0e75df745130dfc7ba7ad31e0207ef5fc121268a8b54075572aeb0e5b7bc524f5a0eabaefe5a86cf37283119eaae223944c458a0b90f87c1c7db45f700e4db723a9a054c5d49d76d80a7383a313ca83cffed0cb951eb6da9e06a574628e1eebc2d6aea0a5f947f10fafbb022d1a331ce80cde4549c1b34f6503c924f6e2dbd1dcdece7ca0f7ccd35758758cf771a6b552d1ffb499c592d08c4f3fce3dd39585be6589d009f8e7a0c5d149d18f68c680b0b3856912ecccb360a505efdae547de4414910b10a518c5a00d9276c93fb5c74433a5d70b7aba097a51dd7cd77cc1654655ad94e8df434a8fa0369f342e402b47bc4689f72ca8a184b219dbea7bc6972850abd6c53452bda934a06badd7c7162ac9af5fa9fe46095570659ffe7beb56e5ddc8b21e8b05f6dd1b72a0967a0c41db92f830a0b854cb30de35053d06a8933d46a2591521c7df612c510ea04709de973e162f3506682fb2508001d047c8dac4702a4ab625c3e0d5b4eca49ea0b8b9672660072ce44a0483525eb10c3bd74b8b80b1a0af68ffee609357fe65ada054419922d73c269466ec910ce89958caf85aab6fa120f554fd6f3d4c7e0e8c09a07e4dda499d53a702e2c4f54a1d412bb9eccefcb1fb7475c1dc9c29085a82de09a0ca656921d627e647215c176cea4a647a79dfbb2b2f801b6a9a89b2b489b19003e3e2c0a00000000000000000000000000000000000000000000000000000000000000000e1a0aaee6a4ffca6a09ab9a8c2179e4ddd1857178f86b967050ead8612b7b3c74c34f844f842a05bf6694964ba9ac0f99900f96336d7ee541580c51a6715501882766c5257c908a035c20e9e16e130a08fb04e218bd8da9dd14519ec8f14f82839c3f4eb3ba4d30b"
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockAPI := NewMockBackendAPI(ctrl)
	mockWallet.api = mockAPI
	mockWallet.currAccount.api = mockAPI
	gomock.InOrder(
		mockAPI.EXPECT().Transfer([]string{"0x" + raw}).Return([]wtypes.SendTxRet{wtypes.SendTxRet{Hash: common.HexToHash("791ef44b47090bfa1f6c81a3629d5a6dd92b3a9a57937520dc5d63154ed5a85b")}}),
		mockAPI.EXPECT().Transfer([]string{"0x" + raw}).Return([]wtypes.SendTxRet{wtypes.SendTxRet{ErrCode: -1}}),
	)
	transTests["TestSubmitUTXOTransactions"] = []TransTest{
		{
			val:    raw,
			output: "791ef44b47090bfa1f6c81a3629d5a6dd92b3a9a57937520dc5d63154ed5a85b",
			error:  "",
		},
		{
			val:    raw,
			output: "",
			error:  "submit transaction fail",
		},
	}
	runTransTests(t, "TestSubmitUTXOTransactions", func(val interface{}) ([]byte, error) {
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
		txes, err := mockWallet.CreateUinTransaction(from, subaddrs, dests, common.EmptyAddress, nil)
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
				addr:   "oVsKCEVEtjvd116pQ6h63T1Q49b6f3hThbpXtY1ebw6Tfk6E1nJfSiV6x3s4MthXBK7X35TcnmeYvTq4ZGWgD7F3xw5DTM",
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
				rawTx:  "f904c8f855cf4852c16c3232f84c80880e92596fd6290000a0f68b88cd483ae2b6c66d6943032442789a07a038de220115819462ff751a6706a0ff61c825db9639041f098804d2daf04bc557e8b278702db7c6c0a31cff65664df84c5842699137a517f843a091b2bcc86d72975292260619c7b4ffb82d4408e7596307aa65eec3b94e819e3180a011efc38b3911fedb2e472c0cd14b94ffa68be360a582280b897591958c9f997d940000000000000000000000000000000000000000a05e7d18bdf19ed03cfd2ac4e86a2cbdc57977fbd3e6306e9a809b8b8f7ae99c31e1a0228afd55bca062e5c6833e87cc1717f169e004d2f53a7bf39ec9d3dfd51ad28887b1a2bc2ec5000080f84582e3e6a0ec5426caa615067b85fe80ed7c903ec591994f08eaa4d167f819a033212efdc5a0404667aa213b59b02c71e1b84e0bfff45ac7a5441e929f22a3329e687647d58cf90378f8b080c0f865f863a0b46025a7551f3ac0bec7000e43a68278e424c11fc1560a6873144fa47e02d60aa09246191cb4673c287d6ff22ca12ad5bcfe7f720ee4b202cf7c104d764203490da00000000000000000000000000000000000000000000000000000000000000000f844f842a00000000000000000000000000000000000000000000000000000000000000000a085e4302fd0c0564b9b7a790ab419fc0b98f6f475f9296c07a79f65d25574b0f880f902c3c0f902bcf902b9a0787b8abf186b33cd812d1b27948accd9d39a12c55b3305159e2f56c2ede3c4dfa0eaa9efaec009ab1403b3629e4cf9faaa4d4786d913abe5a4fc08299606f312f3a0fe0de2774b537546b2b73cf4dfff7bd960168e7471a031fcc49bf0e7501ca998a0cd2e2e218483ba5059965e33d2aa1ace33b716a03c657ee7391d524cb19d1b00a0809f129d36eadf7a66415d4c69e084372c025ae803d74a23147ff5d9f976ec0ea010421ec30c629260af03281def84ddaa08bc604d134e35f724f3c99c5cbac40af8c6a03929a933248fc9dcef02e9bddcfefe1941e129fa0132727fcae89a402db03808a022e04f85e38103cbe455abbfb01b240d6ba418b75d027eedf6b5ef5dc33f6f3aa0348277f32252f37efb26bc3932977682ff5b86f9c11b050bbe0d81846ceeaf4aa0805d928dda03886c9940e550452aa6a83fe0e5b13a08ce006d5cb0bbe564d1cea06fa2929d0e8e18c006f035a806cfd51119517609c2cfd07bcf94638b4f002ac9a0dbb7aba5d0627803ff8c545d41e5ce45522a3536d9f7e8ae737cd87c6e426725f8c6a05ad3b433bc4bb98a950e97999886703f8cd9e889593efff3a7acbee3eac93c0ba06aee964a581cdcdc053f5cb8664c51d02f6894127642a3d82ea5d56aadffe9d8a020b7c571e23a9044f91da59d69d2f404ebfa3049e5553cf12c3bfbfd7c5cdc76a0d0d8e0cc727bd9f64a16ea60621f5e981f7e5311b5edca40d75c406e834e7957a0a1cb8b8cc1d68934a1c0f8985cdc7aa1fe98a6ed1bce15ad5081a6fb8014c1bba0ce67ad5e02e610e09dc1b11dffd64eb23028e6108f696d7e6a2fcc40554cc692a0a20b0386a3248622cc3ea3fbc96ed89f1d1621b9ced075f8aea91567eabde003a0d647994a58c88993a4f2688cbc3bcb7be3530778f800d749ef1aeeaccd7fc80da0c716d5ad93cf408aaf2ca43cfdc251b2ed6ba8388a4be05ef12a04852f942b0bc0c0c0",
				subIdx: 0,
				outIdx: 0,
			},
			output: "746869732069732061207072696d617279206164647265737320746573740000", //"this is a primary address test"
			error:  "",
		},
		{
			val: stest{
				rawTx:  "f904c8f855cf4852c16c3232f84c80880e92596fd6290000a099a2c79818f081d16015df538590c156da0d602f6a6dc9a4e8ffcce5d7234100a0b09f912b0b00458708cedeb1addb0d75e51eea279cc76fc13ed7a12dacbb4bbdf84c5842699137a517f843a03303197b2e2a39d169d3246de96db69dcd2522e5276962e9a25afe5c1b7a26c680a0e7e2758578bd71ab3d95a8e0b736ed9701c25a663799f455dd304564970d065e940000000000000000000000000000000000000000a06c57173dfa10efa53bf9a9f00cd0729472105f315587da56473dca32c36f5579e1a0ba4941d521e7caa93f3229d9d396a84981d13d93cb62502b0b8e203fe312729587b1a2bc2ec5000080f84582e3e5a0b36a2da11cde13cc89f3d58ec34df227cca001a1cdcbfedd441ad355dc251ebda06a512755c2af98256adba13caaec4b8606d71b82ac632349d375121f44b6a794f90378f8b080c0f865f863a006243a92b31b4525d358cee5fba9a994145d9aebf01f3e1bd0c91b35c1ba6500a0b72635b3f47b16dcfbed8ee2768418f6355c097f6ba8cc9f513a34cf37c1ff04a00000000000000000000000000000000000000000000000000000000000000000f844f842a00000000000000000000000000000000000000000000000000000000000000000a02c23ed0d003608d21ff0cc461dea3b707d8ea21bbd8166ccc9e03857ad0c4ae980f902c3c0f902bcf902b9a00696516718789369ddf4d1fb4ee3b6463520edb46be97b585aae8f5c67a147ada08197e3a03882c16df425053842064c88f582cbe0d6d84cc295eac6ab71b7dca5a0603df43e602f36ab1827177cfaa5df8882b628a2d5744929f2207ad734af7eada06d3dd970f2f49a677bb8f297808496cfcaad4866e4ad29d504c8ce8030c908f8a0a5c712391f464b365301a6f5a6db702a7dd9e8399a7a24ea825563d47e0a2301a0ffbf82fd9d104194f89ad03f765b6deebc40db8d6a9e66a0883a9c05670e0707f8c6a05f66ea59559a21187373e5d1d702140a5a40371f013b7077f5ee6c00d5ceed73a083e92bd38451af1f657fb17c4ce5289a54707a569b76255ce0f3b200b71b435ca0c8050af0165c9648b86e010bcfc74fd78ca73ac235906c30bfbf0a994caa47eea0dcfeddef1f2b679e543227c51ee99a9ec7a93712ba48a3296f617840059f7cd5a0e2c9d624b4ccc0c6f2588b25902a21aa1f7e3528ca70bd018bee5b08689c2e30a01b363411708c6aec4f6afd2e479533c577bb5606cd75ae5445f5bc26f51c7c15f8c6a02f31fd3b83d8b4140b91978b9949891c39b9f97dec6f5ff86e141e6dab18c509a0539667fe84da0f185c93d529fa8a5784d6f320da5c4fddbd8895e8290b02fbe0a0f79fea758f6070f081b17d3214b36af76456178bb91b14d1e17d45500380d419a07519b3066233173b7c302093089558070434d20a0703b38b08a5afabda9005eea083c3160637adecb0ae1bfde289913327b529706cd3d1062ae575ad1c345ab4aba076dda415d2ab5dd4a2cf04674bf6845e80b3fb2201046885048ce232ef5d366ca02beacfcb0bef3ddbc5fa35b7fc70e003507aef9f13b5b4600f2ce5c07688e207a0e84b187455bdc607a218f4d9095f84f4393a3950dace9f2e9b5610ba67262203a06fb915ddd1d88bad53248b80ac2968774ed1c7cc8520139e1513700ea8b34e0fc0c0c0",
				subIdx: 1,
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
		hash := crypto.Sha256(scalar[:])
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
