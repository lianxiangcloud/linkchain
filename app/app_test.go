package app

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/lianxiangcloud/linkchain/blockchain"
	"github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/metrics"
	"github.com/lianxiangcloud/linkchain/utxo"

	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/libs/txmgr"
	"github.com/stretchr/testify/mock"

	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/db"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/types"
	//"github.com/stretchr/testify/assert"
)

type ks struct {
	key string
	pwd string
}

var kss = []ks{
	ks{
		key: `{"address":"54fb1c7d0f011dd63b08f85ed7b518ab82028100","crypto":{"cipher":"aes-128-ctr","ciphertext":"e77ec15da9bdec5488ce40b07a860fb5383dffce6950defeb80f6fcad4916b3a","cipherparams":{"iv":"5df504a561d39675b0f9ebcbafe5098c"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":262144,"p":1,"r":8,"salt":"908cd3b189fc8ceba599382cf28c772b735fb598c7dbbc59ef0772d2b851f57f"},"mac":"9bb92ffd436f5248b73a641a26ae73c0a7d673bb700064f388b2be0f35fedabd"},"id":"2e15f180-b4f1-4d9c-b401-59eeeab36c87","version":3}`,
		pwd: `1234`,
	},
	ks{
		key: `{"address":"e6a36f2e34afccdd93c8e657a9795d5d26fb3344","crypto":{"cipher":"aes-128-ctr","ciphertext":"5e759f5ddfed547733832efea4fd46d2df12c6c80430e9ab26823b3f19f2edd2","cipherparams":{"iv":"c5c54ea1db594a447afd1f0dff178345"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":262144,"p":1,"r":8,"salt":"5dd9ac7552cdde1dc4e0867b52d5b9d870a3c862323cbb800baca3b979100cd0"},"mac":"e48a53fbb4ca94ea5b3492acb1eca39afdd5f179bc95f13ce36f8d401fe55f4f"},"id":"ae1c927f-ebd3-45b5-88c0-d633bce79d02","version":3}`,
		pwd: "1234",
	},
}

var (
	kssmultisign = []ks{
		ks{
			key: `{"address":"3f76ec08843942fd164c66507c05bef8f8b7df70","crypto":{"cipher":"aes-128-ctr","ciphertext":"fb7ab9a926785eda97e77ef04f7496063922943236254192f28c2b7a786ceee3","cipherparams":{"iv":"4f5f25711b58361c0747122a41cf52f4"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":262144,"p":1,"r":8,"salt":"be0916a282b34b70a8882bbf9ec2dabbf8fe6374a3271130eadf86f715c78e82"},"mac":"84f22cb1f74adcf33463f4fdce73877e7466dbc936c93d7c0ebcf408b82bf8e9"},"id":"ff528baa-e996-48a7-9650-88c99073a8cc","version":3}`,
			pwd: `1234`,
		},
		ks{
			key: `{"address":"5f502c6a99fd83093625b54a1bf1166bdf597660","crypto":{"cipher":"aes-128-ctr","ciphertext":"c95b4b4a38f14b91d28a85aae3f6eabf1b3bdf58dabaddd43c2c387b911e3e0f","cipherparams":{"iv":"bdb2650473ad9fd3c8cd877d807c95e0"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":262144,"p":1,"r":8,"salt":"bbfd32589e1b2a104d0eb0fe500f341f221d10cb40006c7a548993189274b7f5"},"mac":"dd938504d8bd6358c8309d4ff1e42c2631d6a84f2e8c6dfb3853cdaab247fe2f"},"id":"3c3a15e6-77c4-49c5-b8b4-f9fe29ecfbd5","version":3}`,
			pwd: `1234`,
		},
		ks{
			key: `{"address":"599bb2d47f605b5e655609c13cdaa1450f6b73a0","crypto":{"cipher":"aes-128-ctr","ciphertext":"c04dfbbfaf5ef6b6ecaa5eae416bbe960d5b341f63cde87763ee9818f00cb6c3","cipherparams":{"iv":"8c2901a11037b8680ca1c1cfbe5878d3"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":262144,"p":1,"r":8,"salt":"4110345e538327bf70b52674299fb5e6264759b1a0c007406180dc4476f9e48d"},"mac":"052721103822ec1ad9eabfb975300574b2221452529f063a1cead84b3abebde5"},"id":"31bf3b76-9a4f-455a-9484-cb7cd619773e","version":3}`,
			pwd: `1234`,
		},
	}
	acc []*keystore.Key
)

