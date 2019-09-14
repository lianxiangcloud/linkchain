package rtypes

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/common"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	cptypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/p2p"
	pcomm "github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
)

type ResultValidators struct {
	BlockHeight       uint64             `json:"block_height"`
	LastHeightChanged uint64             `json:"last_changed_height"`
	Validators        []*types.Validator `json:"validators"`
}

func (r ResultValidators) MarshalJSON() ([]byte, error) {
	type data ResultValidators
	enc := data(r)

	return ser.MarshalJSON(enc)
}
func (r *ResultValidators) UnmarshalJSON(input []byte) error {
	type data ResultValidators
	enc := data(*r)
	err := ser.UnmarshalJSON(input, &enc)
	if err == nil {
		*r = ResultValidators(enc)
	}
	return err
}

type PeerStateInfo struct {
	NodeAddress string          `json:"node_address"`
	PeerState   json.RawMessage `json:"peer_state"`
}

// UNSTABLE
type ResultConsensusState struct {
	RoundState json.RawMessage `json:"round_state"`
}

type ResultDumpConsensusState struct {
	RoundState json.RawMessage `json:"round_state"`
	Peers      []PeerStateInfo `json:"peers"`
}

type ResultBlockHeader struct {
	*types.Header
}

func (b ResultBlockHeader) MarshalJSON() ([]byte, error) {
	return ser.MarshalJSON(b.Header)
}

// Single block (with meta)
type ResultBlock struct {
	BlockMeta *types.BlockMeta `json:"block_meta"`
	Block     *types.Block     `json:"block"`
}

func (r ResultBlock) MarshalJSON() ([]byte, error) {
	type data ResultBlock
	enc := data(r)
	return ser.MarshalJSON(enc)
}

func (r *ResultBlock) UnmarshalJSON(input []byte) error {
	type data ResultBlock
	enc := data(*r)
	err := ser.UnmarshalJSON(input, &enc)
	if err == nil {
		*r = ResultBlock(enc)
	}
	return err
}

// Info about the node's syncing state
type SyncInfo struct {
	LatestBlockHash   cmn.HexBytes `json:"latest_block_hash"`
	LatestAppHash     cmn.HexBytes `json:"latest_app_hash"`
	LatestBlockHeight uint64       `json:"latest_block_height"`
	LatestBlockTime   time.Time    `json:"latest_block_time"`
	CatchingUp        bool         `json:"catching_up"`
}

// Info about the node's validator
type ValidatorInfo struct {
	Address     cmn.HexBytes  `json:"address"`
	PubKey      crypto.PubKey `json:"pub_key"`
	VotingPower int64         `json:"voting_power"`
}

// Node Status
type ResultStatus struct {
	NodeInfo      p2p.NodeInfo  `json:"node_info"`
	SyncInfo      SyncInfo      `json:"sync_info"`
	ValidatorInfo ValidatorInfo `json:"validator_info"`
}

// Is TxIndexing enabled
func (s *ResultStatus) TxIndexEnabled() bool {
	if s == nil {
		return false
	}
	for _, s := range s.NodeInfo.Other {
		info := strings.Split(s, "=")
		if len(info) == 2 && info[0] == "tx_index" {
			return info[1] == "on"
		}
	}
	return false
}

func (s ResultStatus) MarshalJSON() ([]byte, error) {
	type data ResultStatus
	enc := data(s)
	return ser.MarshalJSON(enc)
}

type Peer struct {
	p2p.NodeInfo     `json:"node_info"`
	IsOutbound       bool                 `json:"is_outbound"`
	ConnectionStatus p2p.ConnectionStatus `json:"connection_status"`
}

type Node struct {
	IP       string       `json:"ip"`       // len 4 for IPv4 or 16 for IPv6
	UDP_Port uint16       `json:"udp_port"` // port numbers
	TCP_Port uint16       `json:"tcp_port"` // port numbers
	ID       pcomm.NodeID `json:"id"`       // the node's public key
}

// Info about peer connections
type ResultNetInfo struct {
	Listening bool     `json:"listening"`
	Listeners []string `json:"listeners"`
	NPeers    int      `json:"n_peers"`
	Peers     []Peer   `json:"peers"`
	DHTPeers  []Node   `json:"dht_peers"`
}

