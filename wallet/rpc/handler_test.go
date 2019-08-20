package rpc

import (
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/wallet/rpc/mocks"
)

var fakeCtx *Context

func TestMain(m *testing.M) {
	tx := types.UTXOTransaction{}
	wallet := &mocks.Wallet{}
	wallet.On("CreateUTXOTransaction", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return([]*types.UTXOTransaction{&tx}, nil)

	fakeCtx = &Context{
		wallet: wallet,
		logger: log.Test(),
	}

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestSignTx(t *testing.T) {
	// {"jsonrpc":"2.0","id":"0","method":"sign_tx","params":...below...}
	req := `{"subaddrs":[],"dests":[],"token":"0x0000000000000000000000000000000000000000","refundaddr":"0x0000000000000000000000000000000000000000","extra":null}`
	t.Log(req)
	rep, err := signTx(fakeCtx, []byte(req))
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(rep)
}
