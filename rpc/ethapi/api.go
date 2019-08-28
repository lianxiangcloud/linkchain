// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package ethapi

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	cfg "github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/math"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/rpc/rtypes"
	"github.com/lianxiangcloud/linkchain/types"
)

const (
	defaultGasPrice = 50 * cfg.Shannon
	maxCue          = 64
)

// PublicEthereumAPI provides an API to access Ethereum related information.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicEthereumAPI struct {
	b Backend
}

// NewPublicEthereumAPI creates a new Ethereum protocol API.
func NewPublicEthereumAPI(b Backend) *PublicEthereumAPI {
	return &PublicEthereumAPI{b}
}

// GasPrice returns a suggestion for a gas price.
func (s *PublicEthereumAPI) GasPrice(ctx context.Context) (*big.Int, error) {
	return s.b.SuggestPrice(ctx)
}

// ProtocolVersion returns the current Ethereum protocol version this node supports
func (s *PublicEthereumAPI) ProtocolVersion() string {
	return s.b.ProtocolVersion()
}

/*
func (s *PublicEthereumAPI) Coinbase() common.Address {
	return s.b.Coinbase()
}
*/

// PublicAccountAPI provides an API to access accounts managed by this node.
// It offers only methods that can retrieve accounts.
type PublicAccountAPI struct {
	am *accounts.Manager
}

// NewPublicAccountAPI creates a new PublicAccountAPI.
func NewPublicAccountAPI(am *accounts.Manager) *PublicAccountAPI {
	return &PublicAccountAPI{am: am}
}

// Accounts returns the collection of accounts this node manages
func (s *PublicAccountAPI) Accounts() []common.Address {
	addresses := make([]common.Address, 0) // return [] instead of nil if empty
	for _, wallet := range s.am.Wallets() {
		for _, account := range wallet.Accounts() {
			addresses = append(addresses, account.Address)
		}
	}
	return addresses
}

// PrivateAccountAPI provides an API to access accounts managed by this node.
// It offers methods to create, (un)lock en list accounts. Some methods accept
// passwords and are therefore considered private by default.
type PrivateAccountAPI struct {
	am        *accounts.Manager
	nonceLock *AddrLocker
	b         Backend
}

// NewPrivateAccountAPI create a new PrivateAccountAPI.
func NewPrivateAccountAPI(b Backend, nonceLock *AddrLocker) *PrivateAccountAPI {
	return &PrivateAccountAPI{
		am:        b.AccountManager(),
		nonceLock: nonceLock,
		b:         b,
	}
}

// ListAccounts will return a list of addresses for accounts this node manages.
func (s *PrivateAccountAPI) ListAccounts() []common.Address {
	addresses := make([]common.Address, 0) // return [] instead of nil if empty
	for _, wallet := range s.am.Wallets() {
		for _, account := range wallet.Accounts() {
			addresses = append(addresses, account.Address)
		}
	}
	return addresses
}

// rawWallet is a JSON representation of an accounts.Wallet interface, with its
// data contents extracted into plain fields.
type rawWallet struct {
	URL      string             `json:"url"`
	Status   string             `json:"status"`
	Failure  string             `json:"failure,omitempty"`
	Accounts []accounts.Account `json:"accounts,omitempty"`
}

// ListWallets will return a list of wallets this node manages.
func (s *PrivateAccountAPI) ListWallets() []rawWallet {
	wallets := make([]rawWallet, 0) // return [] instead of nil if empty
	for _, wallet := range s.am.Wallets() {
		status, failure := wallet.Status()

		raw := rawWallet{
			URL:      wallet.URL().String(),
			Status:   status,
			Accounts: wallet.Accounts(),
		}
		if failure != nil {
			raw.Failure = failure.Error()
		}
		wallets = append(wallets, raw)
	}
	return wallets
}

// OpenWallet initiates a hardware wallet opening procedure, establishing a USB
// connection and attempting to authenticate via the provided passphrase. Note,
// the method may return an extra challenge requiring a second open (e.g. the
// Trezor PIN matrix challenge).
func (s *PrivateAccountAPI) OpenWallet(url string, passphrase *string) error {
	wallet, err := s.am.Wallet(url)
	if err != nil {
		return err
	}
	pass := ""
	if passphrase != nil {
		pass = *passphrase
	}
	return wallet.Open(pass)
}