func (r ResultNetInfo) MarshalJSON() ([]byte, error) {
	type data ResultNetInfo
	enc := data(r)
	return ser.MarshalJSON(enc)
}

type WholeBlock struct {
	Block    *RPCBlock      `json:"block"`
	Receipts types.Receipts `json:"receipts"`
}

func NewWholeBlock(block *types.Block, receipts types.Receipts) *WholeBlock {
	return &WholeBlock{
		Block:    NewRPCBlock(block, true, true),
		Receipts: receipts,
	}
}

type ReceiptsWithBlockHeight struct {
	BlockHeight uint64         `json:"height"`
	Receipts    types.Receipts `json:"receipts"`
}

func NewReceiptsWithBlockHeight(blockHeight uint64, receipts types.Receipts) *ReceiptsWithBlockHeight {
	return &ReceiptsWithBlockHeight{
		BlockHeight: blockHeight,
		Receipts:    receipts,
	}
}

type BalanceRecordsWithBlockHeight struct {
	BlockHeight         uint64                     `json:"height"`
	BlockBalanceRecords *types.BlockBalanceRecords `json:"block_balance_records"`
}

func NewBalanceRecordsWithBlockMsg(blockHeight uint64, bbr *types.BlockBalanceRecords) *BalanceRecordsWithBlockHeight {
	return &BalanceRecordsWithBlockHeight{
		BlockHeight:         blockHeight,
		BlockBalanceRecords: bbr,
	}
}

type ITX interface{}
type txsAlias Txs
type Txs []ITX

func (t Txs) MarshalJSON() ([]byte, error) {
	ec := txsAlias(t)
	return ser.MarshalJSON(ec)
}

func (t *Txs) UnmarshalJSON(input []byte) error {
	dec := &txsAlias{}
	err := ser.UnmarshalJSON(input, dec)
	if err == nil {
		*t = Txs(*dec)
	}
	return err
}

type rpcBlockAlias RPCBlock
type RPCBlock struct {
	Height          *hexutil.Big     `json:"number"`
	Hash            *common.Hash     `json:"hash"`
	Coinbase        *common.Address  `json:"miner"`
	Time            *hexutil.Big     `json:"timestamp"`
	ParentHash      common.Hash      `json:"parentHash"`
	DataHash        common.Hash      `json:"transactionsRoot"`
	StateHash       common.Hash      `json:"stateRoot"`
	ReceiptHash     common.Hash      `json:"receiptsRoot"`
	GasLimit        hexutil.Uint64   `json:"gasLimit"`
	GasUsed         hexutil.Uint64   `json:"gasUsed"`
	Bloom           types.Bloom      `json:"logsBloom"`
	Txs             Txs              `json:"transactions"`
	TokenOutputSeqs map[string]int64 `json:"token_output_seqs"`
}

// NewRPCBlock converts the given block to the RPC output which depends on fullTx. If inclTx is true transactions are
// returned. When fullTx is true the returned block contains full transaction details, otherwise it will only contain
// transaction hashes.
func NewRPCBlock(b *types.Block, inclTx bool, fullTx bool) *RPCBlock {
	if b == nil || b.Header == nil {
		return nil
	}
	head := b.Header // copies the header once
	hash := b.Hash()
	block := &RPCBlock{
		Height:      (*hexutil.Big)(big.NewInt(int64(head.Height))),
		Hash:        &hash,
		Coinbase:    &head.Coinbase,
		Time:        (*hexutil.Big)(big.NewInt(int64(head.Time))),
		ParentHash:  head.ParentHash,
		DataHash:    head.DataHash,
		StateHash:   b.StateHash,
		ReceiptHash: head.ReceiptHash,
		GasLimit:    hexutil.Uint64(head.GasLimit),
		GasUsed:     hexutil.Uint64(head.GasUsed),
		Bloom:       head.Bloom(),
	}

	if !inclTx {
		return block
	}

	formatTx := func(tx types.Tx, index uint64) interface{} {
		return tx.Hash()
	}
	if fullTx {
		formatTx = func(tx types.Tx, index uint64) interface{} {
			return NewRPCTx(tx, nil)
		}
	}

	txs := b.Txs
	transactions := make(Txs, 0, len(txs))
	for i, tx := range txs {
		if v := formatTx(tx, uint64(i)); v != nil {
			transactions = append(transactions, v)
		}
	}
	block.Txs = transactions

	return block
}

