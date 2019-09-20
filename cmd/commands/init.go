package commands

import (
	"fmt"
	"math/big"
	"path/filepath"
	"time"

	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	"github.com/lianxiangcloud/linkchain/app"
	bc "github.com/lianxiangcloud/linkchain/blockchain"
	cfg "github.com/lianxiangcloud/linkchain/config"
	cs "github.com/lianxiangcloud/linkchain/consensus"
	cc "github.com/lianxiangcloud/linkchain/contract/contractcodes"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/math"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/vm/evm"
	"github.com/lianxiangcloud/linkchain/vm/wasm"
	"github.com/spf13/cobra"
	"github.com/xunleichain/tc-wasm/vm"
)

func init() {
	InitFilesCmd.Flags().String("chain_id", config.ChainID, "Blockchain id")
	InitFilesCmd.Flags().Uint64("init_height", config.InitHeight, "Blockchain initial height")
	InitFilesCmd.Flags().Bool("on_line", config.OnLine, "Set true for the online version, the default value is false")
	InitFilesCmd.Flags().String("genesis_file", config.Genesis, "genesis file for init")

	// log
	InitFilesCmd.Flags().String("log.filename", config.Log.Filename, "log file name")

	// init db flags
	InitFilesCmd.Flags().String("db_backend", config.BaseConfig.DBBackend, "db backend, support leveldb")
	InitFilesCmd.Flags().String("db_path", config.BaseConfig.DBPath, "db path for leveldb backend")
	InitFilesCmd.Flags().Uint64("db_counts", config.BaseConfig.DBCounts, "db counts")

	InitFilesCmd.Flags().Bool("full_node", config.BaseConfig.FullNode, "light-weight node or full node")
	InitFilesCmd.Flags().Bool("save_balance_record", config.BaseConfig.SaveBalanceRecord, "open transactions record storage")

	InitFilesCmd.Flags().String("init_state_root", config.BaseConfig.InitStateRoot, "init global state root")
}

// InitFilesCmd initialises a fresh BlockChain Core instance.
var InitFilesCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize BlockChain",
	RunE:  initFiles,
}

func initFiles(cmd *cobra.Command, args []string) error {
	// delete dir of data
	//os.RemoveAll(config.DBDir())
	types.UpdateBlockHeightZero(config.InitHeight)

	if !config.OnLine {
		err := initFilesOfKeyStore(config)
		if err != nil {
			return err
		}
	}

	err := initFilesWithConfig(config)
	if err != nil {
		return err
	}

	if isGenesisBlockExist(config) {
		return nil
	}

	genDoc, err := types.GenesisDocFromFile(config.GenesisFile())
	if err != nil {
		return err
	}

	// Make first block
	if !config.OnLine {
		genDoc.AllocAccounts = types.GetTestAllocAccounts()
	} else {
		// TODO needCheck
		genDoc.AllocAccounts = types.GetAllocAccounts()
	}

	vals, err := createGenesisBlock(config, genDoc)
	if err != nil {
		return err
	}

	if vals != nil && len(vals) != 0 {
		var validators []types.GenesisValidator
		for _, val := range vals {
			fmt.Println(val)
			validators = append(validators, types.GenesisValidator{
				PubKey:   val.PubKey,
				Power:    val.VotingPower,
				CoinBase: val.CoinBase,
				Name:     "",
			})
		}
		genDoc.Validators = validators
	}

	// Make consensus init status
	return createConsensusStatus(config, genDoc)
}

