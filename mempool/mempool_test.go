package mempool

import (
	"crypto/ecdsa"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	"github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	lktypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"

	//"github.com/lianxiangcloud/linkchain/libs/p2p"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGetAccounts(num int) []*keystore.Key {
	keys := make([]*keystore.Key, num)
	for i := 0; i < num; i++ {
		keys[i] = keystore.NewKeyForDirectICAP(crand.Reader)
	}
	return keys
}

func testGetLogger() log.Logger {
	handler := log.StreamHandler(os.Stdout, log.TerminalFormat(false))
	logger := log.Root()
	logger.SetHandler(handler)
	logger, _ = log.ParseLogLevel("trace", logger, "trace")
	return logger
}

func testNewMockApp(num int) *mockApp {
	accounts := testGetAccounts(num)
	mApp := &mockApp{
		nonce:    make(map[common.Address]uint64),
		balance:  make(map[common.Address]*big.Int),
		accounts: accounts,
	}

	for _, key := range mApp.accounts {
		mApp.balance[key.Address] = big.NewInt(0).Mul(big.NewInt(1e11), big.NewInt(1e11))
	}
	addr := common.HexToAddress("0x54fb1c7d0f011dd63b08f85ed7b518ab82028100")
	mApp.balance[addr] = big.NewInt(0).Mul(big.NewInt(1e11), big.NewInt(1e11))
	return mApp
}

func testGenEtx(from, to *keystore.Key, nonce uint64, amount *big.Int) (*types.Tx, error) {
	tx := types.NewTransaction(nonce, to.Address, amount, 0, new(big.Int), []byte(""))
	err := tx.Sign(types.GlobalSTDSigner, from.PrivateKey)
	if err != nil {
		return nil, err
	}
	tx1 := (types.Tx)(tx)
	return &tx1, nil
}

func genValidators() ([]types.PrivValidator, []*types.Validator) {
	privVals, validators := make([]types.PrivValidator, 0, 4), make([]*types.Validator, 0, 4)

	for i := 0; i < 4; i++ {
		privVals = append(privVals, types.GenFilePV(""))
		validators = append(validators, types.NewValidator(privVals[i].GetPubKey(), common.EmptyAddress, 1))
	}
	return privVals, validators
}

