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
	ID      uint64          `json:"id"`
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
	LocalHeight  *hexutil.Big `json:"local_height"`
	RemoteHeight *hexutil.Big `json:"remote_height"`
}

type BalanceArgs struct {
	AccountIndex hexutil.Uint64  `json:"index"`
	TokenID      *common.Address `json:"token"`
	Addr         *common.Address `json:"addr"`
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

func (ua *UTXOAccount) Equal(t *UTXOAccount) bool {
	if ua == nil {
		if t == nil {
			return true
		}
		return false
	}
	if t == nil {
		return false
	}
	if ua.Address == t.Address &&
		ua.Index.String() == t.Index.String() &&
		ua.Balance.String() == t.Balance.String() {
		return true
	}
	return false
}

type EthAccount struct {
	Address common.Address `json:"address"`
	Balance *hexutil.Big   `json:"balance"`
	Nonce   hexutil.Uint64 `json:"nonce"`
}

func (ea *EthAccount) Equal(t *EthAccount) bool {
	if ea == nil {
		if t == nil {
			return true
		}
		return false
	}
	if t == nil {
		return false
	}
	if ea.Address.String() == t.Address.String() &&
		ea.Balance.String() == t.Balance.String() &&
		ea.Nonce.String() == t.Nonce.String() {
		return true
	}
	return false
}

type GetAccountInfoResult struct {
	EthAccount   EthAccount      `json:"eth_account"`
	UTXOAccounts []UTXOAccount   `json:"utxo_accounts"`
	TotalBalance *hexutil.Big    `json:"total_balance"`
	TokenID      *common.Address `json:"token"`
}

func (g *GetAccountInfoResult) Equal(t *GetAccountInfoResult) bool {
	if g == nil {
		if t == nil {
			return true
		}
		return false
	}
	if t == nil {
		return false
	}
	if !g.EthAccount.Equal(&t.EthAccount) ||
		g.TotalBalance.String() != t.TotalBalance.String() ||
		g.TokenID.String() != t.TokenID.String() ||
		len(g.UTXOAccounts) != len(t.UTXOAccounts) {
		return false
	}
	for i := 0; i < len(g.UTXOAccounts); i++ {
		if !g.UTXOAccounts[i].Equal(&t.UTXOAccounts[i]) {
			return false
		}
	}
	return true
}

type StatusResult struct {
	RemoteHeight         *hexutil.Big   `json:"remote_height"`
	LocalHeight          *hexutil.Big   `json:"local_height"`
	WalletOpen           bool           `json:"wallet_open"`
	AutoRefresh          bool           `json:"auto_refresh"`
	WalletVersion        string         `json:"wallet_version"`
	ChainVersion         string         `json:"chain_version"`
	EthAddress           common.Address `json:"eth_address"`
	RefreshBlockInterval time.Duration  `json:"refresh_block_interval"`
}

type ProofKeyArgs struct {
	Hash    common.Hash     `json:"hash"`
	Addr    string          `json:"addr"`
	EthAddr *common.Address `json:"eth_addr"`
}

type ProofKeyRet struct {
	ProofKey string `json:"proof_key"`
}

type VerifyProofKeyArgs struct {
	Hash    common.Hash     `json:"hash"`
	Addr    string          `json:"addr"`
	Key     string          `json:"key"`
	EthAddr *common.Address `json:"eth_addr"`
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

type UTXOBlock struct {
	Height *hexutil.Big      `json:"height"`
	Time   *hexutil.Big      `json:"timestamp"`
	Txs    []UTXOTransaction `json:"txs"`
}

type RPCInput interface {
}

//Output represents a utxo or account output
type RPCOutput interface {
}
type UTXOTransaction struct {
	Inputs  []RPCInput     `json:"inputs"`
	Outputs []RPCOutput    `json:"outputs"`
	TokenID common.Address `json:"token_id"` //current version one Tx support only one token
	Fee     *hexutil.Big   `json:"fee"`      //fee charge only LKC, without unit (different from gas)
	Hash    common.Hash    `json:"hash"`
	TxFlag  uint8          `json:"tx_flag"` // 1 - income，2- output，3-in and out
}

type AccountInput struct {
	From   common.Address `json:"from"`
	Nonce  hexutil.Uint64 `json:"nonce"`
	Amount *hexutil.Big   `json:"amount"` //Amount  =  user set amount + Fee, b in c = aG + bH
}

type UTXOInput struct {
	GlobalIndex hexutil.Uint64 `json:"global_index"`
}

type AccountOutput struct {
	To     common.Address `json:"to"`
	Amount *hexutil.Big   `json:"amount"`
	Data   hexutil.Bytes  `json:"data"` //contract data
}

//UTXOOutput represents a utxo output
type UTXOOutput struct {
	OTAddr      common.Hash    `json:"otaddr"`
	GlobalIndex hexutil.Uint64 `json:"global_index"`
}

type UTXOOutputDetail struct {
	GlobalIndex  hexutil.Uint64 `json:"global_index"`
	Amount       *hexutil.Big   `json:"amount"`
	SubAddrIndex hexutil.Uint64 `json:"sub_addr_index"`
	TokenID      common.Address `json:"token_id"`
	Remark       hexutil.Bytes  `json:"remark"`
}

type LocalOutputsArgs struct {
	IDs  []hexutil.Uint64 `json:"ids"`
	Addr *common.Address  `json:"addr"`
}