func initFilesWithConfig(config *cfg.Config) error {
	// private validator
	privValFile := config.PrivValidatorFile()
	var pv *types.FilePV
	if common.FileExists(privValFile) {
		pv = types.LoadFilePV(privValFile)
		logger.Info("Found private validator", "path", privValFile)
	} else {
		pv = types.GenFilePV(privValFile)
		pv.Save()
		logger.Info("Generated private validator", "path", privValFile)
	}

	// genesis file
	genFile := config.GenesisFile()
	if common.FileExists(genFile) {
		logger.Info("Found genesis file", "path", genFile)
	} else {
		genDoc := types.GenesisDoc{
			ChainID:         common.Fmt("test-chain-%v", common.RandStr(6)),
			GenesisTime:     time.Now().Local().String(),
			ConsensusParams: types.DefaultConsensusParams(),
		}
		genDoc.Validators = []types.GenesisValidator{{
			PubKey: pv.GetPubKey(),
			Power:  10,
		}}

		if len(config.ChainID) > 0 {
			genDoc.ChainID = config.ChainID
		}

		if err := genDoc.SaveAs(genFile); err != nil {
			return err
		}
		logger.Info("Generated genesis file", "path", genFile)
	}

	return nil
}

func initFilesOfKeyStore(config *cfg.Config) error {
	keystoreDir := config.KeyStoreDir()
	if err := common.EnsureDir(keystoreDir, 0700); err != nil {
		common.PanicSanity(err.Error())
	}

	for filename, content := range keystoreFilesMap {
		storeFileName := filepath.Join(keystoreDir, filename)
		if !common.FileExists(storeFileName) {
			common.WriteFile(storeFileName, []byte(content), 0644)
		}
		logger.Info("Generated keystore file", "path", storeFileName)
	}

	dumps := make([]bool, len(types.TestAccounts))
	for i := 0; i < len(types.TestAccounts); i++ {
		matches, _ := filepath.Glob(keystoreDir + "/UTC--*Z--" + types.TestAccounts[i].Addr[2:])
		if len(matches) == 0 {
			dumps[i] = true
		} else {
			dumps[i] = false
		}
	}
	for i := 0; i < len(types.TestAccounts); i++ {
		if dumps[i] {
			addr := common.HexToAddress(types.TestAccounts[i].Addr)
			keystore.DumpKey(keystoreDir, addr, []byte(types.TestAccounts[i].Key))
		}
	}
	return nil
}

var keystoreFilesMap = map[string]string{
	"UTC--2018-04-15T05-21-48.033606105Z--54fb1c7d0f011dd63b08f85ed7b518ab82028100": `
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
`,
}

