package types

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/common"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto/merkle"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"golang.org/x/crypto/sha3"
)

// Block defines the atomic unit of a blockchain.
// TODO: add Version byte
type Block struct {
	mtx        sync.Mutex
	*Header    `json:"header"`
	*Data      `json:"data"`
	Evidence   EvidenceData `json:"evidence"`
	LastCommit *Commit      `json:"last_commit"`

	// caches
	hash atomic.Value
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	ser.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

// MakeBlock returns a new block with an empty header, except what can be computed from itself.
// It populates the same set of fields validated by ValidateBasic
func MakeBlock(height uint64, txs []Tx, commit *Commit) *Block {
	block := &Block{
		Header: &Header{
			Height: height,
			Time:   uint64(time.Now().Unix()),
			NumTxs: uint64(len(txs)),
		},
		LastCommit: commit,
		Data: &Data{
			Txs: txs,
		},
	}
	return block
}

func NewBlock(header *Header) *Block {
	b := &Block{Header: CopyHeader(header)}
	return b
}

// AddEvidence appends the given evidence to the block
func (b *Block) AddEvidence(evidence []Evidence) {
	if b == nil {
		return
	}
	b.Evidence.Evidence = append(b.Evidence.Evidence, evidence...)
}

// ValidateBasic performs basic validation that doesn't involve state data.
// It checks the internal consistency of the block.
func (b *Block) ValidateBasic() error {
	if b == nil {
		return errors.New("Nil blocks are invalid")
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	newTxs := uint64(len(b.Data.Txs))
	if b.NumTxs != newTxs {
		return fmt.Errorf("Wrong Block.Header.NumTxs. Expected %v, got %v", newTxs, b.NumTxs)
	}
	if !bytes.Equal(b.LastCommitHash.Bytes(), b.LastCommit.Hash().Bytes()) {
		return fmt.Errorf("Wrong Block.Header.LastCommitHash.  Expected %v, got %v", b.LastCommitHash, b.LastCommit.Hash())
	}
	if b.Header.Height != BlockHeightOne {
		if err := b.LastCommit.ValidateBasic(); err != nil {
			return err
		}
	}
	if !bytes.Equal(b.DataHash.Bytes(), b.Data.Hash().Bytes()) {
		return fmt.Errorf("Wrong Block.Header.DataHash.  Expected %v, got %v", b.DataHash, b.Data.Hash())
	}
	if !bytes.Equal(b.EvidenceHash.Bytes(), b.Evidence.Hash().Bytes()) {
		return errors.New(cmn.Fmt("Wrong Block.Header.EvidenceHash.  Expected %v, got %v", b.EvidenceHash, b.Evidence.Hash()))
	}
	return nil
}

// fillHeader fills in any remaining header fields that are a function of the block data
func (b *Block) fillHeader() {
	b.LastCommitHash = b.LastCommit.Hash()
	b.EvidenceHash = b.Evidence.Hash()
}

// Hash computes and returns the block hash.
// If the block is incomplete, block hash is nil for safety.
func (b *Block) Hash() common.Hash {
	if b == nil {
		return common.EmptyHash
	}
	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}

	b.mtx.Lock()
	defer b.mtx.Unlock()

	if b.Header == nil || b.Data == nil || b.LastCommit == nil {
		return common.EmptyHash
	}

	v := b.Header.Hash()
	if v != common.EmptyHash {
		b.hash.Store(v)
	}
	return v
}

// MakePartSet returns a PartSet containing parts of a serialized block.
// This is the form in which the block is gossipped to peers.
func (b *Block) MakePartSet(partSize int) *PartSet {
	if b == nil {
		return nil
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	// We prefix the byte length, so that unmarshaling
	// can easily happen via a reader.
	bz, err := ser.EncodeToBytes(b)
	if err != nil {
		panic(err)
	}
	return NewPartSetFromData(bz, partSize)
}

// HashesTo is a convenience function that checks if a block hashes to the given argument.
// Returns false if the block is nil or the hash is empty.
func (b *Block) HashesTo(hash []byte) bool {
	if len(hash) == 0 {
		return false
	}
	if b == nil {
		return false
	}
	return bytes.Equal(b.Hash().Bytes(), hash)
}

// Size returns size of the block in bytes.
func (b *Block) Size() int {
	bz, err := ser.EncodeToBytes(b)
	if err != nil {
		return 0
	}
	return len(bz)
}

// String returns a string representation of the block
func (b *Block) String() string {
	return b.StringIndented("")
}

// StringIndented returns a string representation of the block
func (b *Block) StringIndented(indent string) string {
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf(`Block{
%s  %v
%s  %v
%s  %v
%s  %v
%s}#%v`,
		indent, b.Header.StringIndented(indent+"  "),
		indent, b.Data.StringIndented(indent+"  "),
		indent, b.Evidence.StringIndented(indent+"  "),
		indent, b.LastCommit.StringIndented(indent+"  "),
		indent, b.Hash().String())
}

// StringShort returns a shortened string representation of the block
func (b *Block) StringShort() string {
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf("Block#%v", b.Hash().String())
}

func (b *Block) GasLimit() uint64  { return b.Header.GasLimit }
func (b *Block) GasUsed() uint64   { return b.Header.GasUsed }
func (b *Block) Time() uint64      { return b.Header.Time }
func (b *Block) HeightU64() uint64 { return b.Height }

func (b *Block) HeightBigInt() *big.Int   { return new(big.Int).SetUint64(b.Height) }
func (b *Block) Bloom() Bloom             { return b.Header.bloom }
func (b *Block) Coinbase() common.Address { return b.Header.Coinbase }
func (b *Block) Statehash() common.Hash   { return b.Header.StateHash }
func (b *Block) Parenthash() common.Hash  { return b.Header.ParentHash }
func (b *Block) TxHash() common.Hash      { return b.Header.DataHash }
func (b *Block) Receipthash() common.Hash { return b.Header.ReceiptHash }

func (b *Block) Head() *Header { return CopyHeader(b.Header) }

//-----------------------------------------------------------------------------

// Header defines the structure of a block header
type Header struct {
	// basic block info
	ChainID  string         `json:"chain_id"`
	Height   uint64         `json:"height"`
	Coinbase common.Address `json:"miner"`
	Time     uint64         `json:"time"`
	NumTxs   uint64         `json:"num_txs"`
	TotalTxs uint64         `json:"total_txs"`
	Recover  uint32         `json:"recover"`

	// prev block info
	ParentHash     common.Hash `json:"parent_hash"` // hash from the prev block
	LastBlockID    BlockID     `json:"last_block_id"`
	LastCommitHash common.Hash `json:"last_commit_hash"` // commit from validators from the last block

	// hashes from the current block
	ValidatorsHash common.Hash `json:"validators_hash"` // validators for the current block
	ConsensusHash  common.Hash `json:"consensus_hash"`  // consensus params for current block
	DataHash       common.Hash `json:"data_hash"`       // transactions
	StateHash      common.Hash `json:"state_hash"`
	ReceiptHash    common.Hash `json:"receipts_hash"`
	GasLimit       uint64      `json:"gasLimit"`
	GasUsed        uint64      `json:"gasUsed"`

	// consensus info
	EvidenceHash common.Hash `json:"evidence_hash"` // evidence included in the block./
	bloom        Bloom
}

func CopyHeader(h *Header) *Header {
	cpy := *h
	cpy.SetBloom(h.Bloom())
	return &cpy
}

func (h *Header) Bloom() Bloom {
	return h.bloom
}

func (h *Header) SetBloom(bloom Bloom) {
	h.bloom = bloom
}

// Hash returns the hash of the header.
// Returns nil if ValidatorHash is missing,
// since a Header is not valid unless there is
// a ValidaotrsHash (corresponding to the validator set).
func (h *Header) Hash() common.Hash {
	if h == nil {
		return common.EmptyHash
	}

	hash := merkle.SimpleHashFromMap(map[string]merkle.Hasher{
		"ChainID":        aminoHasher(h.ChainID),
		"Height":         aminoHasher(h.Height),
		"Coinbase":       aminoHasher(h.Coinbase),
		"Time":           aminoHasher(h.Time),
		"NumTxs":         aminoHasher(h.NumTxs),
		"TotalTxs":       aminoHasher(h.TotalTxs),
		"ParentHash":     aminoHasher(h.ParentHash),
		"LastBlockID":    aminoHasher(h.LastBlockID),
		"LastCommitHash": aminoHasher(h.LastCommitHash),
		"ValidatorsHash": aminoHasher(h.ValidatorsHash),
		"ConsensusHash":  aminoHasher(h.ConsensusHash),
		"DataHash":       aminoHasher(h.DataHash),
		"StateHash":      aminoHasher(h.StateHash),
		"ReceiptHash":    aminoHasher(h.ReceiptHash),
		"GasLimit":       aminoHasher(h.GasLimit),
		"GasUsed":        aminoHasher(h.GasUsed),
		"EvidenceHash":   aminoHasher(h.EvidenceHash),
	})
	return common.BytesToHash(hash)
}

// StringIndented returns a string representation of the header
func (h *Header) StringIndented(indent string) string {
	if h == nil {
		return "nil-Header"
	}
	return fmt.Sprintf(`Header{
%s  ChainID:        %v
%s  Height:         %v
%s  Recover:        %v
%s  Coinbase:       %v
%s  Time:           %v
%s  NumTxs:         %v
%s  TotalTxs:       %v
%s  ParentHash:     %v
%s  LastBlockID:    %v
%s  LastCommitHash: %v
%s  ValidatorsHash: %v
%s  ConsensusHash:  %v
%s  DataHash:       %v
%s  StateHash:      %v
%s  ReceiptHash:    %v
%s  logsBloom:      %v
%s  GasLimit:       %v
%s  GasUsed:        %v
%s  EvidenceHash:   %v
%s}#%v`,
		indent, h.ChainID,
		indent, h.Height,
		indent, h.Recover,
		indent, h.Coinbase.String(),
		indent, time.Unix(int64(h.Time), 0).Local(),
		indent, h.NumTxs,
		indent, h.TotalTxs,
		indent, h.ParentHash.String(),
		indent, h.LastBlockID,
		indent, h.LastCommitHash.String(),
		indent, h.ValidatorsHash.String(),
		indent, h.ConsensusHash.String(),
		indent, h.DataHash.String(),
		indent, h.StateHash.String(),
		indent, h.ReceiptHash.String(),
		indent, h.bloom.Big(),
		indent, h.GasLimit,
		indent, h.GasUsed,
		indent, h.EvidenceHash.String(),
		indent, h.Hash().String())
}

//-------------------------------------

// Commit contains the evidence that a block was committed by a set of validators.
// NOTE: Commit is empty for height 1, but never nil.
type Commit struct {
	// NOTE: The Precommits are in order of address to preserve the bonded ValidatorSet order.
	// Any peer with a block can gossip precommits by index with a peer without recalculating the
	// active ValidatorSet.
	BlockID    BlockID `json:"block_id"`
	Precommits []*Vote `json:"precommits"`

	// Volatile
	firstPrecommit *Vote
	hash           cmn.HexBytes
	bitArray       *cmn.BitArray
}

// FirstPrecommit returns the first non-nil precommit in the commit.
// If all precommits are nil, it returns an empty precommit with height 0.
func (commit *Commit) FirstPrecommit() *Vote {
	if len(commit.Precommits) == 0 {
		return nil
	}
	if commit.firstPrecommit != nil {
		return commit.firstPrecommit
	}
	for _, precommit := range commit.Precommits {
		if precommit != nil {
			commit.firstPrecommit = precommit
			return precommit
		}
	}
	return &Vote{
		Type: VoteTypePrecommit,
	}
}

// Height returns the height of the commit
func (commit *Commit) Height() uint64 {
	if len(commit.Precommits) == 0 {
		return 0
	}
	return commit.FirstPrecommit().Height
}

// Round returns the round of the commit
func (commit *Commit) Round() int {
	if len(commit.Precommits) == 0 {
		return 0
	}
	return commit.FirstPrecommit().Round
}

// Type returns the vote type of the commit, which is always VoteTypePrecommit
func (commit *Commit) Type() byte {
	return VoteTypePrecommit
}

// Size returns the number of votes in the commit
func (commit *Commit) Size() int {
	if commit == nil {
		return 0
	}
	return len(commit.Precommits)
}

// BitArray returns a BitArray of which validators voted in this commit
func (commit *Commit) BitArray() *cmn.BitArray {
	if commit.bitArray == nil {
		commit.bitArray = cmn.NewBitArray(len(commit.Precommits))
		for i, precommit := range commit.Precommits {
			// TODO: need to check the BlockID otherwise we could be counting conflicts,
			// not just the one with +2/3 !
			commit.bitArray.SetIndex(i, precommit != nil)
		}
	}
	return commit.bitArray
}

// GetByIndex returns the vote corresponding to a given validator index
func (commit *Commit) GetByIndex(index int) *Vote {
	return commit.Precommits[index]
}

// IsCommit returns true if there is at least one vote
func (commit *Commit) IsCommit() bool {
	return len(commit.Precommits) != 0
}

// ValidateBasic performs basic validation that doesn't involve state data.
func (commit *Commit) ValidateBasic() error {
	if commit.BlockID.IsZero() {
		return errors.New("Commit cannot be for nil block")
	}
	if len(commit.Precommits) == 0 {
		return errors.New("No precommits in commit")
	}
	height, round := commit.Height(), commit.Round()

	// validate the precommits
	for _, precommit := range commit.Precommits {
		// It's OK for precommits to be missing.
		if precommit == nil {
			continue
		}
		// Ensure that all votes are precommits
		if precommit.Type != VoteTypePrecommit {
			return fmt.Errorf("Invalid commit vote. Expected precommit, got %v",
				precommit.Type)
		}
		// Ensure that all heights are the same
		if precommit.Height != height {
			return fmt.Errorf("Invalid commit precommit height. Expected %v, got %v",
				height, precommit.Height)
		}
		// Ensure that all rounds are the same
		if precommit.Round != round {
			return fmt.Errorf("Invalid commit precommit round. Expected %v, got %v",
				round, precommit.Round)
		}
	}
	return nil
}

// Hash returns the hash of the commit
func (commit *Commit) Hash() common.Hash {
	if commit == nil {
		return common.EmptyHash
	}

	if commit.hash == nil {
		bs := make([]merkle.Hasher, len(commit.Precommits))
		for i, precommit := range commit.Precommits {
			bs[i] = aminoHasher(precommit)
		}
		commit.hash = merkle.SimpleHashFromHashers(bs)
	}

	return common.BytesToHash(commit.hash)
}

// StringIndented returns a string representation of the commit
func (commit *Commit) StringIndented(indent string) string {
	if commit == nil {
		return "nil-Commit"
	}
	precommitStrings := make([]string, len(commit.Precommits))
	for i, precommit := range commit.Precommits {
		precommitStrings[i] = precommit.String()
	}
	return fmt.Sprintf(`Commit{
%s  BlockID:    %v
%s  Precommits: %v
%s}#%v`,
		indent, commit.BlockID,
		indent, strings.Join(precommitStrings, "\n"+indent+"  "),
		indent, commit.Hash().String())
}

//-----------------------------------------------------------------------------

// SignedHeader is a header along with the commits that prove it
type SignedHeader struct {
	Header *Header `json:"header"`
	Commit *Commit `json:"commit"`
}

//-----------------------------------------------------------------------------

// Data contains the set of transactions included in the block
type Data struct {

	// Txs that included in the block
	Txs Txs `json:"txs"`

	// caches
	hash atomic.Value
}

// Hash returns the hash of the data
func (data *Data) Hash() common.Hash {
	if data == nil {
		return (Txs{}).Hash()
	}

	if hash := data.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}

	v := data.Txs.Hash()
	if v != (common.EmptyHash) {
		data.hash.Store(v)
	}
	return v
}

// StringIndented returns a string representation of the transactions
func (data *Data) StringIndented(indent string) string {
	if data == nil {
		return "nil-Data"
	}
	txStrings := make([]string, cmn.MinInt(len(data.Txs), 21))
	for i, tx := range data.Txs {
		if i == 20 {
			txStrings[i] = fmt.Sprintf("... (%v total)", len(data.Txs))
			break
		}
		txStrings[i] = fmt.Sprintf("%v", tx.Hash().String())
	}
	return fmt.Sprintf(`Data{
%s  %v
%s}#%v`,
		indent, strings.Join(txStrings, "\n"+indent+"  "),
		indent, data.Hash().String())
}

// type dataJSON struct {
// 	Txs [][]byte `json:"txs"`
// }

// func (data *Data) MarshalJSON() ([]byte, error) {
// 	dStruct := dataJSON{}
// 	cnt := len(data.Txs)
// 	dStruct.Txs = make([][]byte, cnt)
// 	var d []byte
// 	var err error
// 	for index := 0; index < cnt; index++ {
// 		var txFix [4]byte
// 		d = []byte{}
// 		err = nil
// 		switch t := data.Txs[index].(type) {
// 		case *Transaction:
// 			txFix = TNormal
// 			d, err = json.Marshal(t)
// 			if err != nil {
// 				return nil, err
// 			}
// 		case *UTXOTransaction:
// 			txFix = TUtxo
// 			d, err = json.Marshal(t)
// 			if err != nil {
// 				return nil, err
// 			}
// 		default:
// 			return nil, ErrTxNotSupport
// 		}

// 		dStruct.Txs[index] = append(txFix[:], d...)
// 	}
// 	return json.Marshal(dStruct)
// }

// func (data *Data) UnmarshalJSON(j []byte) error {
// 	dStruct := new(dataJSON)
// 	err := json.Unmarshal(j, &dStruct)
// 	if err != nil {
// 		return err
// 	}
// 	data = &Data{}
// 	cnt := len(dStruct.Txs)
// 	data.Txs = make([]Tx, cnt)
// 	for index := 0; index < cnt; index++ {
// 		txType := [4]byte{dStruct.Txs[index][0], dStruct.Txs[index][1], dStruct.Txs[index][2], dStruct.Txs[index][3]}
// 		// txType = []byte(dStruct.Txs[index][:2])
// 		fmt.Printf("txType:%v\n", txType)
// 		encodedTx := dStruct.Txs[index][4:]
// 		var tx Tx
// 		switch txType {
// 		case TNormal:
// 			tx = new(Transaction)
// 			// case types.TxToken:
// 			// 	tx = new(types.TokenTransaction)
// 			// case types.TxContractCreate:
// 			// 	tx = new(types.ContractCreateTx)
// 			// case types.TxContractUpgrade:
// 			// 	tx = new(types.ContractUpgradeTx)
// 			// case types.TxMultiSignAccount:
// 			// 	tx = new(types.MultiSignAccountTx)
// 		case TUtxo:
// 			fmt.Printf("found UTXOTransaction tx\n")
// 			tx = new(UTXOTransaction)
// 		default:
// 			return ErrTxNotSupport
// 		}

// 		if err := json.Unmarshal(encodedTx, tx); err != nil {
// 			return err
// 		}
// 		tt := tx.(*UTXOTransaction)
// 		fmt.Printf("found UTXOTransaction tx,Extra:%v\n", tt.Extra)
// 		data.Txs[index] = tx
// 	}
// 	return nil
// }

//-----------------------------------------------------------------------------

// EvidenceData contains any evidence of malicious wrong-doing by validators
type EvidenceData struct {
	Evidence EvidenceList `json:"evidence"`

	// Volatile
	hash cmn.HexBytes
}

// Hash returns the hash of the data.
func (data *EvidenceData) Hash() common.Hash {
	if data.hash == nil {
		data.hash = data.Evidence.Hash()
	}
	return common.BytesToHash(data.hash)
}

// StringIndented returns a string representation of the evidence.
func (data *EvidenceData) StringIndented(indent string) string {
	if data == nil {
		return "nil-Evidence"
	}
	evStrings := make([]string, cmn.MinInt(len(data.Evidence), 21))
	for i, ev := range data.Evidence {
		if i == 20 {
			evStrings[i] = fmt.Sprintf("... (%v total)", len(data.Evidence))
			break
		}
		evStrings[i] = fmt.Sprintf("Evidence:%v", ev)
	}
	return fmt.Sprintf(`EDCData{
%s  %v
%s}#%v`,
		indent, strings.Join(evStrings, "\n"+indent+"  "),
		indent, data.hash)
	return ""
}

//--------------------------------------------------------------------------------

// BlockID defines the unique ID of a block as its Hash and its PartSetHeader
type BlockID struct {
	Hash        common.Hash   `json:"hash"`
	PartsHeader PartSetHeader `json:"parts"`
}

// IsZero returns true if this is the BlockID for a nil-block
func (blockID BlockID) IsZero() bool {
	return blockID.Hash == common.EmptyHash && blockID.PartsHeader.IsZero()
}

// Equals returns true if the BlockID matches the given BlockID
func (blockID BlockID) Equals(other BlockID) bool {
	return bytes.Equal(blockID.Hash.Bytes(), other.Hash.Bytes()) &&
		blockID.PartsHeader.Equals(other.PartsHeader)
}

// Key returns a machine-readable string representation of the BlockID
func (blockID BlockID) Key() string {
	bz, err := ser.EncodeToBytes(blockID.PartsHeader)
	if err != nil {
		panic(err)
	}
	return blockID.Hash.String() + string(bz)
}

// String returns a human readable string representation of the BlockID
func (blockID BlockID) String() string {
	return fmt.Sprintf(`%v:%v`, blockID.Hash.String(), blockID.PartsHeader)
}

//-------------------------------------------------------

type hasher struct {
	item interface{}
}

func (h hasher) Hash() []byte {
	hasher := sha3.NewLegacyKeccak256()
	if h.item != nil && !cmn.IsTypedNil(h.item) && !cmn.IsEmpty(h.item) {
		bz, err := ser.EncodeToBytes(h.item)
		if err != nil {
			panic(err)
		}
		_, err = hasher.Write(bz)
		if err != nil {
			panic(err)
		}
	}
	return hasher.Sum(nil)

}

func aminoHash(item interface{}) []byte {
	h := hasher{item}
	return h.Hash()
}

func aminoHasher(item interface{}) merkle.Hasher {
	return hasher{item}
}
