package types

import (
	"encoding/json"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/common"
	lkctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/types"
)

type RPCResponse struct {
	ID      string          `json:"id"`
	Jsonrpc string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   RPCErr          `json:"error"`
}

type RPCErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type SendUTXOTxArgs struct {
	From     common.Address    `json:"from"`
	Nonce    *hexutil.Uint64   `json:"nonce"`
	SubAddrs []uint64          `json:"subaddrs"`
	Dests    []*types.UTXODest `json:"dests"`
	TokenID  *common.Address   `json:"token"`
}

func (s *SendUTXOTxArgs) SetDefaults() {
	if s.Nonce == nil {
		// TODO query nonce from peer
		nonce := uint64(0)
		s.Nonce = (*hexutil.Uint64)(&nonce)
	}
	// cntSubAdr := len(s.SubAddrs)
	// if cntSubAdr > 0 {
	// }
	cntDests := len(s.Dests)
	if cntDests > 0 {
		for i := 0; i < cntDests; i++ {
			if s.Dests[i].Amount == nil {
				s.Dests[i].Amount = new(hexutil.Big)
			}
		}
	}
	if s.TokenID == nil {
		defaultToken := common.EmptyAddress
		s.TokenID = &defaultToken
	}
}

type SignUTXORet struct {
	Raw  string         `json:"raw"`
	Hash common.Hash    `json:"hash"`
	Gas  hexutil.Uint64 `json:"gas"`
}
type SignUTXOTransactionResult struct {
	Txs []SignUTXORet `json:"txs"`
}

type SendTxRet struct {
	Raw     string         `json:"raw"`
	Hash    common.Hash    `json:"hash"`
	Gas     hexutil.Uint64 `json:"gas"`
	ErrCode int            `json:"err_code"`
	ErrMsg  string         `json:"err_msg"`
}

type SendUTXOTransactionResult struct {
	Txs []SendTxRet `json:"tx"`
}

type BlockHeightResult struct {
	LocalHeight  hexutil.Uint64 `json:"local_height"`
	RemoteHeight hexutil.Uint64 `json:"remote_height"`
}

type BalanceArgs struct {
	AccountIndex hexutil.Uint64  `json:"index"`
	TokenID      *common.Address `json:"token"`
}

type BalanceResult struct {
	Balance *hexutil.Big    `json:"balance"`
	Address string          `json:"address"`
	TokenID *common.Address `json:"token"`
}

type UTXOAccount struct {
	Address string         `json:"address"`
	Index   hexutil.Uint64 `json:"index"`
	Balance *hexutil.Big   `json:"balance"`
}

type EthAccount struct {
	Address common.Address `json:"address"`
	Balance *hexutil.Big   `json:"balance"`
	Nonce   hexutil.Uint64 `json:"nonce"`
}

type GetAccountInfoResult struct {
	EthAccount   EthAccount      `json:"eth_account"`
	UTXOAccounts []UTXOAccount   `json:"utxo_accounts"`
	TotalBalance *hexutil.Big    `json:"total_balance"`
	TokenID      *common.Address `json:"token"`
}

type StatusResult struct {
	RemoteHeight         hexutil.Uint64 `json:"remote_height"`
	LocalHeight          hexutil.Uint64 `json:"local_height"`
	WalletOpen           bool           `json:"wallet_open"`
	AutoRefresh          bool           `json:"auto_refresh"`
	WalletVersion        string         `json:"wallet_version"`
	ChainVersion         string         `json:"chain_version"`
	EthAddress           common.Address `json:"eth_address"`
	RefreshBlockInterval time.Duration  `json:"refresh_block_interval"`
}

type ProofKeyArgs struct {
	Hash common.Hash `json:"hash"`
	Addr string      `json:"addr"`
}

type ProofKeyRet struct {
	ProofKey string `json:"proof_key"`
}

type VerifyProofKeyArgs struct {
	Hash common.Hash `json:"hash"`
	Addr string      `json:"addr"`
	Key  string      `json:"key"`
}

type VerifyProofKey struct {
	Hash   common.Hash  `json:"hash"`
	Addr   string       `json:"addr"`
	Amount *hexutil.Big `json:"amount"`
}

type VerifyProofKeyRet struct {
	Records []*VerifyProofKey `json:"records"`
}

type CheckTxKeyArgs struct {
	TxHash   common.Hash  `json:"hash"`
	TxKey    lkctypes.Key `json:"key"`
	DestAddr string       `json:"dest"`
}

type CheckTxKeyResult struct {
	BlockID hexutil.Uint64 `json:"height"`
	Amount  *hexutil.Big   `json:"amount"`
}

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From         common.Address     `json:"from"`
	TokenAddress common.Address     `json:"tokenAddress"`
	To           *common.Address    `json:"to"`
	Gas          hexutil.Uint64     `json:"gas"`
	GasPrice     hexutil.Big        `json:"gasPrice"`
	Value        hexutil.Big        `json:"value"`
	Data         hexutil.Bytes      `json:"data"`
	Nonce        hexutil.Uint64     `json:"nonce"`
	UTXOKind     types.UTXOKind     `json:"utxokind"`
	Outputs      []types.OutputData `json:"outputs"`
}
