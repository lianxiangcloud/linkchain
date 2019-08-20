package types

import (
	"math/big"
)

type BlobData string

type BlockCompleteEntry struct {
	Block BlobData
	Txs   []BlobData
}

type ParsedBlock struct {
	Hash     Hash
	Block    Block
	Txes     []Transaction
	OIndices BlockOutputIndices
	HasErr   bool
}

// type CryptoHash struct{

// }
type CryptoNoteBlock struct {
}
type CryptonoteTransaction struct {
}
type BlockOutputIndices struct {
	Indices []TxOutputIndices
}
type TxOutputIndices struct {
	Indices []uint64
}

type TxCacheData struct {
	TxExtraFields []TxExtraField
	// Primary       []IsOutData
	// Additional    []IsOutData
}
type TxExtraField struct{}

func (tef *TxExtraField) Clear() {
	// TODO
}

type BlockHeader struct {
	MajorVersion uint8  `json:"major_version"`
	MinorVersion uint8  `json:"minor_version"` // now used as a voting mechanism, rather than how this particular block is built
	Timestamp    uint64 `json:"timestamp"`
	PrevID       string `json:"prev_id"`
	Nonce        uint32 `json:"nonce"`
}

type Block struct {
	BlockHeader
	hashValid bool        //`json:"block"`
	MinerTx   Transaction `json:"miner_tx"`
	TxHashes  []string    `json:"tx_hashes"`

	// hash cash
	Hash string `json:"block"`
}

func NewBlock() *Block {
	return &Block{
		hashValid: false,
	}
}

//   block(const block &b): block_header(b), hash_valid(false), miner_tx(b.miner_tx), tx_hashes(b.tx_hashes) { if (b.is_hash_valid()) { hash = b.hash; set_hash_valid(true); } }
//   block &operator=(const block &b) { block_header::operator=(b); hash_valid = false; miner_tx = b.miner_tx; tx_hashes = b.tx_hashes; if (b.is_hash_valid()) { hash = b.hash; set_hash_valid(true); } return *this; }

func (b *Block) InvalidateHashes()   { b.hashValid = false }
func (b *Block) IsHashValid() bool   { return b.hashValid }
func (b *Block) SetHashValid(v bool) { b.hashValid = v }

type TransactionPrefix struct {
	// tx information
	Version    int    `json:"version"`
	UnlockTime uint64 `json:"unlock_time"` //number of block (or time), used as a limitation like: spend this tx not early then block/time

	Vin  []TxinV `json:"vin"`
	Vout []TxOut `json:"vout"`
	//extra
	Extra []uint8 `json:"extra"`
}

// TxinV type is in(TxinGen TxinToScript TxinToScripthash TxinToKey)
type TxinV interface{}

type TxOut struct {
	Amount *big.Int
	Target TxoutTargetV
}

// TxoutTargetV type is in(TxoutToScript TxoutToScripthash TxoutToKey)
type TxoutTargetV struct{}

type TxinGen struct {
	Height uint64 `json:"height"`
}

type TxinToScript struct {
	Prev    Hash
	Prevout int
	Sigset  []uint8
}

type TxinToScripthash struct {
	Prev    Hash
	Prevout int
	Script  TxoutToScript
	Sigset  []uint8
}

type TxoutToScript struct {
	Keys   []PublicKey
	Script []uint8
}

type TxinToKey struct {
	Amount     *big.Int
	KeyOffsets []uint64
	KImage     KeyImage // double spending protection
}

type TxoutToScripthash struct {
	Hash Hash
}

type TxoutToKey struct {
	Key PublicKey
}

type Transaction struct {
	TransactionPrefix

	hashValid     bool
	blobSizeValid bool

	Signatures    [][]Signature //count signatures  always the same as inputs count
	RctSignatures RctSig        `json:"rct_signatures"`

	Hash     Hash
	BlobSize int
}

func (t *Transaction) InvalidateHashes() {
	t.SetHashValid(false)
	t.SetBlobSizeValid(false)
}

func (t *Transaction) SetHashValid(v bool) {
	t.hashValid = v
}
func (t *Transaction) SetBlobSizeValid(v bool) {
	t.blobSizeValid = v
}
func (t *Transaction) IsHashValid() bool {
	return t.hashValid
}
func (t *Transaction) IstBlobSizeValid() bool {
	return t.blobSizeValid
}