// NewRPCBlockUTXO converts the given block to the RPC output which depends on fullTx. If inclTx is true transactions are
// returned. When fullTx is true the returned block contains full transaction details, otherwise it will only contain
// transaction hashes.
// only return utxo txs.
func NewRPCBlockUTXO(b *types.Block, inclTx bool, fullTx bool, tokenOutputSeqs map[string]int64) *RPCBlock {
	if b == nil || b.Header == nil {
		return nil
	}
	head := b.Header // copies the header once
	hash := b.Hash()
	block := &RPCBlock{
		Height:          (*hexutil.Big)(big.NewInt(int64(head.Height))),
		Hash:            &hash,
		Coinbase:        &head.Coinbase,
		Time:            (*hexutil.Big)(big.NewInt(int64(head.Time))),
		ParentHash:      head.ParentHash,
		DataHash:        head.DataHash,
		StateHash:       b.StateHash,
		ReceiptHash:     head.ReceiptHash,
		GasLimit:        hexutil.Uint64(head.GasLimit),
		GasUsed:         hexutil.Uint64(head.GasUsed),
		Bloom:           head.Bloom(),
		TokenOutputSeqs: tokenOutputSeqs,
	}

	if !inclTx {
		return block
	}

	formatTx := func(tx types.Tx, index uint64) interface{} {
		return tx.Hash()
	}
	if fullTx {
		formatTx = func(tx types.Tx, index uint64) interface{} {
			return NewRPCTx(tx, nil)
		}
	}

	txs := b.Txs
	transactions := make(Txs, 0, len(txs))
	for i, tx := range txs {
		switch t := tx.(type) {
		case *types.UTXOTransaction:
			if v := formatTx(t, uint64(i)); v != nil {
				transactions = append(transactions, v)
			}
		default:
		}

	}
	block.Txs = transactions

	return block
}

type QuickRPCBlock struct {
	Block      *RPCBlock    `json:"block,omitempty"`
	NextHeight *hexutil.Big `json:"next_height"`
	MaxHeight  *hexutil.Big `json:"max_height"`
}

type RPCBalanceRecord struct {
	From            common.Address `json:"from"`
	To              common.Address `json:"to"`
	FromAddressType hexutil.Uint   `json:"from_address_type"`
	ToAddressType   hexutil.Uint   `json:"to_address_type"`
	Type            string         `json:"type"`
	TokenID         common.Address `json:"token_id"`
	Amount          *hexutil.Big   `json:"amount"`
}

type RPCTxBalanceRecords struct {
	Hash     common.Hash        `json:"hash"`
	Type     string             `json:"type"`
	Records  []RPCBalanceRecord `json:"records"`
	Payloads []*hexutil.Bytes   `json:"payloads"`
	Nonce    hexutil.Uint64     `json:"nonce"`
	GasLimit hexutil.Uint64     `json:"gas_limit"`
	GasPrice *hexutil.Big       `json:"gas_price"`
	From     common.Address     `json:"from"`
	To       common.Address     `json:"to"`
	TokenId  common.Address     `json:"token_id"`
}

type RPCBlockBalanceRecords struct {
	BlockTime hexutil.Uint64         `json:"block_time"`
	BlockHash common.Hash            `json:"block_hash"`
	TxRecords []*RPCTxBalanceRecords `json:"tx_records"`
}

