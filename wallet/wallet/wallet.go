package wallet

import (
	"math/big"
	"sync"
	"time"

	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/libs/common"
	lkctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	tctypes "github.com/lianxiangcloud/linkchain/types"
	cfg "github.com/lianxiangcloud/linkchain/wallet/config"
	"github.com/lianxiangcloud/linkchain/wallet/types"
)

const (
	defaultUTXOGas                = 0x7a120
	defaultRefreshUTXOGasInterval = 60 * time.Second
)

func init() {
	tctypes.RegisterUTXOTxData()
	ser.RegisterInterface((*tctypes.Input)(nil), nil)
}

// Wallet user wallet
type Wallet struct {
	// cmn.BaseService

	Logger   log.Logger
	lock     sync.Mutex
	walletDB dbm.DB
	utxoGas  *big.Int

	// config
	config *cfg.Config

	addrMap     map[common.Address]*LinkAccount
	currAccount *LinkAccount // latest unlock account

	accManager *accounts.Manager
}

// NewWallet returns a new, ready to go.
func NewWallet(config *cfg.Config,
	logger log.Logger, db dbm.DB, accManager *accounts.Manager) (*Wallet, error) {

	wallet := &Wallet{
		config:     config,
		walletDB:   db,
		accManager: accManager,
		addrMap:    make(map[common.Address]*LinkAccount),
	}
	wallet.utxoGas = new(big.Int).Mul(new(big.Int).SetUint64(defaultUTXOGas), new(big.Int).SetInt64(tctypes.ParGasPrice))

	// wallet.BaseService = *cmn.NewBaseService(logger, "Wallet", wallet)
	wallet.Logger = logger

	height, err := GenesisBlockNumber()
	if err != nil {
		wallet.Logger.Error("GenesisBlockNumber fail", "err", err)
		return nil, err
	}
	defaultInitBlockHeight = uint64(*height)
	wallet.Logger.Info("NewWallet", "defaultInitBlockHeight", defaultInitBlockHeight)

	return wallet, nil
}

// OpenWallet ,open wallet with password
func (w *Wallet) OpenWallet(keystoreFile string, password string) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	la, err := NewLinkAccount(w.walletDB, w.Logger, keystoreFile, password)
	if err != nil {
		w.Logger.Error("OpenWallet NewLinkAccount fail", "err", err)
		return err
	}
	la.SetSyncQuick(w.config.Daemon.SyncQuick)
	addr := la.getEthAddress()

	w.Logger.Info("OpenWallet", "address", addr)

	laOld, ok := w.addrMap[addr]
	if ok {
		w.currAccount = laOld

		return nil
	}

	w.addrMap[addr] = la
	w.currAccount = la

	// default start account refresh
	la.OnStart()

	return nil
}

// IsWalletClosed return true if currAccount is nil
func (w *Wallet) IsWalletClosed() bool {
	return w.currAccount == nil
}

// Start starts the Wallet. It implements cmn.Service.
func (w *Wallet) Start() error {
	w.Logger.Info("starting Wallet OnStart")

	// update at first time
	w.updateUTXOGas()

	go w.refreshUTXOGas()
	return nil
}

// Stop stops the Wallet. It implements cmn.Service.
func (w *Wallet) Stop() {
	w.lock.Lock()
	defer w.lock.Unlock()

	for addr, account := range w.addrMap {
		w.Logger.Info("OnStop", "addr", account.getEthAddress())

		account.OnStop()
		delete(w.addrMap, addr)
	}

	w.Logger.Info("Stopping Wallet")
}

func (w *Wallet) updateUTXOGas() error {
	utxoGas, err := w.getUTXOGas()
	if err != nil {
		w.Logger.Error("updateUTXOGas", "err", err)
		return err
	}
	newUtxoGas := new(big.Int).Mul(new(big.Int).SetUint64(utxoGas), new(big.Int).SetInt64(tctypes.ParGasPrice))
	w.utxoGas.Set(newUtxoGas)
	w.Logger.Debug("refreshUTXOGas set utxoGas", "utxoGas", w.utxoGas.String())
	return nil
}