var (
	accounts            []*keystore.Key
	initBalance, _      = big.NewInt(1e18).SetString("0xfffffffffffffffffffffffffff", 0)
	initTokenBalance, _ = big.NewInt(1e18).SetString("0xfffffffffffffffffffffffffff", 0)
	gasPrice            = big.NewInt(1e11)
	gasLimit            = uint64(1e5)
	zeroAddr            = common.EmptyAddress
	amount1             = big.NewInt(1)
)
var (
	blocksNum     = 10
	txNumPerBlock = 10
	coinbase      = common.HexToAddress("0x0")
	testToAddr    = common.HexToAddress("0x3")
	tokenAddr     = common.HexToAddress("0xc7c22a8e08d3b0643a55e7c087a416171b45922f")

	logger  = log.NewNopLogger() //log.Root()
	gasUsed = new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), gasPrice)
)

func init() {
	for _, k := range kss {
		key, err := keystore.DecryptKey([]byte(k.key), k.pwd)
		if err != nil {
			panic(err)
		}
		accounts = append(accounts, key)
	}

	for _, k := range kssmultisign {
		key, err := keystore.DecryptKey([]byte(k.key), k.pwd)
		if err != nil {
			panic(err)
		}
		acc = append(acc, key)
	}
}

func getPriVals() ([]types.PrivValidator, *types.ValidatorSet) {
	validators := make([]*types.Validator, 0, 10)
	privs := make([]types.PrivValidator, 0, 10)
	for i := 0; i < 10; i++ {
		v, p := types.RandValidator(false, 1)
		validators = append(validators, v)
		privs = append(privs, p)
	}

	sort.Sort(types.ValidatorsByAddress(validators))
	vs := &types.ValidatorSet{
		Validators: validators,
	}

	if len(validators) > 0 {
		vs.IncrementAccum(1)
	}

	return privs, vs
}

func initApp() (*LinkApplication, error) {
	_, valset := getPriVals()

	stateDB := newTestStateDB()
	txpool := &Mempool{}

	blockStore := newTestBlockStore()
	crossState := newTestCrossState(blockStore)
	//crossState := &MockCrossState{}
	blockStore.SetCrossState(crossState)

	sk := crypto.GenPrivKeySecp256k1()
	pk := sk.PubKey()
	metrics.PrometheusMetricInstance.Init(config.DefaultConfig(), pk, log.NewNopLogger())
	metrics.PrometheusMetricInstance.SetCurrentProposerPubkey(pk)
	metrics.PrometheusMetricInstance.SetRole(types.NodePeer)

	app, err := newTestApp(stateDB, txpool, blockStore, crossState)
	if err != nil {
		return nil, err
	}
	app.SetLastChangedVals(0, valset.Validators)
	return app, nil
}