func genUTXOTx(hextx string) *types.UTXOTransaction {
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

func TestMockApp(t *testing.T) {
	app := testNewMockApp(2)
	from := app.accounts[0]
	to := app.accounts[1]

	tx, err := testGenEtx(from, to, 1, big.NewInt(10))
	if err != nil {
		panic(err)
	}
	fmt.Println(app.CheckTx(*tx, false))

	tx, err = testGenEtx(from, to, 0, big.NewInt(1000000000000000000))
	if err != nil {
		panic(err)
	}
	fmt.Println(app.CheckTx(*tx, false))

	tx, err = testGenEtx(from, to, 0, big.NewInt(10))
	if err != nil {
		panic(err)
	}
	fmt.Println(app.CheckTx(*tx, false))

	fmt.Println("from", app.GetNonce(from.Address), app.GetBalance(from.Address))
	fmt.Println("to  ", app.GetNonce(to.Address), app.GetBalance(to.Address))

	tx, err = testGenEtx(from, to, 0, big.NewInt(10))
	if err != nil {
		panic(err)
	}
	fmt.Println(app.CheckTx(*tx, false))

}

func getTestMultiSignMainInfo(nonce uint64, txType types.SupportType) *types.MultiSignMainInfo {
	mainInfo := &types.MultiSignMainInfo{
		AccountNonce:  nonce,
		SupportTxType: txType,
		SignersInfo: types.SignersInfo{
			MinSignerPower: 20,
			Signers: []*types.SignerEntry{
				&types.SignerEntry{
					Power: 10,
					Addr:  common.HexToAddress("0x1"),
				},
				&types.SignerEntry{
					Power: 10,
					Addr:  common.HexToAddress("0x2"),
				},
				&types.SignerEntry{
					Power: 10,
					Addr:  common.HexToAddress("0x3"),
				},
			},
		},
	}
	return mainInfo
}

func buildMutisignTx(t *testing.T, nonce uint64, privVals []types.PrivValidator) *types.MultiSignAccountTx {
	mainInfo := getTestMultiSignMainInfo(nonce, types.TxUpdateValidatorsType)
	signBytes, err := types.GenMultiSignBytes(*mainInfo)
	if err != nil {
		t.Fatalf("ser.EncodeToBytes err:%v", err)
	}

	sigs := make([]types.ValidatorSign, 0, len(privVals))
	for _, priv := range privVals {
		sig, err := priv.SignData(signBytes)
		if err != nil {
			t.Fatalf("priv.SignData err:%v", err)
		}
		sigs = append(sigs, types.ValidatorSign{priv.GetAddress().Bytes(), sig})
	}
	mutisignTx := types.NewMultiSignAccountTx(mainInfo, sigs)
	fmt.Printf("mutisignTx hash:%v\n", mutisignTx.Hash().Hex())
	return mutisignTx
}

func TestMempool(t *testing.T) {
	cfg := config.DefaultMempoolConfig()
	statsReportInterval = 1 * time.Second
	privVals, validators := genValidators()
	types.NewValidatorSet(validators)

	app := testNewMockApp(4)
	//p2pNodeInfo := p2p.NodeInfo{}
	mem := NewMempool(cfg, 0, nil)
	app.mempool = mem
	mem.SetLogger(testGetLogger())
	mem.app = app

	memR := NewMempoolReactor(cfg, mem)
	go memR.OnStart()

	tx, _ := testGenEtx(app.accounts[3], app.accounts[0], uint64(0), big.NewInt(9))
	memR.cacheRev.PushBack(*tx)

	//build mutisign tx
	mutisignTx := buildMutisignTx(t, 0, privVals)
	memR.mutisignCacheRev.PushBack(mutisignTx)
	//

	for i := 0; i < 30; i++ {
		from := app.accounts[rand.Intn(2)]
		to := app.accounts[3]
		nonce := uint64(rand.Intn(5))
		tx, err := testGenEtx(from, to, nonce, big.NewInt(10))
		if err != nil {
			panic(err)
		}
		mem.AddTx("", *tx)
		fmt.Printf("tx hash:%v\n", (*tx).Hash().Hex())
	}

	utxo2utxo := "f90680eb10c698b1d37308e3c105a080d41f601eab3eebe2397a2938be98c4a98e4badcf244add946b2ef5b1f844e5f8985842699137a517f843a0da9cd7a485e127d23003ef119ae963feb7339dda8bcd8109bf4566da4a99bcbc80a07f1feb0fbb72ea475ca91b5fb5698f935883b1407c547695eaf485a70d611e095842699137a517f843a05406b9bd306a86e625d9c1e6388f4e77b5e4b893be1d9cf9f741070c47492c2380a0df72b9079e0db178e71a2b4242876db3295661c6460079ad35f7acd794bb3ef7940000000000000000000000000000000000000000a029be69490a29813f9ec025771df4a5e777d3ad1990d9978c70c04093878b8031f842a0852e682a2a2440284654430dfd73f6cdf564f94c519145532a9d9e8be0df5982a06a3c76405cc0755a94df3526c836eb2949b1c04c7f88149561ea6c2720b2f18b8609184e72a00080c3808080f90531f9015903c0f8caf863a0b36d8e31f375888dbb80333ad59273dde2f27556ca396b1280edb5424839e500a09ff27c0b11e162b36fc863142fe6f56aa97230887a036795081973d8ed3e6f0ba00000000000000000000000000000000000000000000000000000000000000000f863a00d7d6cea26431e97ecce37044b35f7958a6ae271c34f7032aa79d0e19ff93e0da09cfd3572e8bcc609e416293eb9c37489813cbecb8616157a7be500f39e031300a00000000000000000000000000000000000000000000000000000000000000000f888f842a00000000000000000000000000000000000000000000000000000000000000000a0bc6b3e11d16de5441a0090bd3ce76d08780436e737992b115fe6669381acd022f842a00000000000000000000000000000000000000000000000000000000000000000a044de947b1c6ac75da05daeba3447de9dc1a8ae9afd64ee2683bc051c53d72ae780f903d2c0f90342f9033fa09f822a7c4df5010fcafbca690b78371d3807e1faea1713e7cda27f3afeb8b4dda0d957582dda65bc6200b0f45a9069513ecd5e109096a9d740d7f0fa5618f1c64aa036bc9eeed036c3c648e42bed0977881822072bc98d795124b6d9e7582a37a3a3a09ea2518bdc5d7da3f6b17de66aa7631ae20aa2811d11c3332356db603c12a0b9a0a32783e3a2bfb47bcfd41428116898b498fde146c187b7a2a1a4d32c453da206a0b33a02366a45f44f6fec27da76b06beed20a958215008412e94600023de2250bf90108a0f52ccb9823ce2a6d23d0fe2bd3ed95e83a78501c94af433f399f82d08916be44a09942d329932a9f8e1f855e47f53b681d24491fc279342de6fb8019b3ab75fa2fa020afc20997c7eff2fd5d016e744a3be6c9e91bb910a64717782df9ad8861fc28a0c8fee60cd9fe620e7d0536a3ce24d1ff90d0f0e6a5d6319511792d4e7226b81aa0ee6cfcc8a4ac97feb1f185a32e6c9eab3221cfc20ae92146cff6552d8d1b86d7a091215f8f2610221bf95b06dcd9f50a12248a05e112cbd3ed99e9a83b9474f175a07af7359cfec9598b39da96db89ad7b08636ee56e6186178faa2b9b98acc655c6a0b93ad4385ab173a6d4555908794df423759b491cd3156f23cb2689e13721fe55f90108a0ec323ad685edad9845919652d2e67e7dbfe9c573613ddd2399ac487db2252925a085b1ef92b3c479b72ec57d50a1a49b8b4887e145903c25d47996e5d5a8fc63a6a090ebeb215d6d4ee80c152111fa12db0bb616143f9f9702a7e15b57e876c6cc09a0b7a5f31f7ca81b563d40aa9d43ca3d8c868e0202a04ef5ee7869e13359424ecfa04adea5151e5728f42e6882505722aa328d610129ec28666188454164f015ce74a06896bb83198b58a7967d5498e9c035e174e6c63044f733c3eecbebc00e89f3d6a062cd5d64db7b4acdc4590c32cdd7ed42f9be089dc0fbc9f7b2207a8e0226f166a0018ed337bad368826cc08b68495a82d3a6acd3370aaa53f1238aaa4a57fce6a3a06f52f07b44df53e1582f0216e1da0bb37dd9bed7898529dadaa41bdf58d9000ba0c1f9962fa48d693ec3b176cfe68636cdcd392875089e84f980767bb62681fe03a06b046f8fb64a518215e85cfe5d8df47950817429edf15dc1c11753196909eb06e3e2c0a00000000000000000000000000000000000000000000000000000000000000000e1a06a099c2521f2d988aa331903254cc735303a05eea9e432b925122de6c9b2c290f844f842a09c49c108d6cc44f003db042598486af6a1f2e85141eddddf6066b87a61a32506a010c5b88ba969d0a1d7efc721337d7db4999079ca1d9902591dd1735c6b88fc07"
	account2Contract := "f90136f854cf4852c16c3232f84b8087be1f849cb0c800a00000000000000000000000000000000000000000000000000000000000000000a09c6407d7ab4542fde61c9fbb83224420eef23e0d72c21c0afc41e394eba72080f84ac77cca059e4aa7f8419437c9b94a0f4816ff9e209ff9fe56e2a094deefd785e8d4a5100084d0ca6234a0542b7fb14c61712c19256b0b0e14520374a81a62c8b3f31eaaac624b0d863b0c940000000000000000000000000000000000000000a081f793418fdb36afa35bec87772865d1806bc296b08699c1d48104eb6f8ac627c087be1e9bc80bb80080f84582e3e6a027040353b586a34ba17c653c346e2e551fcf5242c711c06a412f47755a358028a0251c29ed7988fd17ca8b3b6b93df9e5d80d445b740d12a80a683678f5fcd4ef0ccc580c0c0c080c5c0c0c0c0c0"
	utxo2Contract := "f905aeeb10c698b1d37308e3c180a03b0947117b33cebd8fc22e26cb203d95f0ca5c3b11d0b6843b387a7fa5290829f892c77cca059e4aa7f83d9437c9b94a0f4816ff9e209ff9fe56e2a094deefd785012a05f20080a0ab039980de18235e5fd3e89fb5999fbb7f6233e73981ef146d0472930b7068315842699137a517f843a005590df2358ba38889df848a79b07dd288246bb023356955e82ced56e67ef74980a01ccf4a4079c4ec97e51be09a9230475d326d5268887e99b965c1253ac8ba02ed940000000000000000000000000000000000000000a0cf95284e03a673acc0c99c5ef41bd519eafb19d50742b892af1b4a060b63a5f9e1a0070d50295717399610d976259cc878356412ee51b84f8a51692ac98d7aa150ee87b91c4fdb60800080f84582e3e5a0f2226baa77cb1fe182de8868ec8b7dc80269b9ba9083513b83479287a9f55c9da061dd4ba9549b5ca8dd18862be8009fcf8537eac91851fc9e6d02f382fc8688cef90443f8b003c0f865f863a0d21d19150211ef30e296fa502fda52c643c38b6cbf4cef03d09f6c7b6ca87c0fa0517e2473aa2c06e40f5ad7af5cf6864ae96b9ab74a978fdff2e8139b9c1de004a00000000000000000000000000000000000000000000000000000000000000000f844f842a00000000000000000000000000000000000000000000000000000000000000000a019574676cedbe237faed66ecaafabf3382bb9db163bbd86ce0408f8cf773f71180f9038ec0f902fef902fba067e5985fa37c9f0f361a11b2b174182f785528d51b6546b7141d0d0db271985ba0751c95495c3821e246df9ed4bfc43f5f3dd46b54a7ef98d6050b1967030a5735a05a8d9a76d6a93a697b243b2c9f66b14294dfc7afccc854297f7091ca64f2a969a01aca5c48a33a8127ae58b4491c4730653bee98df4d5ce84a9d02ab55486b4b74a037ca8238236b5998769a19170a9097f4fdf0bdc7fe135df18f3007848908c60fa0eeaf35c3e5b2fda5b93dd7eb5bec9a4ed0292938f986baaf00d37cce969e8407f8e7a0c8868fc4570c9e4866da08db338f9d6f0bac3850e1acf867144e62e7dfd5f5eda0387f00627937a6d67d2633eb2626cbcac79e3a6898a74994e706d72afc6a6ae8a063bd824073f905817d27cb9935330806feb0e4d962ef903466f540e52c5ede4ca06d2c5d83c52dfcb2db03136a01e612115ce9948c255b2677ed7e3e0d47f2ebfaa033666469052b3ceb6368a2be0661e5a05240ff1f5107a078c4565fb7d7ef2810a0ed76ae50b191d4374b28b019a034fd8ae21ff7f2553e4d053457acec68d21879a016e650415a5b7272f1408f7847150c8a69961132392340a5eddda91facf9fcb5f8e7a0ce4b67624309d62e704401e6b6cc27dfe2ce5d0b9ae8802a83aaee8bee0bb773a02e66f8141074ed6429aebffced2b22e571e96c40e0d4860a87cf6f52715b14d9a0cb4aff8ec9249014dadb077ac1988aff888789e79bf55f37da20b58c0dddd7bda0b3ca1aeb0f366248a967e853a023671d5af7bde28246d06ac5c3d3e0e13f6e04a0b5074a34f8b046e64aa19d1374904aa13a4c02dfdb5e335185e63c0a898fea85a087edc7dfaecf5fed3b8d31609de81e2c18c6fdc2ac9f6e597a72ed0642b8709ba0ec1399c9c00a45157d000c209a9a1a1f8dd386f399b3142b311b4334153c3dcda0d1e6fe08ed8d80afcce4101515d9f72e31724c53ebc95506ad62b06e8a77ef08a0e222fa5ca652d91fb75865f3e405e429145b96b335f6a594dc6d359c7afefa09a03821bda8d286828f59eff85c16999d1529ca4528d995246b2899b05847a02a0ee3e2c0a00000000000000000000000000000000000000000000000000000000000000000e1a0ef8d7abad7a9c6f97027aa570da72a8d1f4b364fa9d5600c5db8cd9bb3002da1f844f842a07b6b27b1ecd0f8c77003c87e9e90a029db188de403218de0c0a87be6e6ff1005a05d99aec15b0a07a5e0cb35c28ed79926db17a1e486bc9e4e69fafee38d97ad08"
	var err error
	account2ContractTx := genUTXOTx(account2Contract)
	err = mem.AddTx("", account2ContractTx)
	require.Nil(t, err)

	err = mem.AddTx("", account2ContractTx)
	require.NotNil(t, err)

	utxo2ContractTx := genUTXOTx(utxo2Contract)
	err = mem.AddTx("", utxo2ContractTx)
	require.Nil(t, err)

	err = mem.AddTx("", utxo2ContractTx)
	require.NotNil(t, err)

	utxo2utxoTx := genUTXOTx(utxo2utxo)
	err = mem.AddTx("", utxo2utxoTx)
	require.Nil(t, err)

	err = mem.AddTx("", utxo2utxoTx)
	require.NotNil(t, err)

	time.Sleep(time.Second * 2)
	fmt.Println("before Reap:")

	printFutureTxs(mem)

	printGoodTxs(mem)
	printSpecGoodTxs(mem)

	//txs := mem.Reap(5)

	mutisignTx = buildMutisignTx(t, 1, privVals)
	memR.mutisignCacheRev.PushBack(mutisignTx)

	mem.Stop()
}

func getUTXOTx(skey *ecdsa.PrivateKey, nonce uint64, amount *big.Int, t *testing.T) *types.UTXOTransaction {
	addr := crypto.PubkeyToAddress(skey.PublicKey)
	accountSource := &types.AccountSourceEntry{
		From:   addr,
		Nonce:  nonce,
		Amount: amount,
	}
	transferGas := types.CalNewAmountGas(amount)
	transferFee := big.NewInt(0).Mul(big.NewInt(types.ParGasPrice), big.NewInt(0).SetUint64(transferGas))
	utxoDest := &types.UTXODestEntry{
		Addr:   lktypes.AccountAddress{},
		Amount: big.NewInt(0).Sub(amount, transferFee),
	}
	dest := []types.DestEntry{utxoDest}

	utxoTx, _, err := types.NewAinTransaction(accountSource, dest, common.EmptyAddress, nil)
	require.Nil(t, err)
	err = utxoTx.Sign(types.GlobalSTDSigner, skey)
	require.Nil(t, err)

	kind := utxoTx.UTXOKind()
	assert.Equal(t, types.Ain|types.Uout, kind)
	return utxoTx
}

func TestReap(t *testing.T) {
	cfg := config.DefaultMempoolConfig()
	app := testNewMockApp(4)
	//p2pNodeInfo := p2p.NodeInfo{}
	mem := NewMempool(cfg, 0, nil)
	app.mempool = mem
	mem.SetLogger(testGetLogger())
	mem.app = app

	fromnonce := uint64(0)
	for i := 0; i < 500; i++ {
		from := app.accounts[0]
		utx := getUTXOTx(from.PrivateKey, fromnonce, big.NewInt(10000000e11), t)
		require.NotNil(t, utx)
		fromnonce++
		mem.addUTXOTx(utx)
	}

	for i := 0; i < 10; i++ {
		from := app.accounts[0]
		to := app.accounts[1]
		tx, err := testGenEtx(from, to, fromnonce, big.NewInt(10))
		if err != nil {
			panic(err)
		}
		mem.AddTx("", *tx)
		fromnonce++
	}
	fmt.Println(mem.Stats())
	reapsize := cfg.Size
	txs := mem.Reap(reapsize)
	fmt.Println(mem.Stats())
	assert.Equal(t, 500, len(txs))
	mem.Update(0, txs, nil)
	fmt.Println(mem.Stats())
}

func TestBenchAdd(t *testing.T) {
	testMempoolBench(1, 20)
}

func testMempoolBench(aNum, tNum int) {
	cfg := config.DefaultMempoolConfig()
	cfg.Lifetime = 3 * time.Hour
	statsReportInterval = 8 * time.Second
	evictionInterval = 1 * time.Minute

	app := testNewMockApp(aNum)
	//p2pNodeInfo := p2p.NodeInfo{}
	mem := NewMempool(cfg, 0, nil)
	//	mem.SetLogger(testGetLogger())
	mem.app = app

	txs := make([][]*types.Tx, aNum)
	to := app.accounts[0]
	for i := 0; i < aNum; i++ {
		txs[i] = make([]*types.Tx, tNum)

		from := app.accounts[i]
		for j := 0; j < tNum; j++ {
			tx, err := testGenEtx(from, to, uint64(j), big.NewInt(1))
			if err != nil {
				panic(err)
			}
			txs[i][j] = tx
		}
	}

	var wg sync.WaitGroup

	loopSum := aNum * tNum
	sTime := time.Now()
	for i := 0; i < aNum; i++ {
		wg.Add(1)
		go func(mtxs []*types.Tx) {
			for _, tx := range mtxs {
				mem.AddTx("", *tx)
			}
			wg.Done()
		}(txs[i])
	}
	wg.Wait()

	used := time.Now().Sub(sTime)
	nsop := float64(used.Nanoseconds()) / float64(loopSum)

	fmt.Printf("BenchTest used %s,  %.2f ns/op,  %.2f op/s\n", used, nsop, float64(time.Second.Nanoseconds())/nsop)

	specPending, pending, queue := mem.Stats()
	fmt.Println("specGood", specPending, "goodTxs:", pending, "futureTxs:", queue)

	mem.Stop()
}

func printGoodTxs(mem *Mempool) {
	fmt.Println("\n----goodTxs")
	for e := mem.goodTxs.Front(); e != nil; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		from, _ := memTx.tx.From()
		fmt.Println(from.Hex(), memTx.tx.TypeName(), memTx.tx.Hash().Hex())
	}
	fmt.Println("----goodTxs end")
}

func printSpecGoodTxs(mem *Mempool) {
	fmt.Println("\n----specgoodTxs")
	for e := mem.specGoodTxs.Front(); e != nil; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		switch tx := memTx.tx.(type) {
		case *types.MultiSignAccountTx:
			from, _ := tx.From()
			fmt.Printf("MultiSignAccountTx:%v from:%v\n", tx, from)
		default:
			panic("tx is not a MultiSignAccountTx or UpdateAddrRouteTx")
		}
	}
	fmt.Println("----specgoodTxs end")
}

func printFutureTxs(mem *Mempool) {
	fmt.Println("\n----futureTxs")
	for addr, m := range mem.futureTxs {
		fmt.Println("addr: ", addr.Hex())
		for _, nonce := range *m.txs.index {
			fmt.Println(addr.Hex(), nonce, m.txs.Get(nonce).Hash().Hex())
		}
	}
	fmt.Println("----futureTxs end")
}
