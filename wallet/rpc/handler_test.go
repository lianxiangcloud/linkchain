package rpc

import (
	"fmt"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/stretchr/testify/assert"
)

// var fakeCtx *Context

func TestSelectAddress(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	assert := assert.New(t)

	mockWallet := NewMockWallet(mockCtl)

	mockBackend := NewMockBackend(mockCtl)
	mockBackend.EXPECT().GetWallet().Return(mockWallet)

	s := NewPublicTransactionPoolAPI(mockBackend, nil)

	emptyAddr := common.EmptyAddress
	mockWallet.EXPECT().SelectAddress(emptyAddr).Return(nil)
	ok, _ := s.SelectAddress(nil, emptyAddr)
	assert.Equal(true, ok, "not equal")

	oneAddr := common.HexToAddress("0xa73810e519e1075010678d706533486d8ecc8000")
	mockWallet.EXPECT().SelectAddress(oneAddr).Return(fmt.Errorf("SelectAddress fail"))
	ok, _ = s.SelectAddress(nil, oneAddr)
	assert.Equal(false, ok, "not equal")
}