func TestApp(t *testing.T) {
	priVals, valset := getPriVals()

	stateDB := newTestStateDB()
	txpool := &Mempool{}

	blockStore := newTestBlockStore()
	crossState := newTestCrossState(blockStore)
	//crossState := &MockCrossState{}
	blockStore.SetCrossState(crossState)

	app, err := newTestApp(stateDB, txpool, blockStore, crossState)
	if err != nil {
		t.Fatalf("initApp err:%v", err)
	}
	app.SetLastChangedVals(0, valset.Validators)
	txs := make(types.Txs, 0, 3000)
	for i := 0; i < 1; i++ {
		nonce := app.checkTxState.GetNonce(accounts[1].Address)
		balance := app.checkTxState.GetBalance(accounts[1].Address)
		fmt.Println("###### nonce", nonce, balance)
		to := common.HexToAddress(fmt.Sprintf("0x%x", i%64))
		tx, _ := genTx(accounts[1], nonce, &to, big.NewInt(1), nil)
		txs = append(txs, tx)
		fmt.Printf("%s: %s, nonce: %d \n", tx.TypeName(), tx.Hash().Hex(), tx.Nonce())
		if err := app.CheckTx(tx, false); err != nil {
			t.Fatalf("CheckTx err:%v", err)
		}
	}

	for i := 0; i < 1; i++ {
		nonce := app.checkTxState.GetNonce(accounts[1].Address)
		to := common.HexToAddress(fmt.Sprintf("0x%x", i%64))
		tx, _ := genTokenTx(accounts[1], &to, zeroAddr, nonce, big.NewInt(1), 0, "")
		txs = append(txs, tx)
		fmt.Printf("%s: %s, nonce: %d \n", tx.TypeName(), tx.Hash().Hex(), tx.Nonce())
		if err := app.CheckTx(tx, false); err != nil {
			t.Fatalf("CheckTx err:%v", err)
		}
	}
	//types.TxUpdateValidatorsType,
	//types.TxContractCreateType,
	for i := 0; i < 2; i++ {
		mstAddr := common.BytesToAddress([]byte("mst"))
		nonce := app.checkTxState.GetNonce(mstAddr)
		tx := genMultiSignAccountTx(nonce, types.SupportType(i))

		signBytes, err := types.GenMultiSignBytes(tx.MultiSignMainInfo)
		if err != nil {
			t.Error(" GenMultiSignBytes error!!")
		}

		sigs := make([]types.ValidatorSign, 0)
		for _, v := range priVals {
			signature, _ := v.SignData(signBytes)
			sig := types.ValidatorSign{
				Addr:      v.GetAddress().Bytes(),
				Signature: signature,
			}
			sigs = append(sigs, sig)
		}
		tx.Signatures = sigs

		if err := tx.VerifySign(valset); err != nil {
			t.Error(err)
		}

		txs = append(txs, tx)
		fmt.Printf("%s: %s, nonce: %d \n", tx.TypeName(), tx.Hash().Hex(), tx.Nonce())
		if err := app.CheckTx(tx, false); err != nil {
			t.Fatalf("CheckTx err:%v nonce %d", err, nonce)
		}
	}

	txpool.On("VerifyTxFromCache", mock.Anything).Return(nil, false)
	txpool.On("Lock").Return()
	txpool.On("Unlock").Return()
	txpool.On("Update", mock.Anything, mock.Anything).Return(nil)
	txpool.On("Reap", mock.Anything).Return(txs)
	txpool.On("GetTxFromCache", mock.Anything).Return(nil)
	txpool.On("KeyImageReset").Return()
	txpool.On("KeyImageRemoveKeys", mock.Anything).Return(nil)

	height := uint64(1)
	block := app.CreateBlock(height, 1000, 1e18, uint64(time.Now().Unix()))
	block.LastCommit = &types.Commit{}
	app.PreRunBlock(block)
	if !app.CheckBlock(block) {
		t.Fatalf("CheckBlock not ok")
	}

	partSet := block.MakePartSet(types.DefaultConsensusParams().BlockGossip.BlockPartSizeBytes)

	_, err = app.CommitBlock(block, partSet, &types.Commit{}, false)
	if err != nil {
		t.Fatalf("CommitBlock err:%v", err)
	}

	types.RegisterBlockAmino()

	js, _ := json.Marshal(app.lastTxsResult)
	fmt.Println("lastTxsResult:", string(js))

	for _, tx := range txs {
		_, txEntry := blockStore.GetTx(tx.Hash())
		js, _ = json.Marshal(txEntry)
		fmt.Println("txEntry:", tx.Hash().Hex(), string(js))
	}

	//return
	nonce := app.checkTxState.GetNonce(accounts[1].Address)
	gasLimit := uint64(0)
	ctx := genContractCreateTx(accounts[1].Address, gasLimit, nonce, "../test/token/sol/SimpleToken.bin")
	ctx.Amount = new(big.Int).SetUint64(1)
	if err := app.CheckTx(ctx, false); err != nil {
		t.Fatalf("CheckTx err:%v", err)
	}
	fmt.Println("ctx:", ctx.String())
	txs = nil
	txs = append(txs, ctx)
	txpool = &Mempool{}
	txpool.On("VerifyTxFromCache", mock.Anything).Return(nil, false)
	txpool.On("Lock").Return()
	txpool.On("Unlock").Return()
	txpool.On("Update", mock.Anything, mock.Anything).Return(nil)
	txpool.On("Reap", mock.Anything).Return(txs)
	txpool.On("GetTxFromCache", mock.Anything).Return(ctx)
	txpool.On("KeyImageReset").Return()
	txpool.On("KeyImageRemoveKeys", mock.Anything).Return(nil)
	app.SetMempool(txpool)
	//block2
	height = uint64(2)
	block = app.CreateBlock(height, 1000, 1e18, uint64(time.Now().Unix()))
	block.LastCommit = &types.Commit{}
	app.PreRunBlock(block)
	if !app.CheckBlock(block) {
		t.Logf("CheckBlock not ok")
	}

	partSet = block.MakePartSet(types.DefaultConsensusParams().BlockGossip.BlockPartSizeBytes)

	_, err = app.CommitBlock(block, partSet, &types.Commit{}, false)
	if err != nil {
		t.Fatalf("CommitBlock err:%v", err)
	}

	for _, tx := range txs {
		_, txEntry := blockStore.GetTx(tx.Hash())
		js, _ = json.Marshal(txEntry)
		fmt.Println("txEntry:", tx.Hash().Hex(), string(js))
		receipt, _, _, _ := blockStore.GetTransactionReceipt(tx.Hash())
		rc, _ := json.Marshal(receipt)
		fmt.Println("receipt:", string(rc))
	}
	// acc0 A->U
	txn1 := genUTXOTransaction("f904ccf857cf4852c16c3232f84e808a0220cfc478d163c80000a0838d633ef6fadb8065d5daf3bec48ccdf51312d38478c756cc4a37099219f60aa0701cd4af38fb7f8b33ce295987fedc04361ff834e4cc19a832c3d28793223327f84c5842699137a517f843a0d6aec79dce7507768811b1d708b5f8c787b35a9e74fbc72f02e20a20925be12b80a074bc7f47da4b20f1df5913b694215bb178f003bd9eac3f80292dbe2b31cdc506940000000000000000000000000000000000000000a0d0d7b2fe0223c1701065f75c4dcb9a8a45696db696fb1573b107655f47346041e1a052eed310ccd26dad3a11bf5d3b2ac31838406c33533c79a56bf4aea3d07b58418902b5e3af16b188000080f84582e3e6a0bea1c230497cea56f6652983e56cc8ec4d00ed18f1f7a5834d767679e021d45aa051fe2b899d475b542b0d9a650fe56f0b172dc9b525d56d1f30d5bb7640c8b7f4f90378f8b080c0f865f863a0ddb5874be41aaeff9b2039cf0f0a17cf577917900b44b007c51e4af177aaca03a0868c639a0efa648b57164c8c386d5390e038413616098dd3130987267f9f6b07a00000000000000000000000000000000000000000000000000000000000000000f844f842a00000000000000000000000000000000000000000000000000000000000000000a0251a4e3c998f9f5f3e18b3cdfc26171ef54564510a62e12228619affa4ae017580f902c3c0f902bcf902b9a05dce977dcb37ee93141ef6fa72085f3ac668aebce77142c1d217761894a22902a056c444e16b1999790a3e4fc4f944205a424f4dc1a013e7d8860376ba585ad854a069d17db1ea96660f6982bc3b81352b702c0c238451805ea49dbbc62749fe4cc2a0aa656b758cca85bfd91feacd7531d48df26ba7bd2c88de072bdcb649291a995ea06b55bbbb6a1f105609203cdc62a34dfa153e1e3ec9df4ff31d794f1b4e423306a06d93792a6804769545144f24503d54394618f506c9eba845d725e2db8b29a409f8c6a05875cbc1c5f549706101550ece94b11e402585840bcd5aa6b56d49d6a96f09e3a001380335b379b99bc1cb3dc57bb4e8b950ed6acb5a0487e5d77d4252d428e938a0fb1c7e123ce939bce624afcdf26aaf634026f6a9800326f03f3f3947f163eb60a0f07fda39c67aac1638edfec0a991e3798161a533e7a75907edde23db3d447267a03fcbda89da0ea79993f3d36f3f2045ce0c719bb017aabfcb40a2358047817a32a038191641af30028e9e417c4f876491011a2f3d77050173fc59b7ee2645561817f8c6a00fc236fb0808e9951219b6bd74a06a8348cbba8b8801f906c23e21c14fc11cd8a0f065fa100a25b7ba074aab42646ebd870096b8d4a090d137d0d2342f9cee792fa055499f36a0f4aab063f8b0c9fd8fc204161aed26bf64396984a5ee66d5c90f54a0f23b051573d7457e24d9f98568b5387245f98b26c59f8e5395dad1100b7783f4a0dd31c1cdf1cc612f12b2a3c324b040456fecfac18be948a3b11cf0cac4f3c947a0261de4095368b9024f2ed1860f00619cddde72448119765cc2312eeb7af9d313a0933fbb29ffaa0806cd60b1ad285659c11540b7920c8fedf4d412ca5981ed9600a013e20a3b1a66ada01c7103e17a9888be8fabcba1053c4c7cb4b09fc9ab60570ca0df28721307ade1148efbac84eb653ce60bc88097702866e0214c992b0428da0bc0c0c0")

	txs = nil
	txs = append(txs, txn1)
	txpool = &Mempool{}
	txpool.On("VerifyTxFromCache", mock.Anything).Return(nil, false)
	txpool.On("Lock").Return()
	txpool.On("Unlock").Return()
	txpool.On("Update", mock.Anything, mock.Anything).Return(nil)
	txpool.On("Reap", mock.Anything).Return(txs)
	txpool.On("GetTxFromCache", mock.Anything).Return(nil)
	txpool.On("KeyImageReset").Return()
	txpool.On("KeyImageRemoveKeys", mock.Anything).Return(nil)
	app.SetMempool(txpool)

	println("\n\n\n", txs[0], "\n\n\n", txpool.Reap(1)[0])
	//block3
	height = uint64(3)
	block = app.CreateBlock(height, 1000, 1e18, uint64(time.Now().Unix()))
	block.LastCommit = &types.Commit{}
	app.PreRunBlock(block)
	if !app.CheckBlock(block) {
		t.Logf("CheckBlock not ok")
	}

	partSet = block.MakePartSet(types.DefaultConsensusParams().BlockGossip.BlockPartSizeBytes)

	_, err = app.CommitBlock(block, partSet, &types.Commit{}, false)
	if err != nil {
		t.Fatalf("CommitBlock err:%v", err)
	}

	for _, tx := range txs {
		_, txEntry := blockStore.GetTx(tx.Hash())
		js, _ = json.Marshal(txEntry)
		fmt.Println("txEntry:", tx.Hash().Hex(), string(js))
		receipt, _, _, _ := blockStore.GetTransactionReceipt(tx.Hash())
		rc, _ := json.Marshal(receipt)
		fmt.Println("receipt:", string(rc))
	}
	receiptHash := block.Receipthash()
	stateHash := block.StateHash
	balanceRecordHash := types.RlpHash(types.BlockBalanceRecordsInstance.TxRecords)
	for _, br := range types.BlockBalanceRecordsInstance.TxRecords {
		log.Debug("BRTEST", "brH", types.RlpHash(br), "br", br)
	}
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0x370d20b400224ad922fcab9ac54e75fce2212aa24026c339f390cdfc40e3e38c", "0xa1a5d1a45e2b76f9ab7311c4486f7a381c294eceb0d728ef3461d0478447330c", "0x59cbfaf329a77f6c5c496a06b2fad0b4b015e705a5ee5833917c52b9f6519777")

	// acc0 U->U
	txn2 := genUTXOTransaction("f9063feb10c698b1d37308e3c180a06e7fd62549a76c9192897f214d950171af4226830266c8127565a8878b412380f8985842699137a517f843a0d4d33f7fc119e0552985eeb797bf69e4b35ec680e2644c3a2d59de932e5783a880a09336a9587b409df96ac953abfa884c2b2b66cd320a9a200419ddb50879ce29755842699137a517f843a043846c2e173b8256e92a77ca9c51c71292ac1b094e9e5674e6990065c225d6bb80a0572a6a8976262f9266af0ae4c8a820a07e84f23f049d0fd3dadef2225925ba03940000000000000000000000000000000000000000a0b3e9f2ca93d66b9110b0d7762e9ad90d51687d3aeca958afc66bb2505e6cabb2f842a098408840892f60db9038f892f9ca5ff3229e0f25aec7274a3dcf6b7cfc5dd948a0981ecd2e3be6b44cedd7d94d926538a49327bfc114831b79fc6e89e80080ebd88902b5e3af16b188000080c3808080f904edf9015903c0f8caf863a0a4b4bcbd0daea41dc6d5d6b97802ff2633e904c0ce3711db9cda16bf3b2e0003a06c36c6acac4939af3397137e6de1e7ed0ac6578371b18c6e11c982c2e314b403a00000000000000000000000000000000000000000000000000000000000000000f863a0fdab76ffd41b721bf987ad07155bee739eceb0302500f66450df290d4d7b7209a0887b1e64f2aceb086b9f498ce3db6610c1cb7e00caf2bb16f162836b80583703a00000000000000000000000000000000000000000000000000000000000000000f888f842a00000000000000000000000000000000000000000000000000000000000000000a08feb4a9a7615f085406db54756f125cfaa4f6110ede16e992f50c61a566d78aaf842a00000000000000000000000000000000000000000000000000000000000000000a07f615cce7162d41734addad15cb8ab33c22e061154df05fccc987ebd9b7b25d380f9038ec0f902fef902fba0b553e58a6313b9141cece5510c95a07bb068367865775eebf55fc877c1d72bf2a0137dcbe237a351b6261f16a94f4c88ae402bef985b7da81eb867394145243bb6a096c723a129e7c18b717cbb8dfab388502ba37499d53947de19ac5edb6de6ba51a03872d00fe92d736aaa1aab4d4c4a68580cff32f2c66fc0c25d1d8839ef494785a049c94cae0da44b5864a9141828dde2354db11c6477ba19d0c32beeb871e9cf02a00429e978bdd3d5073d590e0da28bb5ac66cb292e6b1009d69b1a8b245620db05f8e7a0e73ebdb93f7a544c23f61704cae96e6c14edaa7b9df22423b25729eeb206d9d6a0bfb412a93d1faf648e7b43452408b56ceaa6154e1f55221a33cf26921f0ac0c0a0fb2563e954cdd676ee2076206ebd175f7dca99a9eb2dcf26839cff9dabfd4ec8a0ed93214b97e305f1378b2909fff1bc66f77cf9d611ca112a58b6b675901fcdeaa01da43dff03cd3f6540a46ecaa6d2cdc8b095eb583849fe7238d75c79dc38a622a06678c32aeb9f1865692b41aeb1aec3da6a9f38d3d05db87859bfcd6d056a645ca0486354815b1dab3a9c0290d7a26ef662247faad93281c84f6293d9996b20806ff8e7a08417f0e030be28a75e8c412b5ed520e8ad6c8935724f2c79c559643dbd7e1c04a0ff0789893a6bb72c416c6b0f8642dfa3fac0ceda80d5e74d58f3b1dcadc34271a0d98b0bb99b2278abd1069be52740b526819f5ab085b89afc8681064099df2f0da06a026a2a8f213c1652c3d3690424a2078cdf981e7d75a3e8def1fe20487e2e89a0ef255fe8dafdb5edbdc48c3f8c5bee3a262e0dbc24cba3c9df758767eaf09bbda0dff28da07f0a5858ff9239008fad150c5ed06f481ff90256ac8c93725e19abc5a0f5e0fea76927980998b018fe7a8804af57eb368c8589906a2155bd9b373aab63a02d2a5d75c2063c3e95ee8dc8cf93c52c06d7cdcb17d6695dd0d7b2e31b0d1c0ba058a2511eb785c7428f2cc82584bed12c548a763aa8f0cf0d8d60f381befc4d04a0fa9d93b5670a2ea41fd62ec8478139c63f203d4d59de2f18c073c48f8136830ce3e2c0a00000000000000000000000000000000000000000000000000000000000000000e1a0052c6be201cab1ce85d233bce04f86b0c79ddcabf85f399e8f288595ad24f653f844f842a0888056bf5a1057d6735db42d938fd16ab37da88024f3cdcc950cde69751f3b0ea0d769f4dfa2d9ad4a4540826c199d70fc8dfc499a3e982b8e82c61f08c9e61900")

	txs = nil
	txs = append(txs, txn2)
	txpool = &Mempool{}
	txpool.On("VerifyTxFromCache", mock.Anything).Return(nil, false)
	txpool.On("Lock").Return()
	txpool.On("Unlock").Return()
	txpool.On("Update", mock.Anything, mock.Anything).Return(nil)
	txpool.On("Reap", mock.Anything).Return(txs)
	txpool.On("GetTxFromCache", mock.Anything).Return(nil)
	txpool.On("KeyImageReset").Return()
	txpool.On("KeyImageRemoveKeys", mock.Anything).Return(nil)
	app.SetMempool(txpool)

	//block4
	height = uint64(4)
	block = app.CreateBlock(height, 1000, 1e18, uint64(time.Now().Unix()))
	block.LastCommit = &types.Commit{}
	app.PreRunBlock(block)
	if !app.CheckBlock(block) {
		t.Logf("CheckBlock not ok")
	}

	partSet = block.MakePartSet(types.DefaultConsensusParams().BlockGossip.BlockPartSizeBytes)

	_, err = app.CommitBlock(block, partSet, &types.Commit{}, false)
	if err != nil {
		t.Fatalf("CommitBlock err:%v", err)
	}

	for _, tx := range txs {
		_, txEntry := blockStore.GetTx(tx.Hash())
		js, _ = json.Marshal(txEntry)
		fmt.Println("txEntry:", tx.Hash().Hex(), string(js))
		receipt, _, _, _ := blockStore.GetTransactionReceipt(tx.Hash())
		rc, _ := json.Marshal(receipt)
		fmt.Println("receipt:", string(rc))
	}

	receiptHash = block.Receipthash()
	stateHash = block.StateHash
	balanceRecordHash = types.RlpHash(types.BlockBalanceRecordsInstance.TxRecords)
	for _, br := range types.BlockBalanceRecordsInstance.TxRecords {
		log.Debug("BRTEST", "brH", types.RlpHash(br), "br", br)
	}
	log.Debug("SAVER", "rh", receiptHash.Hex(), "sh", stateHash.Hex(), "brh", balanceRecordHash.Hex())
	hashChecker(t, receiptHash, stateHash, balanceRecordHash, "0x66ae609bdf4359513afb0e33644289c32dea91de418689caaf04f49d0d4332ec", "0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470", "0xa2d6b1b8c3c53f7ff250fc1783614577e886697b7cfc5377608d6f23a799a7bc")

}

