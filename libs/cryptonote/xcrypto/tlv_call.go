package xcrypto

/*
#include "xcrypto.h"
#include <stdint.h>

*/
import "C"

import (
	"encoding/hex"
	"fmt"
	"unsafe"

	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
)

func TlvVerRctNotSemanticsSimple(rctsign *types.RctSig) bool {
	data := make([]byte, rctsign.TlvSize())
	_, err := rctsign.TlvEncode(data)
	if err != nil {
		panic(fmt.Sprintf("[TlvEncode fail] %s", err))
		// return false
	}
	ret := C.tlv_verRctNotSemanticsSimple((*C.uchar)(unsafe.Pointer(&data[0])), (C.int)(len(data)))
	if ret < 0 {
		return false
	}
	return true
}

func TlvVerRctSimple(rctsign *types.RctSig) (error, bool) {
	data := make([]byte, rctsign.TlvSize())
	_, err := rctsign.TlvEncode(data)
	if err != nil {
		return err, false
	}
	ret := C.tlv_verRctSimple((*C.uchar)(unsafe.Pointer(&data[0])), (C.int)(len(data)))
	if ret == -1 {
		return fmt.Errorf("cgo TlvRctSign internal fail"), false
	}
	if ret == 0 {
		return nil, false
	}
	return nil, true
}
func TlvProveRangeBulletproof(amounts types.KeyV, sk types.KeyV) (b *types.Bulletproof, c types.KeyV, masks types.KeyV, err error) {
	tms := types.NewTlvMapSerializerWith(&amounts, &sk)
	inrawSize := tms.TlvSize()
	inraw := make([]byte, inrawSize)
	_, err = tms.TlvEncode(inraw)
	if err != nil {
		return nil, nil, nil, err
	}
	//fmt.Printf("%v\n",hex.EncodeToString(inraw))
	var backp *C.uchar
	ret, err := C.tlv_proveRangeBulletproof((*C.uchar)(unsafe.Pointer(&inraw[0])), (C.int)(inrawSize), (**C.uchar)(unsafe.Pointer(&backp)))
	if err != nil {
		return nil, nil, nil, err
	}
	if ret == -1 {
		return nil, nil, nil, fmt.Errorf("tlv_proveRangeBulletproof internal error")
	}
	defer C.free((unsafe.Pointer(backp)))
	var retslice []byte
	toSlice(unsafe.Pointer(backp), unsafe.Pointer(&retslice), int(ret))
	//fmt.Printf("reslice %d= %v\n",ret,hex.EncodeToString(retslice))
	bulletproof := types.Bulletproof{}
	tms = types.NewTlvMapSerializerWith(&c, &masks, &bulletproof)
	if err := tms.TlvDecode(retslice); err != nil {
		return nil, nil, nil, err
	}
	return &bulletproof, c, masks, nil
}

func TlvProveRangeBulletproof128(amounts types.KeyV, sk types.KeyV) (b *types.Bulletproof, c types.KeyV, masks types.KeyV, err error) {
	tms := types.NewTlvMapSerializerWith(&amounts, &sk)
	inrawSize := tms.TlvSize()
	inraw := make([]byte, inrawSize)
	_, err = tms.TlvEncode(inraw)
	if err != nil {
		return nil, nil, nil, err
	}
	//fmt.Printf("%v\n",hex.EncodeToString(inraw))
	var backp *C.uchar
	ret, err := C.tlv_proveRangeBulletproof128((*C.uchar)(unsafe.Pointer(&inraw[0])), (C.int)(inrawSize), (**C.uchar)(unsafe.Pointer(&backp)))
	if err != nil {
		return nil, nil, nil, err
	}
	if ret == -1 {
		return nil, nil, nil, fmt.Errorf("tlv_proveRangeBulletproof internal error")
	}
	defer C.free((unsafe.Pointer(backp)))
	var retslice []byte
	toSlice(unsafe.Pointer(backp), unsafe.Pointer(&retslice), int(ret))
	//fmt.Printf("reslice %d= %v\n",ret,hex.EncodeToString(retslice))
	bulletproof := types.Bulletproof{}
	tms = types.NewTlvMapSerializerWith(&c, &masks, &bulletproof)
	if err := tms.TlvDecode(retslice); err != nil {
		return nil, nil, nil, err
	}
	return &bulletproof, c, masks, nil
}