func (w *Wallet) refreshUTXOGas() {
	w.Logger.Debug("refreshUTXOGas")
	refresh := time.NewTicker(defaultRefreshUTXOGasInterval)
	defer refresh.Stop()

	for {
		select {
		case <-refresh.C:
			w.updateUTXOGas()
		}
	}
}

// GetBalance rpc get balance
func (w *Wallet) GetBalance(index uint64, token *common.Address, addr *common.Address) (*big.Int, error) {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.GetBalance(index, token), nil
	}

	return nil, types.ErrWalletNotOpen
}

// GetAddress rpc get address
func (w *Wallet) GetAddress(index uint64, addr *common.Address) (string, error) {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.GetAddress(index)
	}

	return "", types.ErrWalletNotOpen
}

// GetHeight rpc get height
func (w *Wallet) GetHeight(addr *common.Address) (localHeight *big.Int, remoteHeight *big.Int) {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.GetHeight()
	}

	rh, err := RefreshMaxBlock()
	if err != nil {
		w.Logger.Error("GetHeight,RefreshMaxBlock fail", "err", err)
		return big.NewInt(0), big.NewInt(0)
	}
	return big.NewInt(0), rh
}

// CreateSubAccount return new sub address and sub index
func (w *Wallet) CreateSubAccount(maxSub uint64, addr *common.Address) error {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.CreateSubAccount(maxSub)
	}

	return types.ErrWalletNotOpen
}

// AutoRefreshBlockchain set autoRefresh
func (w *Wallet) AutoRefreshBlockchain(autoRefresh bool, addr *common.Address) error {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.AutoRefreshBlockchain(autoRefresh)
	}
	return types.ErrWalletNotOpen
}

// GetAccountInfo return eth_account and utxo_accounts
func (w *Wallet) GetAccountInfo(tokenID *common.Address, addr *common.Address) (*types.GetAccountInfoResult, error) {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.GetAccountInfo(tokenID)
	}
	return nil, types.ErrWalletNotOpen
}

// RescanBlockchain ,reset wallet block and transfer info
func (w *Wallet) RescanBlockchain(addr *common.Address) error {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.RescanBlockchain()
	}

	return types.ErrWalletNotOpen
}

// GetWalletEthAddress ,return wallet eth address
func (w *Wallet) GetWalletEthAddress() (*common.Address, error) {
	if w.IsWalletClosed() {
		return nil, types.ErrWalletNotOpen
	}
	addr := w.currAccount.getEthAddress()
	return &addr, nil
}

// LockAccount lock account by addr
func (w *Wallet) LockAccount(addr common.Address) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.Logger.Info("LockAccount", "account", addr)

	account, ok := w.addrMap[addr]
	if !ok {
		return nil
	}

	// stop account refresh and reset secKey
	account.OnStop()

	delete(w.addrMap, addr)
	if addr == w.currAccount.getEthAddress() {
		w.currAccount = nil
		for _, v := range w.addrMap {
			w.currAccount = v
			w.Logger.Info("LockAccount reset currAccount", "currAccount", w.currAccount.getEthAddress())
			break
		}
	}

	return nil
}

// getGOutIndex return curr idx
func (w *Wallet) getGOutIndex(token common.Address) uint64 {
	if w.IsWalletClosed() {
		return 0
	}
	return w.currAccount.GetGOutIndex(token)
}