func createTokenTrans(from *keystore.Key, to *common.Address, tokenAddress common.Address, nonce uint64, amount *big.Int, ret uint8, reterr string) (*types.TokenTransaction, error) {
	signedTx, err := genTokenTx(from, to, tokenAddress, nonce, amount, ret, reterr)
	if err != nil {
		panic(err)
	}
	return signedTx, err
}

func genTokenTx(from *keystore.Key, to *common.Address, tokenAddress common.Address, nonce uint64, amount *big.Int, ret uint8, err string) (*types.TokenTransaction, error) {
	var tx *types.TokenTransaction
	gasLimit := types.CalNewAmountGas(big.NewInt(0), types.EverLiankeFee)
	tx = types.NewTokenTransaction(tokenAddress, nonce, *to, amount, gasLimit, gasPrice, []byte(""))
	if err := tx.Sign(types.GlobalSTDSigner, from.PrivateKey); err != nil {
		return nil, err
	}
	return tx, nil
}

func genTx(from *keystore.Key, nonce uint64, to *common.Address, amount *big.Int, payload []byte) (*types.Transaction, error) {
	toAddr := common.EmptyAddress
	if to != nil {
		toAddr = *to
	}
	gasLimit := types.CalNewAmountGas(amount, types.EverLiankeFee)
	tx := types.NewTransaction(nonce, toAddr, amount, gasLimit, gasPrice, payload)
	if err := tx.Sign(types.GlobalSTDSigner, from.PrivateKey); err != nil {
		return nil, err
	}
	return tx, nil
}

