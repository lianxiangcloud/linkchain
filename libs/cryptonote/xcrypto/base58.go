package xcrypto

/*
#include "xcrypto.h"

*/
// #cgo CFLAGS: -O3 -I./
import "C"

import (
	"unsafe"
)

// Base58Encode base58 encode
func Base58Encode(data []byte) string {
	p := C.x_base58_encode((*C.char)(unsafe.Pointer(&data[0])), C.int(len(data)))
	if unsafe.Pointer(p) != C.NULL {
		addr := C.GoString(p)
		C.free(unsafe.Pointer(p))
		return addr
	}
	return ""
}

// Base58Decode base58 decode
func Base58Decode(addr string) []byte {
	var length C.int
	caddr := C.CString(addr)
	p := C.x_base58_decode(caddr, &length)
	C.free(unsafe.Pointer(caddr))
	if unsafe.Pointer(p) != C.NULL {
		data := make([]byte, length)
		var cdata []byte
		toSlice(unsafe.Pointer(p), unsafe.Pointer(&cdata), len(data))
		copy(data, cdata)
		C.free(unsafe.Pointer(p))
		return data
	}
	return nil
}

//char *x_base58_encode_addr(unsigned long long tag ,char *data, int len);
func Base58EncodeAddr(tag uint64, data string) string {
	p := C.x_base58_encode_addr((C.ulonglong)(tag), C.CString(data), C.int(len(data)))
	if unsafe.Pointer(p) != C.NULL {
		addr := C.GoString(p)
		C.free(unsafe.Pointer(p))
		return addr
	}
	return ""
}

//char *x_base58_decode_addr(unsigned long long *tag ,char *addr, int *len);
func Base58DecodeAddr(tag *uint64, addr string) []byte {
	var length C.int
	caddr := C.CString(addr)
	p := C.x_base58_decode_addr((*C.ulonglong)(tag), caddr, &length)
	C.free(unsafe.Pointer(caddr))
	if unsafe.Pointer(p) != C.NULL {
		data := make([]byte, length)
		var cdata []byte
		toSlice(unsafe.Pointer(p), unsafe.Pointer(&cdata), len(data))
		copy(data, cdata)
		C.free(unsafe.Pointer(p))
		return data
	}
	return nil
}
