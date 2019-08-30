package xcrypto

/*
#include "xcrypto.h"

*/
// #cgo CFLAGS: -O3 -I./
import "C"

import (
	"encoding/hex"
	"unsafe"

	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
)

var (
	EmtpyHash, _ = hex.DecodeString("c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")
)

func FastHash(data []byte) (h types.Hash) {
	if len(data) == 0 {
		copy(h[:], EmtpyHash[:])
		return h
	}
	C.x_cn_fast_hash(unsafe.Pointer(&data[0]), C.int(len(data)), (*C.char)(unsafe.Pointer(&h[0])))
	return h
}