func TlvProveRctMGSimple(message types.Key, pubs types.CtkeyV, inSk types.Ctkey, a, Count types.Key, mscout *types.Key, kLRki *types.MultisigKLRki, index uint32) (sig *types.MgSig, err error) {
	tms := types.NewTlvMapSerializerWith(&message, &pubs, &inSk, &a, &Count)
	if kLRki != nil {
		tms.SetTagAndSerializer(6, kLRki)
	}
	inrawSize := tms.TlvSize()
	inraw := make([]byte, inrawSize)
	_, err = tms.TlvEncode(inraw)
	if err != nil {
		return nil, err
	}
	var backp *C.uchar
	var ret C.int
	if mscout != nil {
		ret, err = C.tlv_proveRctMGSimple((*C.char)(unsafe.Pointer(&(mscout[0]))), (C.uint)(index), (*C.uchar)(unsafe.Pointer(&inraw[0])), (C.int)(inrawSize), (**C.uchar)(unsafe.Pointer(&backp)))
	} else {
		ret, err = C.tlv_proveRctMGSimple((*C.char)(unsafe.Pointer(nil)), (C.uint)(index), (*C.uchar)(unsafe.Pointer(&inraw[0])), (C.int)(inrawSize), (**C.uchar)(unsafe.Pointer(&backp)))
	}
	if err != nil {
		return nil, err
	}
	if ret == -1 {
		return nil, fmt.Errorf("tlv_proveRangeBulletproof internal error")
	}
	defer C.free((unsafe.Pointer(backp)))
	var retslice []byte
	toSlice(unsafe.Pointer(backp), unsafe.Pointer(&retslice), int(ret))
	mgsig := types.MgSig{}
	if err := mgsig.TlvDecode(retslice); err != nil {
		return nil, err
	}
	return &mgsig, nil
}

func TlvGetPreMlsagHash(rctsign *types.RctSig) (key types.Key, err error) {
	data := make([]byte, rctsign.TlvSize())
	_, err = rctsign.TlvEncode(data)
	if err != nil {
		return key, err
	}
	ret, err := C.tlv_get_pre_mlsag_hash((*C.char)(unsafe.Pointer(&key[0])), (*C.uchar)(unsafe.Pointer(&data[0])), (C.int)(len(data)))
	if err != nil {
		return key, err
	}
	if ret == -1 {
		return key, fmt.Errorf("tlv_get_pre_mlsag_hash internal error")
	}
	return key, nil
}

func TlvAddKeyV(a types.KeyV) (sum types.Key, err error) {
	data := make([]byte, a.TlvSize())
	_, err = a.TlvEncode(data)
	if err != nil {
		return sum, err
	}
	ret, err := C.tlv_addKeyV((*C.char)(unsafe.Pointer(&sum[0])), (*C.uchar)(unsafe.Pointer(&data[0])), (C.int)(len(data)))
	if err != nil {
		return sum, err
	}
	if ret == -1 {
		return sum, err
	}
	return sum, nil
}
func TlvVerBulletproof(bp *types.Bulletproof) (bool, error) {
	data := make([]byte, bp.TlvSize())
	_, err := bp.TlvEncode(data)
	if err != nil {
		return false, err
	}
	ret := C.tlv_verBulletproof((*C.uchar)(unsafe.Pointer(&data[0])), (C.int)(len(data)))
	if ret == -1 {
		return false, fmt.Errorf("cgo TlvVerBulletproof internal fail")
	}
	if ret == 0 {
		return false, nil
	}
	return true, nil
}