func createGenesisBlock(config *cfg.Config, genDoc *types.GenesisDoc) ([]*types.Validator, error) {
	blockStoreDB := dbm.NewDB("blockstore", dbm.DBBackendType(config.DBBackend), config.DBDir(), config.DBCounts)
	defer blockStoreDB.Close()

	txDB := dbm.NewDB("txmgr", dbm.DBBackendType(config.DBBackend), config.DBDir(), config.DBCounts)
	defer txDB.Close()

	stateDB := dbm.NewDB("state", dbm.DBBackendType(config.DBBackend), config.DBDir(), config.DBCounts)
	defer stateDB.Close()

	balanceRecordDB := dbm.NewDB("balance_record", dbm.DBBackendType(config.DBBackend), config.DBDir(), config.DBCounts)
	defer balanceRecordDB.Close()

	balanceRecordStore := bc.NewBalanceRecordStore(balanceRecordDB, config.SaveBalanceRecord)

	isTrie := config.FullNode
	stateRoot := common.EmptyHash
	if len(config.InitStateRoot) != 0 {
		stateRoot = common.HexToHash(config.InitStateRoot)
	}
	storeState, err := state.New(stateRoot, state.NewKeyValueDBWithCache(stateDB, 0, isTrie, 0))
	if err != nil {
		return nil, err
	}

	blockStore := bc.NewBlockStore(blockStoreDB)
	blockStore.SaveInitHeight(types.BlockHeightZero)
	defaultParams := genDoc.ConsensusParams

	for straddr, account := range genDoc.AllocAccounts {
		addr := common.HexToAddress(straddr)
		storeState.AddBalance(addr, account.Balance)
		storeState.SetNonce(addr, account.Nonce)
	}

	vals, err := deployOriginalContract(storeState)
	if err != nil {
		return nil, err
	}

	header := &types.Header{
		ChainID:    config.ChainID,
		Height:     types.BlockHeightZero,
		Coinbase:   common.HexToAddress("0x0000000000000000000000000000000000000000"),
		Time:       uint64(1507737600),
		NumTxs:     0,
		TotalTxs:   0,
		ParentHash: common.EmptyHash,
		StateHash:  common.EmptyHash,
		GasLimit:   defaultParams.BlockSize.MaxGas,
	}
	types.SaveBalanceRecord = config.SaveBalanceRecord
	if len(contractData) > 0 && config.OnLine {
		contextWasm := wasm.NewWASMContext(types.CopyHeader(header), blockStore, nil, config.WasmGasRate)
		wasm := wasm.NewWASM(contextWasm, storeState, evm.Config{EnablePreimageRecording: false})
		for _, cData := range contractData {
			sender, contractAddr := common.HexToAddress(cData.sender), common.HexToAddress(cData.contractAddr)
			amount, ok := big.NewInt(0).SetString(cData.amount, 0)
			if !ok {
				return nil, fmt.Errorf("convert big Int fail %s", cData.amount)
			}
			if _, err := app.CallWasmContract(wasm, sender, contractAddr, amount, []byte(cData.input), logger); err != nil {
				return nil, err
			}
		}
		if ok := checkDeposit(storeState); !ok {
			return nil, fmt.Errorf("checkDeposit fail")
		}
		if ok := checkPledgeAccount(storeState); !ok {
			return nil, fmt.Errorf("checkPledgeAccount fail")
		}
		fmt.Println("contract data init!")
	} else {
		fmt.Println("contract data is nil when init!")
	}

	stateHash := storeState.IntermediateRoot(false)

	trieRoot, err := storeState.Commit(false, header.Height)
	if err != nil {
		return nil, err
	}

	storeState.Database().TrieDB().Commit(trieRoot, false)
	txsResult := types.TxsResult{TrieRoot: trieRoot, StateHash: stateHash}

	header.StateHash = stateHash
	if time.Now().Unix() >= 1569409200 {
		header.Time = uint64(1569409200)
	}

	block := &types.Block{
		Header:     header,
		Data:       &types.Data{},
		LastCommit: &types.Commit{},
	}

	fmt.Println("genesisBlock stateHash", stateHash.Hex())
	fmt.Println("genesisBlock trieRoot", trieRoot.Hex())
	fmt.Printf("genesisBlock ChainID:%v Height:%d block.Hash:%v\n", block.ChainID, block.Height, block.Hash().String())
	blockStore.SaveBlock(block, block.MakePartSet(defaultParams.BlockGossip.BlockPartSizeBytes), nil, nil, &txsResult)

	types.BlockBalanceRecordsInstance.SetBlockTime(block.Time())
	types.BlockBalanceRecordsInstance.SetBlockHash(block.Hash())
	balanceRecordStore.Save(block.Height, types.BlockBalanceRecordsInstance)
	types.BlockBalanceRecordsInstance.Reset()

	return vals, nil
}

func createConsensusStatus(config *cfg.Config, genDoc *types.GenesisDoc) error {
	if genDoc == nil {
		return fmt.Errorf("Error create consensus state: genDoc is nil")
	}

	statusDB := dbm.NewDB("consensus_state", dbm.DBBackendType(config.DBBackend), config.DBDir(), config.DBCounts)
	defer statusDB.Close()
	_, err := cs.CreateStatusFromGenesisDoc(statusDB, genDoc)
	return err
}