// DeriveAccount requests a HD wallet to derive a new account, optionally pinning
// it for later reuse.
func (s *PrivateAccountAPI) DeriveAccount(url string, path string, pin *bool) (accounts.Account, error) {
	wallet, err := s.am.Wallet(url)
	if err != nil {
		return accounts.Account{}, err
	}
	derivPath, err := accounts.ParseDerivationPath(path)
	if err != nil {
		return accounts.Account{}, err
	}
	if pin == nil {
		pin = new(bool)
	}
	return wallet.Derive(derivPath, *pin)
}

// NewAccount will create a new account and returns the address for the new account.
func (s *PrivateAccountAPI) NewAccount(password string, cue string) (common.Address, error) {
	passLen := len(password)
	if passLen < 6 || passLen > 64 {
		return common.EmptyAddress, fmt.Errorf("password must be 6 to 64 in length")
	}
	if len(cue) > maxCue {
		return common.EmptyAddress, fmt.Errorf("cue is too long")
	}

	acc, err := fetchKeystore(s.am).NewAccount(password, cue)
	if err == nil {
		return acc.Address, nil
	}
	return common.EmptyAddress, err
}

// fetchKeystore retrives the encrypted keystore from the account manager.
func fetchKeystore(am *accounts.Manager) *keystore.KeyStore {
	return am.Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
}

// UnlockAccount will unlock the account associated with the given address with
// the given password for duration seconds. If duration is nil it will use a
// default of 300 seconds. It returns an indication if the account was unlocked.
func (s *PrivateAccountAPI) UnlockAccount(addr common.Address, password string, duration *uint64) (bool, error) {
	const max = uint64(time.Duration(math.MaxInt64) / time.Second)
	var d time.Duration
	if duration == nil {
		d = 300 * time.Second
	} else if *duration > max {
		return false, errors.New("unlock duration too large")
	} else {
		d = time.Duration(*duration) * time.Second
	}
	err := fetchKeystore(s.am).TimedUnlock(accounts.Account{Address: addr}, password, d, nil)
	return err == nil, err
}

// LockAccount will lock the account associated with the given address when it's unlocked.
func (s *PrivateAccountAPI) LockAccount(addr common.Address) bool {
	return fetchKeystore(s.am).Lock(addr) == nil
}

// signTransactions sets defaults and signs the given transaction
// NOTE: the caller needs to ensure that the nonceLock is held, if applicable,
// and release it after the transaction has been submitted to the tx pool
func (s *PrivateAccountAPI) signTransaction(ctx context.Context, args rtypes.SendTxArgs, passwd string) (types.Tx, error) {
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: args.From}
	wallet, err := s.am.Find(account)
	if err != nil {
		return nil, err
	}
	// Set some sanity defaults and terminate on failure
	if err := args.SetDefaults(ctx, s.b); err != nil {
		return nil, err
	}
	// Assemble the transaction and sign with the wallet
	tx := args.ToTransaction()
	return wallet.SignTxWithPassphrase(account, passwd, tx, types.SignParam)
}

// SendTransaction will create a transaction from the given arguments and
// tries to sign it with the key associated with args.To. If the given passwd isn't
// able to decrypt the key it fails.
func (s *PrivateAccountAPI) SendTransaction(ctx context.Context, args rtypes.SendTxArgs, passwd string) (common.Hash, error) {
	if args.Nonce == nil {
		// Hold the addresse's mutex around signing to prevent concurrent assignment of
		// the same nonce to multiple accounts.
		s.nonceLock.LockAddr(args.From)
		defer s.nonceLock.UnlockAddr(args.From)
	}
	signed, err := s.signTransaction(ctx, args, passwd)
	if err != nil {
		return common.EmptyHash, err
	}
	return submitTransaction(ctx, s.b, signed)
}