func genContractTx(from *keystore.Key, nonce uint64, to *common.Address, amount *big.Int, payload []byte) (*types.Transaction, error) {
	tx := types.NewContractCreation(nonce, amount, gasLimit, gasPrice, payload)
	if err := tx.Sign(types.GlobalSTDSigner, from.PrivateKey); err != nil {
		return nil, err
	}
	return tx, nil
}

func genTxForCreateContract(from *keystore.Key, gas uint64, nonce uint64, contractFile string) *types.Transaction {
	bin, err := ioutil.ReadFile(contractFile)
	if err != nil {
		panic(err)
	}
	ccode := common.Hex2Bytes(string(bin))
	gasLimit = gas
	tx, _ := genContractTx(from, nonce, nil, big.NewInt(0), ccode)
	return tx
}

func genContractCreateTx(fromaddr common.Address, gasLimit uint64, nonce uint64, contractFile string) *types.ContractCreateTx {
	//gasPrice := big.NewInt(1e11)
	var ccode []byte
	if len(contractFile) < 100 {
		bin, err := ioutil.ReadFile(contractFile)
		if err != nil {
			panic(err)
		}
		ccode = common.Hex2Bytes(string(bin))
	} else {
		ccode = common.Hex2Bytes(contractFile)
	}

	ccMainInfo := &types.ContractCreateMainInfo{
		FromAddr:     fromaddr,
		AccountNonce: nonce,
		Amount:       big.NewInt(0),
		Payload:      ccode,
		//GasLimit:     gasLimit,
		//Price:        gasPrice,
	}
	tx := types.CreateContractTx(ccMainInfo, nil)
	return tx
}

