package types

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"sync/atomic"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/crypto/merkle"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

var (
	TNormal = [4]byte{0x0, 0x0, 0x0, 0x0}
	TUtxo   = [4]byte{0x0, 0x0, 0x0, 0x1}
)

// TxData represent a tx content
type Tx interface {
	Hash() common.Hash
	From() (common.Address, error)
	To() *common.Address
	TokenAddress() common.Address
	TypeName() string

	CheckBasic(censor TxCensor) error
	CheckState(censor TxCensor) error
}

type IMessage interface {
	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int
	Nonce() uint64
	Data() []byte
	AsMessage() (Message, error)
}

type RegularTx interface {
	Tx
	IMessage
}

func RegisterTxData() {
	ser.RegisterInterface((*Tx)(nil), nil)
	ser.RegisterInterface((*RegularTx)(nil), nil)
	ser.RegisterConcrete(&Transaction{}, TxNormal, nil)
	ser.RegisterConcrete(&TokenTransaction{}, TxToken, nil)
	ser.RegisterConcrete(&MultiSignAccountTx{}, TxMultiSignAccount, nil)
	ser.RegisterConcrete(&ContractUpgradeTx{}, TxContractUpgrade, nil)
	RegisterUTXOTxData()
}

var (
	logger = log.Root()
)

func init() {
	logger.SetHandler(log.StdoutHandler)
}

//SetLogger set log object
func SetLogger(l log.Logger) {
	logger = l
}

func GetLogger() log.Logger {
	return logger
}

func transactionHash(hashcache *atomic.Value, tx interface{}) common.Hash {
	if hash := hashcache.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := rlpHash(tx)
	hashcache.Store(v)
	return v
}

// Txs is a slice of Tx.
type Txs []Tx

// Hash returns the simple Merkle root hash of the transactions.
func (txs Txs) Hash() common.Hash {
	switch len(txs) {
	case 0:
		return common.EmptyHash
	case 1:
		return txs[0].Hash()
	default:
		left := Txs(txs[:(len(txs)+1)/2]).Hash().Bytes()
		right := Txs(txs[(len(txs)+1)/2:]).Hash().Bytes()
		hash := merkle.SimpleHashFromTwoHashes(left, right)
		return common.BytesToHash(hash)
	}
}

type txsJSON struct {
	Txs []string
}

// MarshalJSON returns *m as the JSON encoding of m.
func (txs *Txs) MarshalJSON() ([]byte, error) {
	cntTx := len(*txs)
	js := txsJSON{}
	js.Txs = make([]string, cntTx)

	for i, v := range *txs {
		bz, err := ser.EncodeToBytes(&v)
		if err != nil {
			return nil, err
		}
		js.Txs[i] = hex.EncodeToString(bz)
	}
	return json.Marshal(js)
}

// UnmarshalJSON sets *m to a copy of data.
func (txs *Txs) UnmarshalJSON(data []byte) error {
	if txs == nil {
		return errors.New("RawString: UnmarshalJSON on nil pointer")
	}
	keyJSON := new(txsJSON)
	err := json.Unmarshal(data, &keyJSON)
	if err != nil {
		return err
	}

	cntTx := len(keyJSON.Txs)

	for index := 0; index < cntTx; index++ {
		var tx Tx
		data, err := hex.DecodeString(keyJSON.Txs[index])
		if err != nil {
			return err
		}
		if err := ser.DecodeBytes(data, &tx); err != nil {
			return err
		}
		*txs = append(*txs, tx)
	}
	return nil
}

// IndexByHash returns the index of this transaction hash in the list, or -1 if not found
func (txs Txs) IndexByHash(hash common.Hash) int {
	for i := range txs {
		if bytes.Equal(txs[i].Hash().Bytes(), hash.Bytes()) {
			return i
		}
	}
	return -1
}

// TxProof represents a Merkle proof of the presence of a transaction in the Merkle tree.
type TxProof struct {
	Index, Total int
	RootHash     common.Hash
	Data         Tx
	Proof        merkle.SimpleProof
}

// LeadHash returns the hash of the this proof refers to.
func (tp TxProof) LeafHash() common.Hash {
	return tp.Data.Hash()
}

// Validate verifies the proof. It returns nil if the RootHash matches the dataHash argument,
// and if the proof is internally consistent. Otherwise, it returns a sensible error.
func (tp TxProof) Validate(dataHash common.Hash) error {
	if !bytes.Equal(dataHash.Bytes(), tp.RootHash.Bytes()) {
		return errors.New("Proof matches different data hash")
	}
	if tp.Index < 0 {
		return errors.New("Proof index cannot be negative")
	}
	if tp.Total <= 0 {
		return errors.New("Proof total must be positive")
	}
	valid := tp.Proof.Verify(tp.Index, tp.Total, tp.LeafHash().Bytes(), tp.RootHash.Bytes())
	if !valid {
		return errors.New("Proof is not internally consistent")
	}
	return nil
}

// TxsResult contains results of executing the transactions.
type TxsResult struct {
	GasUsed       uint64                       `json:"gasUsed"`
	TrieRoot      common.Hash                  `json:"trieRoot"`
	StateHash     common.Hash                  `json:"state_hash"`
	ReceiptHash   common.Hash                  `json:"receipts_hash"`
	LogsBloom     Bloom                        `json:"logs_bloom"`
	Candidates    []*CandidateInOrder          `json:"candidates"` //candidates in order
	CandidatesMap map[string]*CandidateInOrder `json:"-" rlp:"-"`
	specialTxs    []Tx
	utxoOutPuts   []*UTXOOutputData
	keyImages     []*types.Key
}

func (txResult *TxsResult) SpecialTxs() []Tx       { return txResult.specialTxs }
func (txResult *TxsResult) SetSpecialTxs(txs []Tx) { txResult.specialTxs = txs }

func (txResult *TxsResult) UpdateCandidates(candidates []*CandidateInOrder) {
	txResult.Candidates = candidates
	txResult.CandidatesMap = make(map[string]*CandidateInOrder)
}

func (txResult *TxsResult) SetCandidates(candidates []*CandidateInOrder) {
	can := make([]*CandidateInOrder, 0, len(candidates))
	canMap := make(map[string]*CandidateInOrder)
	for _, v := range candidates {
		vCopy := v.Copy()
		can = append(can, vCopy)
		canMap[v.Address.String()] = vCopy
	}
	txResult.Candidates = can
	txResult.CandidatesMap = canMap
}
func (txresult *TxsResult) SetUTXOOutputs(utxoOutputData []*UTXOOutputData) {
	txresult.utxoOutPuts = utxoOutputData
}
func (txresult *TxsResult) UTXOOutputs() []*UTXOOutputData {
	return txresult.utxoOutPuts
}
func (txresult *TxsResult) SetKeyImages(keyImages []*types.Key) {
	txresult.keyImages = keyImages
}
func (txresult *TxsResult) KeyImages() []*types.Key {
	return txresult.keyImages
}

///////////////////////////////////////////////////////////////////////////
func jsonHash(x interface{}) (h common.Hash) {
	b, _ := json.Marshal(x)
	return crypto.Keccak256Hash(b)
}