func isGenesisBlockExist(config *cfg.Config) bool {
	blockStoreDB := dbm.NewDB("blockstore", dbm.DBBackendType(config.DBBackend), config.DBDir(), config.DBCounts)
	defer blockStoreDB.Close()

	blockStore := bc.NewBlockStore(blockStoreDB)
	genesisBlock := blockStore.GetHeader(0)
	if genesisBlock != nil {
		fmt.Println("genesisBlock is exist")
		return true
	}
	return false
}

func deployOriginalContract(st *state.StateDB) ([]*types.Validator, error) {
	if len(cc.CandidatesCodes) == 0 {
		fmt.Println("candidates contract code nil!!!")
	} else {
		if err := initWasmContract(st, cfg.ContractCandidatesAddr, cc.CandidatesCodes, logger); err != nil {
			return nil, fmt.Errorf("deploy candidates Contract error:%v", err)
		}
	}

	if len(cc.CoefficientCodes) == 0 {
		fmt.Println("coefficient contract code nil!!!")
	} else if err := initWasmContract(st, cfg.ContractCoefficientAddr, cc.CoefficientCodes, logger); err != nil {
		return nil, fmt.Errorf("deploy coefficient Contract error:%v", err)
	}

	if len(cc.CommitteeCodes) == 0 {
		fmt.Println("committee contract code nil!!!")
	} else if err := initWasmContract(st, cfg.ContractCommitteeAddr, cc.CommitteeCodes, logger); err != nil {
		return nil, fmt.Errorf("deploy committee Contract error:%v", err)
	}

	if len(cc.FoundationCodes) == 0 {
		fmt.Println("foundation contract code nil!!!")
	} else if err := initWasmContract(st, cfg.ContractFoundationAddr, cc.FoundationCodes, logger); err != nil {
		return nil, fmt.Errorf("deploy foundation Contract error:%v", err)
	}

	if len(cc.PledgeCodes) == 0 {
		fmt.Println("pledge contract code nil!!!")
	} else if err := initWasmContract(st, cfg.ContractPledgeAddr, cc.PledgeCodes, logger); err != nil {
		return nil, fmt.Errorf("deploy pledge Contract error:%v", err)
	}

	if len(cc.ConsCommitteeCodes) == 0 {
		fmt.Println("ContractConsCommittee contract code nil!!!")
	} else if err := initWasmContract(st, cfg.ContractConsCommitteeAddr, cc.ConsCommitteeCodes, logger); err != nil {
		return nil, fmt.Errorf("deploy ContractConsCommittee Contract error:%v", err)
	}

	if len(cc.BlacklistCode) == 0 {
		fmt.Println("blacklist contract code nil!!!")
	} else {
		if err := initWasmContract(st, cfg.ContractBlacklistAddr, cc.BlacklistCode, logger); err != nil {
			return nil, fmt.Errorf("deploy blacklist Contract error:%v", err)
		}
	}

	var validatorsCode string
	if config.OnLine {
		if len(cc.ValidatorsCodesOnline) == 0 {
			return nil, fmt.Errorf("Error:validators white list contract code nil in online mode")
		}
		validatorsCode = cc.ValidatorsCodesOnline
	} else {
		if len(cc.ValidatorsCodes) == 0 {
			fmt.Println("validators white list contract code nil!!!")
		}
		validatorsCode = cc.ValidatorsCodes
	}

	if len(validatorsCode) != 0 {
		if err := initWasmContract(st, cfg.ContractValidatorsAddr, validatorsCode, logger); err != nil {
			return nil, fmt.Errorf("deploy validators Contract error:%v", err)
		}
		return st.GetWhiteValidators(logger), nil
	}

	return nil, nil
}