func genContractCreateTx2(fromaddr common.Address, gasLimit uint64, nonce uint64, contractFile string, amount *big.Int) *types.ContractCreateTx {
	//gasPrice := big.NewInt(1e11)
	var ccode []byte
	if len(contractFile) < 100 {
		bin, err := ioutil.ReadFile(contractFile)
		if err != nil {
			panic(err)
		}
		ccode = common.Hex2Bytes(string(bin))
	} else {
		ccode = common.Hex2Bytes(contractFile)
	}

	ccMainInfo := &types.ContractCreateMainInfo{
		FromAddr:     fromaddr,
		AccountNonce: nonce,
		Amount:       amount,
		Payload:      ccode,
		//GasLimit:     gasLimit,
		//Price:        gasPrice,
	}
	tx := types.CreateContractTx(ccMainInfo, nil)
	return tx
}

func genContractUpgradeTx(fromaddr common.Address, contract common.Address, nonce uint64, contractFile string) *types.ContractUpgradeTx {
	bin, err := ioutil.ReadFile(contractFile)
	if err != nil {
		panic(err)
	}
	ccode := common.Hex2Bytes(string(bin))

	ccMainInfo := &types.ContractUpgradeMainInfo{
		FromAddr:     fromaddr,
		Recipient:    contract,
		AccountNonce: nonce,
		Payload:      ccode,
	}
	tx := types.UpgradeContractTx(ccMainInfo, nil)
	return tx
}