func TlvVerBulletproof128(bp *types.Bulletproof) (bool, error) {
	data := make([]byte, bp.TlvSize())
	_, err := bp.TlvEncode(data)
	if err != nil {
		return false, err
	}
	ret := C.tlv_verBulletproof128((*C.uchar)(unsafe.Pointer(&data[0])), (C.int)(len(data)))
	if ret == -1 {
		return false, fmt.Errorf("cgo TlvVerBulletproof internal fail")
	}
	if ret == 0 {
		return false, nil
	}
	return true, nil
}

func TlvGetSubaddress(keys *types.AccountKey, index uint32) (addr types.AccountAddress, err error) {
	data := make([]byte, keys.TlvSize())
	_, err = keys.TlvEncode(data)
	if err != nil {
		return addr, err
	}
	var backp *C.uchar
	ret, err := C.tlv_get_subaddress(C.uint32_t(index), (*C.uchar)(unsafe.Pointer(&data[0])), (C.int)(len(data)), (**C.uchar)(unsafe.Pointer(&backp)))
	if err != nil {
		return addr, err
	}
	if ret == -1 {
		return addr, fmt.Errorf("cgo TlvVerBulletproof internal fail")
	}

	defer C.free((unsafe.Pointer(backp)))
	var retslice []byte
	toSlice(unsafe.Pointer(backp), unsafe.Pointer(&retslice), int(ret))
	if err := addr.TlvDecode(retslice); err != nil {
		return addr, err
	}
	return addr, nil
}

// for test---------------------------------------------------------------------------------

func TlvKeyVTest(keysie int) error {
	cKeyV := make(types.KeyV, keysie)
	for i := 0; i < keysie; i++ {
		cKeyV[i] = types.Key{121, 201, 148, 20, 165, 225, 8, 37, 186, 117, 239, 0, 3, 148, 76, 241, 86, 55, 38, 123, 182, 35, 115, 126, 76, 56, 186, 191, 23, 80, 177, 49}
	}
	data := make([]byte, cKeyV.TlvSize())
	_, err := cKeyV.TlvEncode(data)
	if err != nil {
		return err
	}
	var backp *C.uchar
	ret := C.test_tlv_keyV((*C.uchar)(unsafe.Pointer(&data[0])), (C.int)(len(data)), (**C.uchar)(unsafe.Pointer(&backp)))
	if ret == -1 {
		return fmt.Errorf("cgo test_tlv_keyV fail")
	}
	defer C.free((unsafe.Pointer(backp)))
	retslice := C.GoBytes(unsafe.Pointer(backp), C.int(ret))

	//fmt.Printf("ret=%v\n", retslice)
	to := types.KeyV{}
	to.TlvDecode(retslice)
	if !to.IsEqual(&cKeyV) {
		return fmt.Errorf("not equal")
	}
	return nil
}
func TlvRctSign(rctsign *types.RctSig) error {
	data := make([]byte, rctsign.TlvSize())
	_, err := rctsign.TlvEncode(data)
	if err != nil {
		return err
	}
	ret := C.tlv_verRctSimple((*C.uchar)(unsafe.Pointer(&data[0])), (C.int)(len(data)))
	if ret == -1 {
		return fmt.Errorf("cgo TlvRctSign fail")
	}
	return nil
}

func TlvRctsigForTest(rctsign *types.RctSig) (*types.RctSig, error) {
	data := make([]byte, rctsign.TlvSize())
	_, err := rctsign.TlvEncode(data)
	if err != nil {
		return nil, err
	}
	var backp *C.uchar
	ret := C.test_tlv_rctsig((*C.uchar)(unsafe.Pointer(&data[0])), (C.int)(len(data)), (**C.uchar)(unsafe.Pointer(&backp)))
	if ret == -1 {
		fmt.Printf("%v len=%v\n", hex.EncodeToString(data), len(data))
		return nil, fmt.Errorf("cgo test_tlv_rctsig fail")
	}
	defer C.free((unsafe.Pointer(backp)))
	var retslice []byte
	toSlice(unsafe.Pointer(backp), unsafe.Pointer(&retslice), int(ret))
	to := &types.RctSig{}
	to.TlvDecode(retslice)

	return to, nil
}