// SignTransaction will create a transaction from the given arguments and
// tries to sign it with the key associated with args.To. If the given passwd isn't
// able to decrypt the key it fails. The transaction is returned in SER-form, not broadcast
// to other nodes
func (s *PrivateAccountAPI) SignTransaction(ctx context.Context, args rtypes.SendTxArgs, passwd string) (*rtypes.SignTransactionResult, error) {
	// No need to obtain the noncelock mutex, since we won't be sending this
	// tx into the transaction pool, but right back to the user
	if args.Gas == nil {
		return nil, fmt.Errorf("gas not specified")
	}
	if args.GasPrice == nil {
		return nil, fmt.Errorf("gasPrice not specified")
	}
	if args.Nonce == nil {
		return nil, fmt.Errorf("nonce not specified")
	}
	signed, err := s.signTransaction(ctx, args, passwd)
	if err != nil {
		return nil, err
	}
	data, err := ser.EncodeToBytes(signed)
	if err != nil {
		return nil, err
	}
	return &rtypes.SignTransactionResult{data, signed}, nil
}

// SignAndSendTransaction was renamed to SendTransaction. This method is deprecated
// and will be removed in the future. It primary goal is to give clients time to update.
func (s *PrivateAccountAPI) SignAndSendTransaction(ctx context.Context, args rtypes.SendTxArgs, passwd string) (common.Hash, error) {
	return s.SendTransaction(ctx, args, passwd)
}

// PublicDebugAPI is the collection of Ethereum APIs exposed over the public
// debugging endpoint.
type PublicDebugAPI struct {
	b Backend
}

// NewPublicDebugAPI creates a new API definition for the public debug methods
// of the Ethereum service.
func NewPublicDebugAPI(b Backend) *PublicDebugAPI {
	return &PublicDebugAPI{b: b}
}

// GetBlockSer retrieves the SER encoded for of a single block.
func (s *PublicDebugAPI) GetBlockSer(ctx context.Context, number uint64) (string, error) {
	block, _ := s.b.BlockByNumber(ctx, rpc.BlockNumber(number))
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	encoded, err := ser.EncodeToBytes(block)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", encoded), nil
}

// PrintBlock retrieves a block and returns its pretty printed form.
func (s *PublicDebugAPI) PrintBlock(ctx context.Context, number uint64) (string, error) {
	block, _ := s.b.BlockByNumber(ctx, rpc.BlockNumber(number))
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	return spew.Sdump(block), nil
}

func (s *PublicDebugAPI) GetBlockBinary(ctx context.Context, number rpc.BlockNumber) (*rtypes.ResultBlock, error) {
	if number == rpc.PendingBlockNumber || number == rpc.LatestBlockNumber {
		return s.b.Block(nil)
	}
	height := uint64(number.Int64())
	return s.b.Block(&height)
}

func (s *PublicDebugAPI) GetBlockHeader(ctx context.Context, number rpc.BlockNumber) (*rtypes.ResultBlockHeader, error) {
	header, _ := s.b.HeaderByNumber(ctx, number)
	if header == nil {
		return nil, fmt.Errorf("block meta #%d not found", number)
	}
	return &rtypes.ResultBlockHeader{header}, nil
}

type PublicMempoolAPI struct {
	b Backend
}

func NewPublicMempoolAPI(b Backend) *PublicMempoolAPI {
	return &PublicMempoolAPI{
		b: b,
	}
}

func (s *PublicMempoolAPI) Status() map[string]hexutil.Uint {
	specPending, pending, queued := s.b.Stats()
	return map[string]hexutil.Uint{
		"specGoodTxs": hexutil.Uint(specPending),
		"goodTxs":     hexutil.Uint(pending),
		"futureTxs":   hexutil.Uint(queued),
	}
}

type PublicNetAPI struct {
	b              Backend
	networkVersion uint64
}

func NewPublicNetAPI(b Backend, netVersion uint64) *PublicNetAPI {
	return &PublicNetAPI{
		b:              b,
		networkVersion: netVersion,
	}
}

func (s *PublicNetAPI) Listening() bool {
	return true
}

func (s *PublicNetAPI) Info() (*rtypes.ResultNetInfo, error) {
	return s.b.NetInfo()
}

// Version returns the current ethereum protocol version.
func (s *PublicNetAPI) Version() string {
	return fmt.Sprintf("%d", s.networkVersion)
}