//initWasmContract deploy wasm contract when init
func initWasmContract(st *state.StateDB, contractAddr common.Address, codeStr string, logger log.Logger) error {
	input := []byte("init|{}")

	code := common.Hex2Bytes(codeStr)

	caller := common.EmptyAddress
	to := contractAddr
	value := big.NewInt(0)
	gas := uint64(1000000000000000000)

	st.CreateAccount(contractAddr)
	st.SetNonce(contractAddr, 1)
	st.SetCode(contractAddr, code)

	innerContract := vm.NewContract(caller.Bytes(), to.Bytes(), value, gas)
	innerContract.SetCallCode(contractAddr.Bytes(), crypto.Keccak256Hash(code).Bytes(), code)
	innerContract.Input = input
	innerContract.CreateCall = true
	eng := vm.NewEngine(innerContract, innerContract.Gas, st, logger)
	eng.SetTrace(false) // trace app execution.

	app, err := eng.NewApp(innerContract.Address().String(), innerContract.Code, false)
	if err != nil {
		return fmt.Errorf("exec.NewApp fail: %s", err)
	}

	app.EntryFunc = vm.APPEntry
	ret, err := eng.Run(app, innerContract.Input)
	if err != nil {
		return fmt.Errorf("eng.Run fail: err=%s", err)
	}

	vmem := app.VM.VMemory()
	_, err = vmem.GetString(ret)
	if err != nil {
		return fmt.Errorf("vmem.GetString fail: err=%v", err)
	}
	return nil
}

func checkDeposit(st *state.StateDB) bool {
	addrs := make([]common.Address, len(rateInfo))
	for k := range rateInfo {
		addrs = append(addrs, common.HexToAddress(k))
	}

	deposits := st.GetCandidatesDeposit(addrs, logger)

	for i, v := range addrs {
		deposit := big.NewInt(rateInfo[v.String()])
		if deposits[i].Cmp(deposit.Mul(deposit, big.NewInt(cfg.Ether))) != 0 {
			fmt.Println("deposit compare fail!", "should ether", deposit, "but", deposits[i])
			return false
		}
	}

	return true
}

func checkPledgeAccount(st *state.StateDB) bool {
	oldPledgeAddress := common.HexToAddress("0xe474d6001848146aa6155bbfd7b269f2f91fe075")
	b := st.GetBalance(oldPledgeAddress)
	if b.Sign() == 0 {
		fmt.Println("Warn:Old EVM pledge account balance is 0! May not transfer stateDB")
		types.BlockBalanceRecordsInstance.Reset()
		return true
	}
	deposit := big.NewInt(0).Mul(big.NewInt(subDeposit), big.NewInt(cfg.Ether))
	if b.Cmp(deposit) < 0 {
		fmt.Println("Error:Old EVM pledge account balance not enough!", "pledge balacne", b, "should be", deposit)
		types.BlockBalanceRecordsInstance.Reset()
		return false
	}
	if b.Cmp(deposit) > 0 {
		fmt.Println("Warn:Old EVM pledge account balance is more! May not finish withDraw!", "pledge balacne", b, "should be", deposit)
	}

	st.SubBalance(oldPledgeAddress, deposit)
	st.AddBalance(cfg.ContractPledgeAddr, deposit)

	// save balance record
	payloads := make([]types.Payload, 0)
	tbr := types.NewTxBalanceRecords()
	tbr.SetOptions(common.EmptyHash, types.TxNormal, payloads, 0, uint64(math.MaxUint64),
		big.NewInt(types.GasPrice), oldPledgeAddress, cfg.ContractPledgeAddr, common.EmptyAddress)
	txBr := types.GenBalanceRecord(oldPledgeAddress, cfg.ContractPledgeAddr, types.AccountAddress, types.AccountAddress, types.TxTransfer, common.EmptyAddress, deposit)
	tbr.AddBalanceRecord(txBr)
	br := types.GenBalanceRecord(oldPledgeAddress, cfg.ContractFoundationAddr, types.AccountAddress, types.AccountAddress, types.TxFee, common.EmptyAddress, big.NewInt(0))
	tbr.AddBalanceRecord(br)
	types.BlockBalanceRecordsInstance.AddTxBalanceRecord(tbr)

	return true
}
