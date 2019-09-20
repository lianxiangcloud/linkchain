package types

import (
	"errors"
	"fmt"
)

var (
	ErrNoConnectionToDaemon = errors.New("no_connection_to_daemon")
	// ErrDaemonBusy           = errors.New("daemon_busy")
	// ErrGetHashes            = errors.New("get_hashes_error")
	// ErrGetBlocks            = errors.New("get_blocks_error")
	ErrWalletNotOpen       = errors.New("wallet not open")
	ErrNotFoundTxKey       = errors.New("not found tx key")
	ErrTxNotFound          = errors.New("tx not found")
	ErrNoTransInTx         = errors.New("no trans in tx")
	ErrArgsInvalid         = errors.New("args invalid")
	ErrUTXONotSupportToken = errors.New("utxo not support token")
	ErrUTXODestsOverLimit  = fmt.Errorf("utxo dests over limit, should less than %d", UTXO_DESTS_MAX_NUM)
	ErrNoNeedToProof       = errors.New("account input utxo trans do not need proof")
	ErrBlockNotFound       = errors.New("block not found")
	ErrBlockParentHash     = errors.New("err block parent hash")
	ErrOutputNotFound      = errors.New("output not found")
	ErrSubAccountTooLarge  = errors.New("subaccount is too large")
	ErrSaveAccountSubCnt   = errors.New("save AccountSubCnt fail")
	ErrAddInfoNotFound     = errors.New("utxo add info not found")
)
