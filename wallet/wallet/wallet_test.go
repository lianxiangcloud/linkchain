package wallet

// var gwallet *Wallet

// func init() {
// 	config := cfg.DefaultConfig()
// 	config.Daemon.Host = "127.0.0.1"
// 	config.Daemon.Port = 18081

// 	logger := log.Root()
// 	var err error
// 	gwallet, err = NewWallet(config, logger, nil)
// 	if err != nil {
// 		panic(err.Error())
// 	}
// 	// init daemon
// 	daemon.InitDaemonClient(config.Daemon)
// }

// func TestRefreshMaxBlock(t *testing.T) {
// 	err := gwallet.RefreshMaxBlock()
// 	if err != nil {
// 		t.Fatal("RefreshMaxBlock fail,err:", err)
// 	}
// }

// func TestGetBlock(t *testing.T) {
// 	height := uint64(1)
// 	block, err := gwallet.getBlock(height)
// 	if err != nil {
// 		t.Fatal("getBlock fail,err:", err)
// 	}
// 	fmt.Printf("block:%v,datalen %d \n", height, len(block.Data.Txs))
// 	for _, v := range block.Data.Txs {
// 		switch t := v.(type) {
// 		case *types.Transaction:

// 		case *types.UTXOTransaction:
// 			fmt.Printf("utxo tx:%v", t.Fee.String())
// 		default:

// 		}
// 	}
// }
