package xcrypto

/*
#include "xcrypto.h"
#include <stdint.h>

#define MALLOC(tp) \
	static inline tp *malloc_##tp() {\
		return (tp *)malloc(sizeof(tp));\
	}

#define SETN_VECTOR(tp, field) \
	static inline void setn_##tp(tp *vec, void *p, int nums) {\
		vec->field = p; \
		vec->nums = nums; \
	}

#define INIT_VECTOR(tp, field) \
	static inline void init_##tp(tp *vec, int nums) {\
		if (nums > 0) {\
			vec->field = malloc(sizeof(vec->field) * nums); \
		} \
		vec->nums = nums; \
	}

#define SETP_VECTOR(tp, field) \
	static inline void setp_##tp(tp *vec, void *p) {\
		memcpy(vec->field, p, sizeof(vec->field) * (vec->nums)); \
	}

#define FREE_VECTOR(tp, field) \
	static inline void free_rct_keyV_t(tp *vec) {\
		if (vec->nums > 0) {\
			free(vec->field);\
		}\
	}

// rct_keyV_t
SETN_VECTOR(rct_keyV_t, v)
INIT_VECTOR(rct_keyV_t, v)
SETP_VECTOR(rct_keyV_t, v)
FREE_VECTOR(rct_keyV_t, v)


// signature_t
static inline void fill_signature(signature_t *sig, void *c, void *r) {
	memcpy(&sig->c[0], c, X_HASH_SIZE);
	memcpy(&sig->r[0], r, X_HASH_SIZE);
}
static inline void extract_signature(signature_t *sig, void *c, void *r) {
	memcpy(c, &sig->c[0], X_HASH_SIZE);
	memcpy(r, &sig->r[0], X_HASH_SIZE);
}

*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
)

// ---------------------------------------------------------------------------------

//GenerateRingSignature --
func GenerateRingSignature(prefix types.Hash, keyImage types.KeyImage, pks []types.PublicKey, sec types.SecretKey, secIndex uint) (*types.Signature, error) {
	if len(pks) == 0 {
		return nil, fmt.Errorf("pubkeys zero")
	}

	cKeyV := C.struct_rct_keyV{}
	C.init_rct_keyV_t(&cKeyV, C.int(len(pks)))

	ptrs := make([]uintptr, len(pks))
	for i := 0; i < len(pks); i++ {
		ptrs[i] = uintptr(unsafe.Pointer(&pks[i][0]))
	}
	C.setp_rct_keyV_t(&cKeyV, unsafe.Pointer(&ptrs[0]))

	cSig := C.struct_signature{}
	ret := C.x_generate_ring_signature((*C.char)(unsafe.Pointer(&prefix[0])), (*C.char)(unsafe.Pointer(&keyImage[0])), &cKeyV,
		(C.p_rct_key_t)(unsafe.Pointer(&sec[0])), C.size_t(secIndex), &cSig)
	C.free_rct_keyV_t(&cKeyV)

	if ret != 0 {
		return nil, fmt.Errorf("x_generate_ring_signature")
	}

	var sig types.Signature
	C.extract_signature(&cSig, unsafe.Pointer(&sig.C[0]), unsafe.Pointer(&sig.R[0]))
	return &sig, nil
}

// CheckRingSignature true means RingSignature ok.
func CheckRingSignature(prefix types.Hash, keyImage types.KeyImage, pks []types.PublicKey, sig *types.Signature) bool {
	if len(pks) == 0 {
		panic("pubkeys zero")
	}

	cKeyV := C.struct_rct_keyV{}
	C.init_rct_keyV_t(&cKeyV, C.int(len(pks)))

	ptrs := make([]uintptr, len(pks))
	for i := 0; i < len(pks); i++ {
		ptrs[i] = uintptr(unsafe.Pointer(&pks[i][0]))
	}
	C.setp_rct_keyV_t(&cKeyV, unsafe.Pointer(&ptrs[0]))

	cSig := C.struct_signature{}
	C.fill_signature(&cSig, unsafe.Pointer(&sig.C[0]), unsafe.Pointer(&sig.R[0]))

	ret := C.x_check_ring_signature((*C.char)(unsafe.Pointer(&prefix[0])), (*C.char)(unsafe.Pointer(&keyImage[0])), &cKeyV, &cSig)
	if ret == 0 {
		return true
	}

	return false
}

func ScalarmultKey(p, a types.Key) (ret types.Key, err error) {
	_, err = C.x_scalarmultKey((*C.char)(unsafe.Pointer(&ret[0])), (*C.char)(unsafe.Pointer(&p[0])), (*C.char)(unsafe.Pointer(&a[0])))
	return ret, err
}

func ScalarmultBase(a types.Key) (ret types.Key) {
	C.x_scalarmultBase((*C.char)(unsafe.Pointer(&ret[0])), (*C.char)(unsafe.Pointer(&a[0])))
	return ret
}

func SkpkGen() (sk types.Key, pk types.Key) {
	C.x_skpkGen((*C.char)(unsafe.Pointer(&sk[0])), (*C.char)(unsafe.Pointer(&pk[0])))
	return sk, pk
}

func ScalarmultH(a types.Key) (ret types.Key) {
	C.x_scalarmultH((*C.char)(unsafe.Pointer(&ret[0])), (*C.char)(unsafe.Pointer(&a[0])))
	return ret
}
func ZeroCommit(amount types.Lk_amount) (ret types.Key, err error) {
	_, err = C.x_zeroCommit((*C.char)(unsafe.Pointer(&ret[0])), (C.ulonglong)(amount))
	return ret, err
}
func CheckKey(key types.PublicKey) bool {
	ret := C.x_checkKey((*C.char)(unsafe.Pointer(&key[0])))
	if ret == 0 {
		return true
	}
	return false
}

// EcdhDecode decode ecdhTuple info
func EcdhDecode(masked *types.EcdhTuple, sharedSec types.Key, shortAmount bool) bool {
	flag := 0
	if shortAmount {
		flag = 1
	}
	cMasked := C.malloc(C.sizeof_rct_ecdhTuple_t)
	defer C.free(unsafe.Pointer(cMasked))

	toRctEcdhTuple(masked, (*C.rct_ecdhTuple_t)(cMasked))
	ret := C.x_ecdh_decode((*C.rct_ecdhTuple_t)(cMasked), C.p_rct_key_t(unsafe.Pointer(&sharedSec[0])), C.int(flag))

	if ret < 0 {
		return false
	}

	fromRctEcdhTuple((*C.rct_ecdhTuple_t)(cMasked), masked)
	return true
}

// EcdhEncode encode ecdhTuple info
func EcdhEncode(unmasked *types.EcdhTuple, sharedSec types.Key, shortAmount bool) bool {
	flag := 0
	if shortAmount {
		flag = 1
	}
	cUnmasked := C.malloc(C.sizeof_rct_ecdhTuple_t)
	defer C.free(unsafe.Pointer(cUnmasked))

	toRctEcdhTuple(unmasked, (*C.rct_ecdhTuple_t)(cUnmasked))
	ret := C.x_ecdh_encode((*C.rct_ecdhTuple_t)(unsafe.Pointer(cUnmasked)), C.p_rct_key_t(unsafe.Pointer(&sharedSec[0])), C.int(flag))

	if ret < 0 {
		return false
	}

	fromRctEcdhTuple((*C.rct_ecdhTuple_t)(cUnmasked), unmasked)
	return true
}

//Computes 8P
// x_scalarmult8(rct_key_t p ,rct_key_t ret);
func Scalarmult8(p types.Key) (ret types.Key, err error) {
	_, err = C.x_scalarmult8((*C.char)(unsafe.Pointer(&p[0])), (*C.char)(unsafe.Pointer(&ret[0])))
	return ret, err
}

//void x_sc_add(ec_scalar_t s, ec_scalar_t a, ec_scalar_t b);
func ScAdd(a, b types.EcScalar) (ret types.Key) {
	C.x_sc_add((*C.char)(unsafe.Pointer(&ret[0])), (*C.char)(unsafe.Pointer(&a[0])), (*C.char)(unsafe.Pointer(&b[0])))
	return ret
}

//void x_sc_sub(ec_scalar_t s, ec_scalar_t a, ec_scalar_t b);
func ScSub(a, b types.EcScalar) (ret types.Key) {
	C.x_sc_sub((*C.char)(unsafe.Pointer(&ret[0])), (*C.char)(unsafe.Pointer(&a[0])), (*C.char)(unsafe.Pointer(&b[0])))
	return ret
}

//void x_skGen(rct_key_t key);
func SkGen() (ret types.Key) {
	C.x_skGen((*C.char)(unsafe.Pointer(&ret[0])))
	return ret
}

//void x_genC(rct_key_t c,rct_key_t a,unsigned long long amount);
func GenC(a types.Key, amount types.Lk_amount) (ret types.Key, err error) {
	_, err = C.x_genC((*C.char)(unsafe.Pointer(&ret[0])), (*C.char)(unsafe.Pointer(&a[0])), (C.ulonglong)(amount))
	return ret, err
}
func AddKeys(a, b types.Key) (ret types.Key, err error) {
	_, err = C.x_addKeys((*C.char)(unsafe.Pointer(&ret[0])), (*C.char)(unsafe.Pointer(&a[0])), (*C.char)(unsafe.Pointer(&b[0])))
	return ret, err
}
func AddKeys2(a, b, B types.Key) (ret types.Key, err error) {
	_, err = C.x_addKeys2((*C.char)(unsafe.Pointer(&ret[0])), (*C.char)(unsafe.Pointer(&a[0])), (*C.char)(unsafe.Pointer(&b[0])), (*C.char)(unsafe.Pointer(&B[0])))
	return ret, err
}

// ---------------------------------------------------------------------------------

// func toSlice(cPtr, goPtr unsafe.Pointer, n int) {
// 	sh := (*reflect.SliceHeader)(goPtr)
// 	sh.Cap = n
// 	sh.Len = n
// 	sh.Data = uintptr(cPtr)
// }

func toRctEcdhTuple(from *types.EcdhTuple, to *C.struct_rct_ecdhTuple) {
	to.mask = C.p_rct_key_t(unsafe.Pointer(&from.Mask[0]))
	to.amount = C.p_rct_key_t(unsafe.Pointer(&from.Amount[0]))
}

func fromRctEcdhTuple(from *C.struct_rct_ecdhTuple, to *types.EcdhTuple) {
	copy(to.Mask[:], C.GoBytes(unsafe.Pointer(from.mask), C.int(types.COMMONLEN)))
	copy(to.Amount[:], C.GoBytes(unsafe.Pointer(from.amount), C.int(types.COMMONLEN)))
}
