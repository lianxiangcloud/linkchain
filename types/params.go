package types

import (
	"math/big"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto/merkle"
)

var (
	// BlockHeightZero is the genesis block height
	BlockHeightZero = uint64(0)
	BlockHeightOne  = BlockHeightZero + 1
)

func UpdateBlockHeightZero(height uint64) {
	BlockHeightZero = height
	BlockHeightOne = BlockHeightZero + 1
}

var IsTestMode = false

const (
	// BloomBitsBlocks is the number of blocks a single bloom bit section vector
	// contains.
	BloomBitsBlocks uint64 = 4096
)

const (
	// MaxBlockSizeBytes is the maximum permitted size of the blocks.
	MaxBlockSizeBytes = 104857600 // 100MB
)

// ConsensusParams contains consensus critical parameters
// that determine the validity of blocks.
type ConsensusParams struct {
	BlockSize      `json:"block_size_params"`
	TxSize         `json:"tx_size_params"`
	BlockGossip    `json:"block_gossip_params"`
	EvidenceParams `json:"evidence_params"`
}

// BlockSize contain limits on the block size.
type BlockSize struct {
	MaxBytes int    `json:"max_bytes"` // NOTE: must not be 0 nor greater than 100MB
	MaxTxs   int    `json:"max_txs"`
	MaxGas   uint64 `json:"max_gas"`
}

// TxSize contain limits on the tx size.
type TxSize struct {
	MaxBytes int    `json:"max_bytes"`
	MaxGas   uint64 `json:"max_gas"`
}

// BlockGossip determine consensus critical elements of how blocks are gossiped
type BlockGossip struct {
	BlockPartSizeBytes int `json:"block_part_size_bytes"` // NOTE: must not be 0
}

// EvidenceParams determine how we handle evidence of malfeasance
type EvidenceParams struct {
	MaxAge uint64 `json:"max_age"` // only accept new evidence more recent than this
}

// DefaultConsensusParams returns a default ConsensusParams.
func DefaultConsensusParams() *ConsensusParams {
	return &ConsensusParams{
		DefaultBlockSize(),
		DefaultTxSize(),
		DefaultBlockGossip(),
		DefaultEvidenceParams(),
	}
}

// DefaultBlockSize returns a default BlockSize.
func DefaultBlockSize() BlockSize {
	return BlockSize{
		MaxBytes: 22020096, // 21MB
		MaxTxs:   10000,
		MaxGas:   5000000000,
	}
}

// DefaultTxSize returns a default TxSize.
func DefaultTxSize() TxSize {
	return TxSize{
		MaxBytes: 10240, // 10kB
		MaxGas:   5000000000,
	}
}

// DefaultBlockGossip returns a default BlockGossip.
func DefaultBlockGossip() BlockGossip {
	return BlockGossip{
		BlockPartSizeBytes: 32 * 1024, // 32kB
	}
}

// DefaultEvidence Params returns a default EvidenceParams.
func DefaultEvidenceParams() EvidenceParams {
	return EvidenceParams{
		MaxAge: 100000, // 27.8 hrs at 1block/s
	}
}

// Validate validates the ConsensusParams to ensure all values
// are within their allowed limits, and returns an error if they are not.
func (params *ConsensusParams) Validate() error {
	// ensure some values are greater than 0
	if params.BlockSize.MaxBytes <= 0 {
		return cmn.NewError("BlockSize.MaxBytes must be greater than 0. Got %d", params.BlockSize.MaxBytes)
	}
	if params.BlockGossip.BlockPartSizeBytes <= 0 {
		return cmn.NewError("BlockGossip.BlockPartSizeBytes must be greater than 0. Got %d", params.BlockGossip.BlockPartSizeBytes)
	}

	// ensure blocks aren't too big
	if params.BlockSize.MaxBytes > MaxBlockSizeBytes {
		return cmn.NewError("BlockSize.MaxBytes is too big. %d > %d",
			params.BlockSize.MaxBytes, MaxBlockSizeBytes)
	}
	return nil
}

