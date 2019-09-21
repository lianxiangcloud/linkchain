package node

import (
	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	cfg "github.com/lianxiangcloud/linkchain/wallet/config"
	"github.com/lianxiangcloud/linkchain/wallet/daemon"
	"github.com/lianxiangcloud/linkchain/wallet/rpc"
	"github.com/lianxiangcloud/linkchain/wallet/wallet"
)

// DBContext specifies config information for loading a new DB.
type DBContext struct {
	ID     string
	Config *cfg.Config
}

// DBProvider takes a DBContext and returns an instantiated DB.
type DBProvider func(*DBContext) (dbm.DB, error)

// DefaultDBProvider returns a database using the DBBackend and DBDir
// specified in the ctx.Config.
func DefaultDBProvider(ctx *DBContext) (dbm.DB, error) {
	dbType := dbm.DBBackendType(ctx.Config.DBBackend)
	return dbm.NewDB(ctx.ID, dbType, ctx.Config.DBDir(), 1), nil
}

// NodeProvider takes a config and a logger and returns a ready to go Node.
type NodeProvider func(*cfg.Config, log.Logger) (*Node, error)

// DefaultNewNode returns a blockchain node with default settings for the
// PrivValidator, and DBProvider.
// It implements NodeProvider.
func DefaultNewNode(config *cfg.Config, logger log.Logger) (*Node, error) {
	return NewNode(config, logger, DefaultDBProvider)
}

func makeAccountManager(config *cfg.Config) (*accounts.Manager, error) {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	keystoreDir := config.KeyStoreDir()

	// Assemble the account manager and supported backends
	backends := []accounts.Backend{
		keystore.NewKeyStore(keystoreDir, scryptN, scryptP),
	}

	return accounts.NewManager(backends...), nil
}

func NewNode(config *cfg.Config, logger log.Logger, dbProvider DBProvider) (*Node, error) {
	// logger.With("module", "node")
	// logger.Info("DefaultNewNode", "conf", *config)
	// init daemon
	daemon.InitClient(config.Daemon, wallet.WalletVersion)

	// init db
	walletDB, err := dbProvider(&DBContext{"wallet", config})
	if err != nil {
		return nil, err
	}

	// Ensure that the AccountManager method works before the node has started.
	accountManager, err := makeAccountManager(config)
	if err != nil {
		return nil, err
	}

	// init wallet
	localWallet, err := wallet.NewWallet(config, logger.With("module", "wallet"), walletDB, accountManager)
	if err != nil {
		return nil, err
	}

	// init rpc

	// Make rpc context and service
	rpcContext := rpc.NewContext()
	rpcContext.SetLogger(logger)
	rpcContext.SetWallet(localWallet)
	rpcContext.SetAccountManager(accountManager)

	rpcSrv, err := rpc.NewService(config, rpcContext)
	if err != nil {
		return nil, err
	}

	node := &Node{
		config:         config,
		accountManager: accountManager,
		localWallet:    localWallet,
		rpcSrv:         rpcSrv,
	}

	node.BaseService = *cmn.NewBaseService(logger, "Node", node)
	return node, nil
}

type Node struct {
	cmn.BaseService

	// config
	config *cfg.Config
	// wallet
	localWallet *wallet.Wallet

	// accounts manager
	accountManager *accounts.Manager

	// rpc
	rpcSrv *rpc.Service
}

// OnStart starts the Node. It implements cmn.Service.
func (n *Node) OnStart() error {
	n.Logger.Info("starting Node")
	n.rpcSrv.Start()
	n.localWallet.Start()
	return nil
}

// OnStop stops the Node. It implements cmn.Service.
func (n *Node) OnStop() {
	n.BaseService.OnStop()
	n.Logger.Info("Stopping Node")
	n.localWallet.Stop()
	n.rpcSrv.Stop()
}

// RunForever waits for an interrupt signal and stops the node.
func (n *Node) RunForever() {
	// Sleep forever and then...
	cmn.TrapSignal(func() {
		n.Stop()
	})
}