func NewRPCBlockBalanceRecord(bbr *types.BlockBalanceRecords) *RPCBlockBalanceRecords {
	txRecords := make([]*RPCTxBalanceRecords, 0)
	for _, tx := range bbr.TxRecords {
		records := make([]RPCBalanceRecord, 0)
		for _, br := range tx.Records {
			record := RPCBalanceRecord{
				From:            br.From,
				To:              br.To,
				FromAddressType: hexutil.Uint(br.FromAddressType),
				ToAddressType:   hexutil.Uint(br.ToAddressType),
				Type:            br.Type,
				TokenID:         br.TokenID,
				Amount:          (*hexutil.Big)(br.Amount),
			}
			records = append(records, record)
		}
		payloads := make([]*hexutil.Bytes, 0)
		for _, payload := range tx.Payloads {
			payloads = append(payloads, (*hexutil.Bytes)(&payload))
		}
		txRecord := &RPCTxBalanceRecords{
			Hash:     tx.Hash,
			Type:     tx.Type,
			Payloads: payloads,
			Nonce:    hexutil.Uint64(tx.Nonce),
			GasLimit: hexutil.Uint64(tx.GasLimit),
			GasPrice: (*hexutil.Big)(tx.GasPrice),
			Records:  records,
			From:     tx.From,
			To:       tx.To,
			TokenId:  tx.TokenId,
		}
		txRecords = append(txRecords, txRecord)
	}
	rbbr := &RPCBlockBalanceRecords{
		BlockTime: hexutil.Uint64(bbr.BlockTime),
		BlockHash: bbr.BlockHash,
		TxRecords: txRecords,
	}
	return rbbr
}

type iRPCTx interface {
	TypeName() string
	Hash() common.Hash
	From() (common.Address, error)
}

//RPCTx represents a RPCTx that will serialize to the RPC representation of a tx.
type rpcTxAlias RPCTx
type RPCTx struct {
	TxType   string          `json:"txType"`
	TxHash   common.Hash     `json:"txHash"`
	SignHash *common.Hash    `json:"signHash,omitempty"`
	From     *common.Address `json:"from,omitempty"`
	Tx       types.Tx        `json:"tx"`
	TxEntry  *types.TxEntry  `json:"txEntry,omitempty"`
}

func (t RPCTx) MarshalJSON() ([]byte, error) {
	ec := rpcTxAlias(t)
	return ser.MarshalJSON(ec)
}

func (t *RPCTx) UnmarshalJSON(input []byte) error {
	dec := &rpcTxAlias{}
	err := ser.UnmarshalJSON(input, dec)
	if err == nil {
		*t = RPCTx(*dec)
	}
	return err
}

type signHasher interface {
	SignHash() common.Hash
}

// NewRPCTx returns a tx that will serialize to the RPC
// representation, with the given location metadata set (if available).
func NewRPCTx(tx types.Tx, entry *types.TxEntry) *RPCTx {
	if tx == nil {
		return nil
	}
	itx, ok := tx.(iRPCTx)
	if !ok {
		return nil
	}
	rpcTx := &RPCTx{
		TxEntry: entry,
		TxType:  itx.TypeName(),
		TxHash:  itx.Hash(),
		Tx:      tx,
	}
	if rpcTx.TxType == types.TxNormal || rpcTx.TxType == types.TxToken {
		if sh, ok := tx.(signHasher); ok {
			signHash := sh.SignHash()
			rpcTx.SignHash = &signHash
		}
	}

	if from, _ := itx.From(); from != common.EmptyAddress {
		rpcTx.From = &from
	}
	return rpcTx
}

type TxRecordReq struct {
	TxHash string `json:"tx_hash"`
	Type   string `json:"type"`
}

// SignTransactionResult represents a RLP encoded signed transaction.
type SignTransactionResult struct {
	Raw hexutil.Bytes `json:"raw"`
	Tx  types.Tx      `json:"tx"`
}

type RPCKey cptypes.Key

func (k RPCKey) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%x"`, k[:])), nil
}

func (k *RPCKey) UnmarshalJSON(input []byte) error {
	bytes, err := hex.DecodeString(string(input[1 : len(input)-1]))
	if err != nil {
		return err
	}
	copy(k[:], bytes)
	return nil
}

type RPCOutput struct {
	Out RPCKey `json:"out"`
	//UnlockTime uint64         `json:"unlock_time"`
	Height  uint64         `json:"height"`
	Commit  RPCKey         `json:"commit"`
	TokenID common.Address `json:"token"`
}
