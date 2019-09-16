package types

import (
	"sync"
	"math/big"
	"encoding/json"

	"github.com/lianxiangcloud/linkchain/libs/common"
)

const (
	AccountAddress uint32 = 0
	PrivateAddress uint32 = 1
	NoAddress      uint32 = 2
)

var (
	SaveBalanceRecord bool = false
	BlockBalanceRecordsInstance *BlockBalanceRecords
)

type Payload []byte

type BlockBalanceRecords struct {
	TxRecords []*TxBalanceRecords `json:"tx_records"`
	BlockHash common.Hash         `json:"block_hash"`
	BlockTime uint64              `json:"block_time"`
	mu        sync.Mutex
}

type TxBalanceRecords struct {
	Hash     common.Hash     `json:"hash"`
	Type     string          `json:"type"`
	Records  []BalanceRecord `json:"records"`
	Payloads []Payload       `json:"payloads"`
	Nonce    uint64          `json:"nonce"`
	GasLimit uint64          `json:"gas_limit"`
	GasPrice *big.Int        `json:"gas_price"`
	From     common.Address  `json:"from"`
	To       common.Address  `json:"to"`
	TokenId  common.Address  `json:"token_id"`
}

type BalanceRecord struct {
	From            common.Address `json:from`
	To              common.Address `json:to`
	FromAddressType uint32         `json:"from_address_type"`
	ToAddressType   uint32         `json:"to_address_type"`
	Type            string         `json:"type"`
	TokenID         common.Address `json:"token_id"`
	Amount          *big.Int       `json:"amount"`
}

func init() {
	BlockBalanceRecordsInstance = &BlockBalanceRecords{
		TxRecords: make([]*TxBalanceRecords, 0),
	}
}

func NewTxBalanceRecords() *TxBalanceRecords {
	return &TxBalanceRecords{}
}

func GenBalanceRecord(from common.Address, to common.Address, fromAddressType uint32, toAddressType uint32, typeStr string, tokenId common.Address, amount *big.Int) BalanceRecord {
	if !SaveBalanceRecord {
		return BalanceRecord{}
	}
	finnalAmount := big.NewInt(0).Add(big.NewInt(0), amount)

	return BalanceRecord{
		From:            from,
		To:              to,
		FromAddressType: fromAddressType,
		ToAddressType:   toAddressType,
		Type:            typeStr,
		TokenID:         tokenId,
		Amount:          finnalAmount,
	}
}

func NewBlockBalanceRecords() *BlockBalanceRecords {
	return &BlockBalanceRecords{
		TxRecords: make([]*TxBalanceRecords, 0),
	}
}

func (b *BlockBalanceRecords) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.BlockHash = common.EmptyHash
	b.BlockTime = 0
	b.TxRecords = make([]*TxBalanceRecords, 0)
}

func (b *BlockBalanceRecords) Json() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	jb, err := json.Marshal(b)
	if err != nil {
		panic(err)
	}
	return jb
}

func (b *BlockBalanceRecords) AddTxBalanceRecord(t *TxBalanceRecords)  {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.TxRecords = append(b.TxRecords, t)
}

func (b *BlockBalanceRecords) SetBlockHash(blockHash common.Hash) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.BlockHash = blockHash
}

func (b *BlockBalanceRecords) SetBlockTime(blockTime uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.BlockTime = blockTime
}

func (t *TxBalanceRecords) SetOptions(hash common.Hash, typenane string, payloads []Payload, nonce uint64,
	gasLimit uint64, gasPrice *big.Int, from common.Address, to common.Address, tokenId common.Address) {
	t.Hash     = hash
	t.Type     = typenane
	t.Payloads = payloads
	t.Nonce    = nonce
	t.GasLimit = gasLimit
	t.GasPrice = gasPrice
	t.From     = from
	t.To       = to
	t.TokenId  = tokenId
}

func (t *TxBalanceRecords) AddBalanceRecord(br BalanceRecord) {
	t.Records = append(t.Records, br)
}

func (t *TxBalanceRecords) IsBalanceRecordEmpty() bool {
	return len(t.Records) == 0
}

func (t *TxBalanceRecords) ClearBalanceRecord() {
	t.Records = make([]BalanceRecord, 0)
}

func RlpHash(x interface{}) (h common.Hash) {
	return rlpHash(x)
}