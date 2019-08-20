package xcrypto

/*
#include "xcrypto.h"

*/
// #cgo CFLAGS: -O3 -I./
import "C"

import (
	"encoding/hex"
	"sync"
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
	cdata := unsafe.Pointer(&data[0])
	chash := unsafe.Pointer(&h[0])
	C.x_cn_fast_hash(cdata, C.int(len(data)), (*C.char)(chash))
	return h
}

var (
	slowHashLock sync.Mutex
)

func SlowHash(data []byte, variant int, height uint64) (h types.Hash) {
	slowHashLock.Lock()

	cdata := unsafe.Pointer(&data[0])
	chash := unsafe.Pointer(&h[0])
	C.x_cn_slow_hash(cdata, C.int(len(data)), (*C.char)(chash), C.int(variant), C.uint64_t(height))

	slowHashLock.Unlock()
	return h
}

/*
func SlowHashPreHashed(data []byte, variant int, height uint64) (h types.Hash) {
	slowHashLock.Lock()

	cdata := unsafe.Pointer(&data[0])
	chash := unsafe.Pointer(&h[0])
	C.slow_hash_prehashed(cdata, C.int(len(data)), chash, C.int(variant), C.ulonglong(height))

	slowHashLock.Unlock()
	return h
}
*/

type SlowHashState struct{}

func (s SlowHashState) Allocate() {
	slowHashLock.Lock()
	C.x_slow_hash_allocate_state()
}

func (s SlowHashState) Free() {
	C.x_slow_hash_free_state()
	slowHashLock.Unlock()
}

func (s SlowHashState) Hash(data []byte, variant int, height uint64) (h types.Hash) {
	cdata := unsafe.Pointer(&data[0])
	chash := unsafe.Pointer(&h[0])
	C.x_cn_slow_hash(cdata, C.int(len(data)), (*C.char)(chash), C.int(variant), C.uint64_t(height))
	return h
}

func (s SlowHashState) PreHashed(data []byte, variant int, height uint64) (h types.Hash) {
	cdata := unsafe.Pointer(&data[0])
	chash := unsafe.Pointer(&h[0])
	C.x_cn_slow_hash_prehashed(cdata, C.int(len(data)), (*C.char)(chash), C.int(variant), C.uint64_t(height))
	return h
}
