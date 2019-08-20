// Copyright 2014 The go-ethereum Authors
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

package types

import (
	"unsafe"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto/merkle"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

//go:generate gencodec -type Receipt -field-override receiptMarshaling -out gen_receipt_json.go

var (
	receiptStatusFailedRLP     = []byte{}
	receiptStatusSuccessfulRLP = []byte{0x01}
)

const (
	// ReceiptStatusFailed is the status code of a transaction if execution failed.
	ReceiptStatusFailed = uint64(0)

	// ReceiptStatusSuccessful is the status code of a transaction if execution succeeded.
	ReceiptStatusSuccessful = uint64(1)
)

// Receipt represents the results of a transaction.
type Receipt struct {
	// Consensus fields
	PostState         []byte `json:"root"`
	Status            uint64 `json:"status"`
	VMErr             string `json:"vmErr"`
	CumulativeGasUsed uint64 `json:"cumulativeGasUsed" gencodec:"required"`
	Bloom             Bloom  `json:"logsBloom"         gencodec:"required"`
	Logs              []*Log `json:"logs"              gencodec:"required"`

	// Implementation fields (don't reorder!)
	TxHash          common.Hash    `json:"transactionHash" gencodec:"required"`
	ContractAddress common.Address `json:"contractAddress"`
	GasUsed         uint64         `json:"gasUsed" gencodec:"required"`
}

type ReceiptForStorage struct {
	PostState         []byte
	Status            uint64
	VMErr             string
	CumulativeGasUsed uint64
	Bloom             Bloom
	Logs              []*LogForStorage

	TxHash          common.Hash
	ContractAddress common.Address
	GasUsed         uint64
}

func (r *Receipt) ForStorage() *ReceiptForStorage {
	s := ReceiptForStorage{
		PostState:         r.PostState,
		Status:            r.Status,
		VMErr:             r.VMErr,
		CumulativeGasUsed: r.CumulativeGasUsed,
		Bloom:             r.Bloom,
		Logs:              make([]*LogForStorage, len(r.Logs)),
		TxHash:            r.TxHash,
		ContractAddress:   r.ContractAddress,
		GasUsed:           r.GasUsed,
	}

	for index, log := range r.Logs {
		s.Logs[index] = (*LogForStorage)(log)
	}
	return &s
}

func (s *ReceiptForStorage) ToReceipt() *Receipt {
	r := Receipt{
		PostState:         s.PostState,
		Status:            s.Status,
		VMErr:             s.VMErr,
		CumulativeGasUsed: s.CumulativeGasUsed,
		Bloom:             s.Bloom,
		Logs:              make([]*Log, len(s.Logs)),
		TxHash:            s.TxHash,
		ContractAddress:   s.ContractAddress,
		GasUsed:           s.GasUsed,
	}

	for index, log := range s.Logs {
		r.Logs[index] = (*Log)(log)
	}
	return &r
}

// NewReceipt creates a barebone transaction receipt, copying the init fields.
func NewReceipt(root []byte, vmerr error, cumulativeGasUsed uint64) *Receipt {
	r := &Receipt{PostState: common.CopyBytes(root), CumulativeGasUsed: cumulativeGasUsed}
	if vmerr != nil {
		r.Status = ReceiptStatusFailed
		r.VMErr = vmerr.Error()
	} else {
		r.Status = ReceiptStatusSuccessful
	}
	return r
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (r *Receipt) Size() common.StorageSize {
	size := common.StorageSize(unsafe.Sizeof(*r)) + common.StorageSize(len(r.PostState))

	size += common.StorageSize(len(r.Logs)) * common.StorageSize(unsafe.Sizeof(Log{}))
	for _, log := range r.Logs {
		size += common.StorageSize(len(log.Topics)*common.HashLength + len(log.Data))
	}
	return size
}

func (r *Receipt) Hash() common.Hash {
	return rlpHash(r)
}

// Receipts is a wrapper around a Receipt array to implement DerivableList.
type Receipts []*Receipt

// Len returns the number of receipts in this list.

func (r Receipts) Len() int { return len(r) }

// GetRlp returns the RLP encoding of one receipt from the list.
func (r Receipts) GetRlp(i int) []byte {
	bytes, err := ser.EncodeToBytes(r[i])
	if err != nil {
		panic(err)
	}
	return bytes
}

func (r Receipts) Hash() common.Hash {
	switch len(r) {
	case 0:
		return common.EmptyHash
	case 1:
		return r[0].Hash()
	default:
		left := Receipts(r[:(len(r)+1)/2]).Hash().Bytes()
		right := Receipts(r[(len(r)+1)/2:]).Hash().Bytes()
		hash := merkle.SimpleHashFromTwoHashes(left, right)
		return common.BytesToHash(hash)
	}
}
