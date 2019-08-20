package types

import (
	"github.com/lianxiangcloud/linkchain/libs/common"
)

// TxEntry is a positional metadata to help looking up the data content of
// a transaction or receipt given only its hash.
type TxEntry struct {
	BlockHash   common.Hash `json:"blockHash"`
	BlockHeight uint64      `json:"blockHeight"`
	Index       uint64      `json:"txIndex"`
}
