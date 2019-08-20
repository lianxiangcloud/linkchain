package main

import (
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/types"
)

type MockChainContext struct {
	// GetHeader returns the hash corresponding to their hash.
}

func (m *MockChainContext) GetHeader(uint64) *types.Header {
	h := types.Header{}
	return &h
}
type MockAccountRef common.Address

// Address casts AccountRef to a Address
func (ar MockAccountRef) Address() common.Address { return (common.Address)(ar) }