func (w *Wallet) defaultStatus(addr *common.Address) *types.StatusResult {
	rh, err := RefreshMaxBlock()
	if err != nil {
		w.Logger.Error("GetHeight,RefreshMaxBlock fail", "err", err)
		rh = big.NewInt(0)
	}
	chainVersion, err := GetChainVersion()
	if err != nil {
		w.Logger.Error("Status getChainVersion fail", "err", err)
		chainVersion = "0.0.0"
	}
	if addr == nil {
		addr = &common.EmptyAddress
	}
	return &types.StatusResult{
		RemoteHeight:         (*hexutil.Big)(rh),
		LocalHeight:          (*hexutil.Big)(new(big.Int).SetUint64(defaultInitBlockHeight)),
		WalletOpen:           false,
		AutoRefresh:          false,
		WalletVersion:        WalletVersion,
		ChainVersion:         chainVersion,
		EthAddress:           *addr,
		RefreshBlockInterval: 0,
		InitBlockHeight:      (hexutil.Uint64)(defaultInitBlockHeight),
	}
}

// Status return wallet status
func (w *Wallet) Status(addr *common.Address) *types.StatusResult {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.Status()
	}

	return w.defaultStatus(addr)
}

func (w *Wallet) getLKAccountByAddress(addr *common.Address) *LinkAccount {
	if w.IsWalletClosed() {
		return nil
	}
	if addr == nil || *addr == common.EmptyAddress {
		return w.currAccount
	}
	lkaccount, ok := w.addrMap[*addr]
	if ok {
		return lkaccount
	}
	return nil
}

// GetTxKey return transaction's tx secKey
func (w *Wallet) GetTxKey(hash *common.Hash, addr *common.Address) (*lkctypes.Key, error) {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.GetTxKey(hash)
	}

	return nil, types.ErrWalletNotOpen
}

// GetMaxOutput return tokenID max output idx
func (w *Wallet) GetMaxOutput(tokenID common.Address, addr *common.Address) (*hexutil.Uint64, error) {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.GetMaxOutput(tokenID)
	}

	return nil, types.ErrWalletNotOpen
}

// GetUTXOTx return UTXOTransaction
func (w *Wallet) GetUTXOTx(hash common.Hash, addr *common.Address) (*tctypes.UTXOTransaction, error) {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.GetUTXOTx(hash)
	}

	return nil, types.ErrWalletNotOpen
}

// SelectAddress return
func (w *Wallet) SelectAddress(addr common.Address) error {
	la, ok := w.addrMap[addr]
	if !ok {
		return types.ErrWalletNotOpen
	}
	w.currAccount = la

	w.Logger.Info("SelectAddress", "address", la.getEthAddress())

	return nil
}

// SetRefreshBlockInterval return
func (w *Wallet) SetRefreshBlockInterval(interval time.Duration, addr *common.Address) error {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		lkaccount.SetRefreshBlockInterval(interval)
		return nil
	}
	return types.ErrWalletNotOpen
}

// GetLocalUTXOTxsByHeight return
func (w *Wallet) GetLocalUTXOTxsByHeight(height *big.Int, addr *common.Address) (*types.UTXOBlock, error) {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.GetLocalUTXOTxsByHeight(height)
	}
	return nil, types.ErrWalletNotOpen
}

// GetLocalOutputs return
func (w *Wallet) GetLocalOutputs(ids []hexutil.Uint64, addr *common.Address) ([]types.UTXOOutputDetail, error) {
	lkaccount := w.getLKAccountByAddress(addr)
	if lkaccount != nil {
		return lkaccount.GetLocalOutputs(ids)
	}
	return nil, types.ErrWalletNotOpen
}

// GetUTXOAddInfo return UTXO Additional info
func (w *Wallet) GetUTXOAddInfo(hash common.Hash) (*types.UTXOAddInfo, error) {
	addr := w.currAccount.getEthAddress()
	lkaccount := w.getLKAccountByAddress(&addr)
	if lkaccount != nil {
		return lkaccount.GetUTXOAddInfo(hash)
	}

	return nil, types.ErrWalletNotOpen
}

// DelUTXOAddInfo del UTXO Additional info
func (w *Wallet) DelUTXOAddInfo(hash common.Hash) error {
	addr := w.currAccount.getEthAddress()
	lkaccount := w.getLKAccountByAddress(&addr)
	if lkaccount != nil {
		return lkaccount.DelUTXOAddInfo(hash)
	}

	return types.ErrWalletNotOpen
}
