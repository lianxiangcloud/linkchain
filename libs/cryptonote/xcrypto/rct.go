package xcrypto

/*
#include "xcrypto.h"
#include <stdint.h>

// signature_t
static inline int generate_signature(void *prefix, void *key_image, void *pubs, int count, void *sec, size_t index, void *sig_c, void *sig_r) {
	rct_keyV_t key_v;
	key_v.nums = count;
	key_v.v = malloc(count * sizeof(p_rct_key_t));
	memcpy(key_v.v, pubs, count * sizeof(p_rct_key_t));

	signature_t sig;
	int ret = x_generate_ring_signature(prefix, key_image, &key_v, sec, index, &sig);
	if (ret == 0) {
		memcpy(sig_c, &sig.c[0], X_HASH_SIZE);
		memcpy(sig_r, &sig.r[0], X_HASH_SIZE);
	}

	free(key_v.v);
	return ret;
}

static inline int check_signature(void *prefix, void *key_image, void *pubs, int count, void *sig_c, void *sig_r) {
	rct_keyV_t key_v;
	key_v.nums = count;
	key_v.v = malloc(count * sizeof(p_rct_key_t));
	memcpy(key_v.v, pubs, count * sizeof(p_rct_key_t));

	signature_t sig;
	memcpy(&sig.c[0], sig_c, X_HASH_SIZE);
	memcpy(&sig.r[0], sig_r, X_HASH_SIZE);

	int ret = x_check_ring_signature(prefix, key_image, &key_v, &sig);

	free(key_v.v);
	return ret;
}


// ecdhTuple
static inline int ecdh_decode(void *mask, void *amount, void *shared_key, int flag) {
	rct_ecdhTuple_t t;
	t.mask = mask;
	t.amount = amount;

	return x_ecdh_decode(&t, shared_key, flag);
}

static inline int ecdh_encode(void *mask, void *amount, void *shared_key, int flag) {
	rct_ecdhTuple_t t;
	t.mask = mask;
	t.amount = amount;

	return x_ecdh_encode(&t, shared_key, flag);
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

	ptrs := make([]uintptr, len(pks))
	for i := 0; i < len(pks); i++ {
		ptrs[i] = uintptr(unsafe.Pointer(&pks[i][0]))
	}

	var sig types.Signature
	ret := C.generate_signature(unsafe.Pointer(&prefix[0]), unsafe.Pointer(&keyImage[0]),
		unsafe.Pointer(&ptrs[0]), C.int(len(pks)), unsafe.Pointer(&sec[0]), C.size_t(secIndex),
		unsafe.Pointer(&sig.C[0]), unsafe.Pointer(&sig.R[0]))
	if ret != 0 {
		return nil, fmt.Errorf("x_generate_ring_signature")
	}
	return &sig, nil
}

// CheckRingSignature true means RingSignature ok.
func CheckRingSignature(prefix types.Hash, keyImage types.KeyImage, pks []types.PublicKey, sig *types.Signature) bool {
	if len(pks) == 0 {
		panic("pubkeys zero")
	}

	ptrs := make([]uintptr, len(pks))
	for i := 0; i < len(pks); i++ {
		ptrs[i] = uintptr(unsafe.Pointer(&pks[i][0]))
	}

	ret := C.check_signature(unsafe.Pointer(&prefix[0]), unsafe.Pointer(&keyImage[0]),
		unsafe.Pointer(&ptrs[0]), C.int(len(pks)),
		unsafe.Pointer(&sig.C[0]), unsafe.Pointer(&sig.R[0]))
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

	ret := C.ecdh_decode(unsafe.Pointer(&masked.Mask[0]), unsafe.Pointer(&masked.Amount[0]),
		unsafe.Pointer(&sharedSec[0]), C.int(flag))
	if ret < 0 {
		return false
	}
	return true
}

// EcdhEncode encode ecdhTuple info
func EcdhEncode(unmasked *types.EcdhTuple, sharedSec types.Key, shortAmount bool) bool {
	flag := 0
	if shortAmount {
		flag = 1
	}

	ret := C.ecdh_encode(unsafe.Pointer(&unmasked.Mask[0]), unsafe.Pointer(&unmasked.Amount[0]),
		unsafe.Pointer(&sharedSec[0]), C.int(flag))
	if ret < 0 {
		return false
	}
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