// Hash returns a merkle hash of the parameters to store
// in the block header
func (params *ConsensusParams) Hash() []byte {
	return merkle.SimpleHashFromMap(map[string]merkle.Hasher{
		"block_gossip_part_size_bytes": aminoHasher(params.BlockGossip.BlockPartSizeBytes),
		"block_size_max_bytes":         aminoHasher(params.BlockSize.MaxBytes),
		"block_size_max_gas":           aminoHasher(params.BlockSize.MaxGas),
		"block_size_max_txs":           aminoHasher(params.BlockSize.MaxTxs),
		"tx_size_max_bytes":            aminoHasher(params.TxSize.MaxBytes),
		"tx_size_max_gas":              aminoHasher(params.TxSize.MaxGas),
	})
}

/*
// Update returns a copy of the params with updates from the non-zero fields of p2.
// NOTE: note: must not modify the original
func (params ConsensusParams) Update(params2 *abci.ConsensusParams) ConsensusParams {
	res := params // explicit copy

	if params2 == nil {
		return res
	}

	// we must defensively consider any structs may be nil
	// XXX: it's cast city over here. It's ok because we only do int32->int
	// but still, watch it champ.
	if params2.BlockSize != nil {
		if params2.BlockSize.MaxBytes > 0 {
			res.BlockSize.MaxBytes = int(params2.BlockSize.MaxBytes)
		}
		if params2.BlockSize.MaxTxs > 0 {
			res.BlockSize.MaxTxs = int(params2.BlockSize.MaxTxs)
		}
		if params2.BlockSize.MaxGas > 0 {
			res.BlockSize.MaxGas = params2.BlockSize.MaxGas
		}
	}
	if params2.TxSize != nil {
		if params2.TxSize.MaxBytes > 0 {
			res.TxSize.MaxBytes = int(params2.TxSize.MaxBytes)
		}
		if params2.TxSize.MaxGas > 0 {
			res.TxSize.MaxGas = params2.TxSize.MaxGas
		}
	}
	if params2.BlockGossip != nil {
		if params2.BlockGossip.BlockPartSizeBytes > 0 {
			res.BlockGossip.BlockPartSizeBytes = int(params2.BlockGossip.BlockPartSizeBytes)
		}
	}
	return res
}*/

//VoteRate decides how many candidates will be voted
type VoteRate struct {
	Deno       int `json:"Deno"` //Denominator
	Nume       int `json:"Nume"` //molecule
	UpperLimit int `json:"UpperLimit"`
}

//CalRate decide the rate when calculate the candidate's rankresult
type CalRate struct {
	Srate int64 `json:"Srate"` //score rate
	Drate int64 `json:"Drate"` //deposit rate
	Rrate int64 `json:"Rrate"` //randnum rate
}

//Coefficient some coefficient which may be changed
type Coefficient struct {
	VotePeriod uint64 `json:"VotePeriod"` //voting per VotePeriod blocks
	VoteRate   `json:"VoteRate"`
	CalRate    `json:"CalRate"`
	MaxScore   int64    `json:"MaxScore"`
	UTXOFee    *big.Int `json:"UTXOFee"`
}

// DefaultCoefficient returns a default Coefficient.
func DefaultCoefficient() *Coefficient {
	return &Coefficient{
		1321,
		DefaultVoteRate(),
		DefaultCalRate(),
		500,
		big.NewInt(500000000), //0.05
	}
}

// DefaultVoteRate returns a default VoteRate.
func DefaultVoteRate() VoteRate {
	return VoteRate{
		Deno:       5,
		Nume:       3,
		UpperLimit: 12,
	}
}

// DefaultCalRate returns a default CalRate.
func DefaultCalRate() CalRate {
	return CalRate{
		Srate: 4,
		Drate: 4,
		Rrate: 2,
	}
}