func genMultiSignAccountTx(nonce uint64, supportType types.SupportType) *types.MultiSignAccountTx {
	return &types.MultiSignAccountTx{
		MultiSignMainInfo: types.MultiSignMainInfo{
			AccountNonce:  nonce,
			SupportTxType: supportType,
			SignersInfo: types.SignersInfo{
				MinSignerPower: 20,
				Signers: []*types.SignerEntry{
					&types.SignerEntry{
						Power: 10,
						Addr:  acc[0].Address,
					},
					&types.SignerEntry{
						Power: 10,
						Addr:  acc[1].Address,
					},
					&types.SignerEntry{
						Power: 10,
						Addr:  acc[2].Address,
					},
				},
			},
		},
	}
}

func newTestCrossState(blockStore *blockchain.BlockStore) *txmgr.Service {
	return txmgr.NewCrossState(db.NewMemDB(), blockStore)
}

func newTestStateDB() dbm.DB {
	return dbm.NewMemDB()
}

func newTestBlockStore() *blockchain.BlockStore {
	blockStoreDB := db.NewMemDB()
	return blockchain.NewBlockStore(blockStoreDB)
}

func newTestApp(sdb dbm.DB, txpool types.Mempool, blockStore *blockchain.BlockStore, crossState *txmgr.Service) (*LinkApplication, error) {
	block := &types.Block{
		Header: &types.Header{
			Height:     0,
			Coinbase:   common.HexToAddress("0x0000000000000000000000000000000000000000"),
			Time:       uint64(1507737600),
			NumTxs:     0,
			TotalTxs:   0,
			ParentHash: common.EmptyHash,
			StateHash:  common.EmptyHash,
			GasLimit:   types.DefaultConsensusParams().BlockSize.MaxGas,
		},
		Data:       &types.Data{},
		LastCommit: &types.Commit{},
	}

	txsResult := types.TxsResult{}

	BlockPartSet := types.DefaultConsensusParams().BlockGossip.BlockPartSizeBytes
	blockParts := block.MakePartSet(BlockPartSet)

	blockStore.SaveBlock(block, blockParts, nil, nil, &txsResult)

	utxoStore := utxo.NewUtxoStore(dbm.NewMemDB(), dbm.NewMemDB(), dbm.NewMemDB())
	utxoStore.SetLogger(logger.With("module", "apptest"))

	//var linkApp *LinkApplication
	balanceRecord := blockchain.NewBalanceRecordStore(dbm.NewMemDB(), false)
	linkApp, err := NewLinkApplication(sdb, blockStore, utxoStore, crossState, nil, false, balanceRecord, nil, nil)
	linkApp.SetMempool(txpool)
	for i := 0; i < 2; i++ {
		state := linkApp.storeState
		if i == 1 {
			state = linkApp.checkTxState
		}
		for _, acc := range accounts {
			state.AddBalance(acc.Address, initBalance)
			state.AddTokenBalance(acc.Address, tokenAddr, initTokenBalance)
		}
		for _, acc := range acc {
			state.AddBalance(acc.Address, initBalance)
			state.AddTokenBalance(acc.Address, tokenAddr, initTokenBalance)
		}
	}

	return linkApp, err
}

func newTestState() *state.StateDB {
	sdb := dbm.NewMemDB()
	// state, _ := state.New(common.EmptyHash, state.NewDatabase(sdb))
	state, _ := state.New(common.EmptyHash, state.NewKeyValueDBWithCache(sdb, 128, false, 0))
	for _, acc := range accounts {
		state.AddBalance(acc.Address, initBalance)
		state.AddTokenBalance(acc.Address, tokenAddr, initTokenBalance)
		state.IntermediateRoot(false)
	}
	return state
}

func init() {
	types.RegisterUTXOTxData()
}

func genUTXOTransaction(hextx string) *types.UTXOTransaction {
	var utxoTx types.UTXOTransaction
	hexData, err := hex.DecodeString(hextx)
	if err != nil {
		fmt.Printf("hex Decode err %v\n", err)
		return nil
	}
	err = ser.DecodeBytes(hexData, &utxoTx)
	if err != nil {
		fmt.Printf("DecodeBytes err %v\n", err)
		return nil
	}
	fmt.Printf("UTXOTx\n: %s\n", utxoTx)
	return &utxoTx
}
