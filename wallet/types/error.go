package types

import (
	"fmt"
)

type WErr struct {
	code int
	msg  string
}

func NewWErr(code int, msg string) *WErr {
	return &WErr{
		code: code,
		msg:  msg,
	}
}

func (e *WErr) Error() string {
	return e.msg
}

func (e *WErr) ErrorCode() int {
	return e.code
}

var (
	ErrArgsInvalid         = NewWErr(-600001, "args invalid")
	ErrUTXONotSupportToken = NewWErr(-600002, "utxo not support token")
	ErrPasswdEmpty         = NewWErr(-600003, "password empty")
	ErrCueTooLong          = NewWErr(-600004, "cue too long")
	ErrUTXODestsOverLimit  = NewWErr(-600005, fmt.Sprintf("utxo dests over limit, should less than %d", UTXO_DESTS_MAX_NUM))
	ErrAccDestsOverLimit   = NewWErr(-600006, "account output too more")
	ErrTxTypeNotSupport    = NewWErr(-600007, "not support tx type")
	ErrNoNeedToProof       = NewWErr(-600008, "account input utxo trans do not need proof")
	ErrNoTransInTx         = NewWErr(-600009, "no trans in tx")
	ErrWalletNotOpen       = NewWErr(-600010, "wallet not open")
	ErrAccountNeedUnlock   = NewWErr(-600011, "account need unlock")

	ErrAccountNotFound      = NewWErr(-601001, "account not found")
	ErrNewAccount           = NewWErr(-601002, "new account fail")
	ErrStrToAddressInvalid  = NewWErr(-601003, "str to address invalid")
	ErrStrToAddressCheckSum = NewWErr(-601004, "str to address check sum fail")
	ErrAccBalanceUpdate     = NewWErr(-601005, "update balance fail")
	ErrSubaddrIdxOverRange  = NewWErr(-601006, "subaddr index over range")
	ErrSubAccountOverLimit  = NewWErr(-601007, "subaccount over limit")

	ErrTransNeedSplit       = NewWErr(-602001, "Transaction would be too large.  try transfer_split")
	ErrBlockParentHash      = NewWErr(-602002, "err block parent hash")
	ErrOutputQueryNotMatch  = NewWErr(-602003, "output query not match")
	ErrOutputEmpty          = NewWErr(-602004, "output empty")
	ErrOutputMoneyInvalid   = NewWErr(-602005, "output money invalid")
	ErrOutputMoneyOverFlow  = NewWErr(-602006, "sum output money over flow")
	ErrBalanceNotEnough     = NewWErr(-602007, "balance not enough")
	ErrSignTx               = NewWErr(-602008, "sign tx err")
	ErrNewAinTrans          = NewWErr(-602009, "new account input transaction err")
	ErrNoMoreOutput         = NewWErr(-602010, "no more output")
	ErrTxTooBig             = NewWErr(-602011, "tx too big")
	ErrNewUinTrans          = NewWErr(-602012, "new utxo input transaction err")
	ErrUinTransWithSign     = NewWErr(-602013, "utxo transaction with sign err")
	ErrNotSupportContractTx = NewWErr(-602014, "not support contract tx")
	ErrMixInputNotSupport   = NewWErr(-602015, "mix input not support")
	ErrTransInvalid         = NewWErr(-602016, "transaction output amount invalid")

	ErrNoConnectionToDaemon = NewWErr(-603001, "no_connection_to_daemon")
	ErrDaemonResponseBody   = NewWErr(-603002, "dameon response body err")
	ErrDaemonResponseCode   = NewWErr(-603003, "dameon response code err")
	ErrDaemonResponseData   = NewWErr(-603004, "dameon response data err")
	ErrSubmitTrans          = NewWErr(-603005, "submit transaction fail")

	ErrSaveAccountSubCnt = NewWErr(-604001, "save AccountSubCnt fail")
	ErrNotFoundTxKey     = NewWErr(-604002, "not found tx key")
	ErrBlockNotFound     = NewWErr(-604003, "block not found")
	ErrOutputNotFound    = NewWErr(-604004, "output not found")
	ErrBatchSave         = NewWErr(-604005, "batch save fail")
	ErrBatchCommit       = NewWErr(-604006, "batch commit fail")
	ErrSaveTxKey         = NewWErr(-604007, "tx key save fail")
	ErrTxNotFound        = NewWErr(-604008, "tx not found")
	ErrUTXOTxCommit      = NewWErr(-604009, "utxo tx commit fail")
	ErrAddInfoNotFound   = NewWErr(-604010, "utxo add info not found")
	ErrTxAddInfoCommit   = NewWErr(-604011, "tx add info commit fail")
	ErrTxAddInfoDel      = NewWErr(-604012, "tx add info del fail")

	ErrInnerServer = NewWErr(-605001, "server inner error")
)
