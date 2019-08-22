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

// rct_ctkeyV_t
SETN_VECTOR(rct_ctkeyV_t, v)

// signature_t
static inline void fill_signature(signature_t *sig, void *c, void *r) {
	memcpy(&sig->c[0], c, X_HASH_SIZE);
	memcpy(&sig->r[0], r, X_HASH_SIZE);
}
static inline void extract_signature(signature_t *sig, void *c, void *r) {
	memcpy(c, &sig->c[0], X_HASH_SIZE);
	memcpy(r, &sig->r[0], X_HASH_SIZE);
}

// rct_keyM_t
SETN_VECTOR(rct_keyM_t, m)

// rct_ctkeyM_t
SETN_VECTOR(rct_ctkeyM_t, m)

// rct_ecdhTupleV_t
SETN_VECTOR(rct_ecdhTupleV_t, v)


// rct_key64_t
static inline void set_rct_key64_t(rct_key64_t *self, void *v) {
	memcpy(self, v, sizeof(p_rct_key_t)*64);
}
static inline void get_rct_key64_t(rct_key64_t *self, void *v) {
	memcpy(v, self, sizeof(p_rct_key_t)*64);
}

// rct_rangeSigV_t
SETN_VECTOR(rct_rangeSigV_t, v)

// rct_bulletproofV_t
SETN_VECTOR(rct_bulletproofV_t, v)

// rct_mgSigV_t
SETN_VECTOR(rct_mgSigV_t, v)

// amountV_t
SETN_VECTOR(amountV_t, v)

// indexV_t
SETN_VECTOR(indexV_t, v)


// rct_multisig_kLRkiV_t
SETN_VECTOR(rct_multisig_kLRkiV_t, v)


extern void GoFromRctSig(rct_sig_t *p);
static int x_genSig_callback(void *self) {
	GoFromRctSig((rct_sig_t *)self);
	return 0;
}
static inline void init_rct_sig_callback(rct_sig_t *self) {
	self->cb = x_genSig_callback;
}
MALLOC(rct_sig_t)

// -------------------------------------------------------------------

// just for test
static inline void xx_genRct(rct_key_t message, rct_ctkeyV_t *inSk, rct_keyV_t *destinations, amountV_t *amounts, rct_ctkeyM_t *mixRing, rct_keyV_t *amount_keys,
	 rct_multisig_kLRki_t *kLRki, rct_multisig_out_t *msout, unsigned int index, rct_ctkeyV_t *outSk, RCTConfig_t *rct_config,  rct_sig_t *rctSig) {
		return;
}

extern void GoFromRctKey(test_rct_key_t *p);
static void test_rct_key_callback(void *self) {
	GoFromRctKey((test_rct_key_t *)self);
}
static inline void init_test_rct_key_callback(test_rct_key_t *self) {
	self->cb = test_rct_key_callback;
}


extern void GoFromRctKeyV(test_rct_keyV_t *p);
static void test_rct_keyV_callback(void *self) {
	GoFromRctKeyV((test_rct_keyV_t *)self);
}
static inline void init_test_rct_keyV_callback(test_rct_keyV_t *self) {
	self->cb = test_rct_keyV_callback;
}


extern void GoFromRctKeyM(test_rct_keyM_t *p);
static void test_rct_keyM_callback(void *self) {
	GoFromRctKeyM((test_rct_keyM_t *)self);
}
static inline void init_test_rct_keyM_callback(test_rct_keyM_t *self) {
	self->cb = test_rct_keyM_callback;
}

extern void GoFromRctCtKeyV(test_rct_ctkeyV_t *p);
static void test_rct_ctkeyV_callback(void *self) {
	GoFromRctCtKeyV((test_rct_ctkeyV_t *)self);
}
static inline void init_test_rct_ctkeyV_callback(test_rct_ctkeyV_t *self) {
	self->cb = test_rct_ctkeyV_callback;
}

extern void GoFromRctCtKeyM(test_rct_ctkeyM_t *p);
static void test_rct_ctkeyM_callback(void *self) {
	GoFromRctCtKeyM((test_rct_ctkeyM_t *)self);
}
static inline void init_test_rct_ctkeyM_callback(test_rct_ctkeyM_t *self) {
	self->cb = test_rct_ctkeyM_callback;
}

extern void GoFromRctSigbase(test_rct_sigbase_t *p);
static void test_rct_sigbase_callback(void *self) {
	GoFromRctSigbase((test_rct_sigbase_t *)self);
}
static inline void init_test_rct_sigbase_callback(test_rct_sigbase_t *self) {
	self->cb = test_rct_sigbase_callback;
}

MALLOC(rct_sigbase_t)
MALLOC(rct_sig_prunable_t)

*/
import "C"

import (
	"fmt"
	"reflect"
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

func VerRctNonSemanticsSimple(rs *types.RctSig) bool {
	cIn := C.malloc_rct_sig_t()
	defer C.free(unsafe.Pointer(cIn))
	toRctSig(rs, cIn)
	ret := C.x_verRctNonSemanticsSimple(cIn)
	if ret == 0 {
		return true
	}
	return false
}

func VerRctWithSemantics(rs *types.RctSig, semantics int) bool {
	cIn := C.malloc_rct_sig_t()
	defer C.free(unsafe.Pointer(cIn))
	toRctSig(rs, cIn)
	ret := C.x_verRctWithSemantics(cIn, C.int(semantics))
	if ret == 0 {
		return true
	}
	return false
}

func VerRct(rs *types.RctSig) bool {
	cIn := C.malloc_rct_sig_t()
	defer C.free(unsafe.Pointer(cIn))
	toRctSig(rs, cIn)

	// @Note: we will release memory at x_verRct before it return.
	ret := C.x_verRct(cIn)
	if ret == 0 {
		return true
	}
	return false
}

type GenRctContext struct {
	Message      types.Key
	InSk         types.CtkeyV
	Destinations types.KeyV
	Amounts      []types.Lk_amount
	MixRing      types.CtkeyM
	AmountKeys   types.KeyV
	KLRki        *types.MultisigKLRki // avoid panic: cgo argument has Go pointer to Go pointer
	Msout        *types.MultisigOut
	Index        uint
	OutSk        types.CtkeyV
	RctConfig    *types.RctConfig
}

// GenRct generate RingSignature
func GenRct(ctx *GenRctContext) (*types.RctSig, error) {
	cInSk := C.struct_rct_ctkeyV{}
	cDestinations := C.struct_rct_keyV{}
	cAmounts := C.struct_amountV{}
	cMixRing := C.struct_rct_ctkeyM{}
	cAmountKeys := C.struct_rct_keyV{}
	cKLRki := C.struct_rct_multisig_kLRki{}
	// cMsout := C.struct_rct_multisig_out{}
	cMsout := C.struct_rct_keyV{}
	cIndex := C.uint(ctx.Index)
	cOutSk := C.struct_rct_ctkeyV{}
	cRctConfig := C.struct_RCTConfig{}

	// convert type
	toRctCtkeyV(ctx.InSk, &cInSk)
	toRctKeyV(ctx.Destinations, &cDestinations)
	toAmountV(ctx.Amounts, &cAmounts)
	toRctCtKeyM(ctx.MixRing, &cMixRing)
	toRctKeyV(ctx.AmountKeys, &cAmountKeys)
	toRctMultisigKLRki(ctx.KLRki, &cKLRki)

	toRctKeyV(ctx.Msout.C, &cMsout)
	toRctCtkeyV(ctx.OutSk, &cOutSk)
	cRctConfig.range_proof_type = C.int(ctx.RctConfig.RangeProofType)
	cRctConfig.bp_version = C.int(ctx.RctConfig.BpVersion)

	cRctSig := C.struct_rct_sig{}
	C.init_rct_sig_callback(&cRctSig)

	_, err := C.x_genRct((*C.char)(unsafe.Pointer(&ctx.Message[0])), &cInSk, &cDestinations, &cAmounts, &cMixRing, &cAmountKeys, &cKLRki, &cMsout, cIndex, &cOutSk, &cRctConfig, &cRctSig)
	if err != nil {
		return nil, err
	}

	out := free(objectID(cRctSig.id)).(*types.RctSig)
	ctx.Msout.C = fromRctKeyV(&cMsout)
	ctx.OutSk = fromRctCtKeyV(&cOutSk)
	// @Note: release memory
	C.x_free_rct_keyV(&cMsout)
	C.x_free_rct_ctkeyV(&cOutSk)
	return out, err
}

type GenRctSimpleContext struct {
	Message      types.Key
	InSk         types.CtkeyV
	Destinations types.KeyV
	InAmounts    []types.Lk_amount
	OutAmounts   []types.Lk_amount
	TxnFee       uint64
	MixRing      types.CtkeyM
	AmountKeys   types.KeyV
	KLRki        []types.MultisigKLRki
	Msout        *types.MultisigOut
	Indexs       []uint
	OutSk        types.CtkeyV
	RctConfig    *types.RctConfig
}

func GenRctSimple(ctx *GenRctSimpleContext) (*types.RctSig, error) {

	cInSk := C.struct_rct_ctkeyV{}
	cDestinations := C.struct_rct_keyV{}
	cInAmounts := C.struct_amountV{}
	cOutAmounts := C.struct_amountV{}
	cTxnFee := C.uint64_t(ctx.TxnFee)
	cMixRing := C.struct_rct_ctkeyM{}
	cAmountKeys := C.struct_rct_keyV{}
	cKLRki := C.struct_rct_multisig_kLRkiV{}
	// cMsout := C.struct_rct_multisig_out{}v
	cMsout := C.struct_rct_keyV{}
	cIndexs := C.struct_indexV{}
	cOutSk := C.struct_rct_ctkeyV{}
	cRctConfig := C.struct_RCTConfig{}

	toRctCtkeyV(ctx.InSk, &cInSk)
	toRctKeyV(ctx.Destinations, &cDestinations)
	toAmountV(ctx.InAmounts, &cInAmounts)
	toAmountV(ctx.OutAmounts, &cOutAmounts)
	toRctCtKeyM(ctx.MixRing, &cMixRing)
	toRctKeyV(ctx.AmountKeys, &cAmountKeys)
	toRctMultisigKLRkiV(ctx.KLRki, &cKLRki)
	toRctKeyV(ctx.Msout.C, &cMsout)
	toIndexV(ctx.Indexs, &cIndexs)
	toRctCtkeyV(ctx.OutSk, &cOutSk)
	cRctConfig.range_proof_type = C.int(ctx.RctConfig.RangeProofType)
	cRctConfig.bp_version = C.int(ctx.RctConfig.BpVersion)

	cRctSig := C.struct_rct_sig{}
	C.init_rct_sig_callback(&cRctSig)
	_, err := C.x_genRctSimple((*C.char)(unsafe.Pointer(&ctx.Message[0])), &cInSk, &cDestinations, &cInAmounts, &cOutAmounts, cTxnFee, &cMixRing, &cAmountKeys, &cKLRki, &cMsout, &cIndexs, &cOutSk, &cRctConfig, &cRctSig)
	if err != nil {
		return nil, err
	}

	out := free(objectID(cRctSig.id)).(*types.RctSig)

	ctx.Msout.C = fromRctKeyV(&cMsout)
	ctx.OutSk = fromRctCtKeyV(&cOutSk)
	// @Note: release memory
	C.x_free_rct_keyV(&cMsout)
	C.x_free_rct_ctkeyV(&cOutSk)
	return out, err
}

func ScalarmultKey(p, a types.Key) (ret types.Key, err error) {
	retp := unsafe.Pointer(&ret[0])
	_, err = C.x_scalarmultKey((*C.char)(retp), (*C.char)(unsafe.Pointer(&p[0])), (*C.char)(unsafe.Pointer(&a[0])))
	return ret, err
}
func VerRctSemanticsSimple(rs *types.RctSig) bool {
	cIn := C.malloc_rct_sig_t()
	defer C.free(unsafe.Pointer(cIn))
	toRctSig(rs, cIn)
	ret := C.x_verRctSemanticsSimple(cIn)
	if ret == 0 {
		return true
	}
	return false
}
func VerRctSimple(rs *types.RctSig) bool {
	cIn := C.malloc_rct_sig_t()
	defer C.free(unsafe.Pointer(cIn))
	toRctSig(rs, cIn)
	ret := C.x_verRctSimple(cIn)
	if ret == 0 {
		return true
	}
	return false
}
func ScalarmultBase(a types.Key) (ret types.Key) {
	retp := unsafe.Pointer(&ret[0])
	C.x_scalarmultBase((*C.char)(retp), (*C.char)(unsafe.Pointer(&a[0])))
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

func HashToScalar(keys types.KeyV) (key types.Key) {
	cKeys := C.rct_keyV_t{}
	toRctKeyV(keys, &cKeys)
	C.x_hash_to_scalar(&cKeys, C.p_rct_key_t(unsafe.Pointer(&key[0])))
	return key
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

func GetPreMlsagHash(rs *types.RctSig) (hash types.Key, err error) {
	cIn := C.malloc_rct_sig_t()
	defer C.free(unsafe.Pointer(cIn))
	toRctSig(rs, cIn)
	_, err = C.x_get_pre_mlsag_hash((*C.char)(unsafe.Pointer(&hash[0])), cIn)
	return hash, err
}

// ---------------------------------------------------------------------------------

func toSlice(cPtr, goPtr unsafe.Pointer, n int) {
	sh := (*reflect.SliceHeader)(goPtr)
	sh.Cap = n
	sh.Len = n
	sh.Data = uintptr(cPtr)
}

func toRctCtkeyV(from types.CtkeyV, to *C.struct_rct_ctkeyV) {
	to.nums = C.int(len(from))
	if len(from) > 0 {
		ptrs := C.malloc(C.sizeof_rct_ctkey_t * C.ulong(len(from)))
		var ps []C.rct_ctkey_t
		toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), len(from))

		for index := 0; index < len(from); index++ {
			ps[index].dest = C.p_rct_key_t(unsafe.Pointer(&from[index].Dest[0]))
			ps[index].mask = C.p_rct_key_t(unsafe.Pointer(&from[index].Mask[0]))
		}
		to.v = C.p_rct_ctkey_t(ptrs)
	}
}

func toRctKeyV(from types.KeyV, to *C.struct_rct_keyV) {
	to.nums = C.int(len(from))
	if len(from) > 0 {
		ptrs := C.malloc(C.sizeof_p_rct_key_t * C.ulong(len(from)))
		var ps []C.p_rct_key_t
		toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), len(from))

		for index := 0; index < len(from); index++ {
			ps[index] = C.p_rct_key_t(unsafe.Pointer(&from[index][0]))
		}
		to.v = C.pp_rct_key_t(ptrs)
	}
}

func toAmountV(amounts []types.Lk_amount, to *C.struct_amountV) {
	if len(amounts) > 0 {
		ptrs := C.malloc(C.sizeof_uint64_t * C.ulong(len(amounts)))
		var ps []C.uint64_t
		toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), len(amounts))

		for index := 0; index < len(amounts); index++ {
			ps[index] = C.uint64_t(amounts[index])
		}
		C.setn_amountV_t(to, ptrs, C.int(len(amounts)))
	} else {
		C.setn_amountV_t(to, C.NULL, C.int(len(amounts)))
	}
}

func toIndexV(indexs []uint, to *C.struct_indexV) {
	if len(indexs) > 0 {
		ptrs := C.malloc(C.sizeof_uint64_t * C.ulong(len(indexs)))
		var ps []C.uint64_t
		toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), len(indexs))

		for index := 0; index < len(indexs); index++ {
			ps[index] = C.uint64_t(indexs[index])
		}
		C.setn_indexV_t(to, ptrs, C.int(len(indexs)))
	} else {
		C.setn_indexV_t(to, C.NULL, C.int(len(indexs)))
	}
}

func toRctKeyM(from types.KeyM, to *C.struct_rct_keyM) {
	to.nums = C.int(len(from))
	if len(from) > 0 {
		ptrs := C.malloc(C.sizeof_rct_keyV_t * C.ulong(len(from)))
		var ps []C.rct_keyV_t
		toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), len(from))

		for index := 0; index < len(from); index++ {
			toRctKeyV(from[index], &ps[index])
		}
		to.m = C.p_rct_keyV_t(ptrs)
	}
}

func toRctCtKeyM(from types.CtkeyM, to *C.struct_rct_ctkeyM) {
	to.nums = C.int(len(from))
	if len(from) > 0 {
		ptrs := C.malloc(C.sizeof_rct_ctkeyV_t * C.ulong(len(from)))
		var ps []C.rct_ctkeyV_t
		toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), len(from))

		for index := 0; index < len(from); index++ {
			toRctCtkeyV(from[index], &ps[index])
		}
		to.m = C.p_rct_ctkeyV_t(ptrs)
	}
}

func toRctMultisigKLRki(klrki *types.MultisigKLRki, to *C.struct_rct_multisig_kLRki) {
	to.k = C.p_rct_key_t(unsafe.Pointer(&klrki.K[0]))
	to.L = C.p_rct_key_t(unsafe.Pointer(&klrki.L[0]))
	to.R = C.p_rct_key_t(unsafe.Pointer(&klrki.R[0]))
	to.ki = C.p_rct_key_t(unsafe.Pointer(&klrki.Ki[0]))
}

func toRctMultisigKLRkiV(v []types.MultisigKLRki, to *C.struct_rct_multisig_kLRkiV) {
	if len(v) > 0 {
		ptrs := C.malloc(C.sizeof_rct_multisig_kLRki_t * C.ulong(len(v)))
		var ps []C.rct_multisig_kLRki_t
		toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), len(v))

		for index := 0; index < len(v); index++ {
			toRctMultisigKLRki(&v[index], &ps[index])
		}
		C.setn_rct_multisig_kLRkiV_t(to, ptrs, C.int(len(v)))
	} else {
		C.setn_rct_multisig_kLRkiV_t(to, C.NULL, C.int(len(v)))
	}
}

func fromRctKeyV(from *C.struct_rct_keyV) types.KeyV {
	v := make([]types.Key, from.nums)
	if len(v) > 0 {
		var ps []C.p_rct_key_t
		toSlice(unsafe.Pointer(from.v), unsafe.Pointer(&ps), int(from.nums))

		var data []byte
		sh := (*reflect.SliceHeader)((unsafe.Pointer(&data)))
		sh.Cap = types.HASH_SIZE
		sh.Len = types.HASH_SIZE

		for index := 0; index < len(v); index++ {
			sh.Data = uintptr(unsafe.Pointer(ps[index]))
			copy(v[index][:], data)
		}
	}
	return v
}

func fromRctKeyM(from *C.struct_rct_keyM) types.KeyM {
	m := make([]types.KeyV, int(from.nums))
	if len(m) > 0 {
		var ps []C.rct_keyV_t
		toSlice(unsafe.Pointer(from.m), unsafe.Pointer(&ps), int(from.nums))

		for index := 0; index < len(m); index++ {
			m[index] = fromRctKeyV(&ps[index])
		}
	}
	return m
}

func fromRctCtKeyV(from *C.struct_rct_ctkeyV) types.CtkeyV {
	v := make([]types.Ctkey, int(from.nums))
	if len(v) > 0 {
		var ps []C.rct_ctkey_t
		toSlice(unsafe.Pointer(from.v), unsafe.Pointer(&ps), int(from.nums))

		var data []byte
		sh := (*reflect.SliceHeader)((unsafe.Pointer(&data)))
		sh.Cap = types.HASH_SIZE
		sh.Len = types.HASH_SIZE

		for index := 0; index < len(v); index++ {
			sh.Data = uintptr(unsafe.Pointer(ps[index].dest))
			copy(v[index].Dest[:], data)
			sh.Data = uintptr(unsafe.Pointer(ps[index].mask))
			copy(v[index].Mask[:], data)
		}
	}
	return v
}

func fromRctCtKeyM(from *C.struct_rct_ctkeyM) types.CtkeyM {
	m := make([]types.CtkeyV, from.nums)
	if len(m) > 0 {
		var ps []C.rct_ctkeyV_t
		toSlice(unsafe.Pointer(from.m), unsafe.Pointer(&ps), len(m))

		for index := 0; index < len(m); index++ {
			m[index] = fromRctCtKeyV(&ps[index])
		}
	}
	return m
}

func toRctEcdhTuple(from *types.EcdhTuple, to *C.struct_rct_ecdhTuple) {
	to.mask = C.p_rct_key_t(unsafe.Pointer(&from.Mask[0]))
	to.amount = C.p_rct_key_t(unsafe.Pointer(&from.Amount[0]))
}

func toRctEcdhTupleV(from []types.EcdhTuple, to *C.struct_rct_ecdhTupleV) {
	to.nums = C.int(len(from))
	if len(from) > 0 {
		ptrs := C.malloc(C.sizeof_rct_ecdhTuple_t * C.ulong(len(from)))
		var ps []C.rct_ecdhTuple_t
		toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), len(from))

		for index := 0; index < len(from); index++ {
			toRctEcdhTuple(&from[index], &ps[index])
			// ps[index].mask = C.p_rct_key_t(unsafe.Pointer(&from[index].Mask[0]))
			// ps[index].amount = C.p_rct_key_t(unsafe.Pointer(&from[index].Amount[0]))
		}
		to.v = C.p_rct_ecdhTuple_t(ptrs)
	}
}

func fromRctEcdhTuple(from *C.struct_rct_ecdhTuple, to *types.EcdhTuple) {
	var data []byte
	sh := (*reflect.SliceHeader)((unsafe.Pointer(&data)))
	sh.Cap = types.HASH_SIZE
	sh.Len = types.HASH_SIZE

	sh.Data = uintptr(unsafe.Pointer(from.mask))
	copy(to.Mask[:], data)
	sh.Data = uintptr(unsafe.Pointer(from.amount))
	copy(to.Amount[:], data)
}

func fromRctEcdhTupleV(from *C.struct_rct_ecdhTupleV) []types.EcdhTuple {
	v := make([]types.EcdhTuple, from.nums)
	if len(v) > 0 {
		var ps []C.rct_ecdhTuple_t
		toSlice(unsafe.Pointer(from.v), unsafe.Pointer(&ps), len(v))

		var data []byte
		sh := (*reflect.SliceHeader)((unsafe.Pointer(&data)))
		sh.Cap = types.HASH_SIZE
		sh.Len = types.HASH_SIZE

		for index := 0; index < len(v); index++ {
			sh.Data = uintptr(unsafe.Pointer(ps[index].mask))
			copy(v[index].Mask[:], data)
			sh.Data = uintptr(unsafe.Pointer(ps[index].amount))
			copy(v[index].Amount[:], data)
		}
	}
	return v
}

func toRctKey64(from *types.Key64, to *C.rct_key64_t) {
	ptrs := C.malloc(C.sizeof_p_rct_key_t * 64)
	var ps []C.p_rct_key_t
	toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), 64)

	for index := 0; index < 64; index++ {
		ps[index] = C.p_rct_key_t(unsafe.Pointer(&from[index][0]))
	}
	C.set_rct_key64_t(to, ptrs)
	C.free(unsafe.Pointer(ptrs))
}

func fromRctKey64(from *C.rct_key64_t, v *types.Key64) {
	ptrs := C.malloc(C.sizeof_p_rct_key_t * 64)
	C.get_rct_key64_t(from, ptrs)

	var ps []C.p_rct_key_t
	toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), 64)

	var data []byte
	sh := (*reflect.SliceHeader)((unsafe.Pointer(&data)))
	sh.Cap = types.HASH_SIZE
	sh.Len = types.HASH_SIZE
	for index := 0; index < 64; index++ {
		sh.Data = uintptr(unsafe.Pointer(ps[index]))
		copy(v[index][:], data)
	}

	C.free(unsafe.Pointer(ptrs))
}

func toRctRangeSigV(from []types.RangeSig, to *C.struct_rct_rangeSigV) {
	to.nums = C.int(len(from))
	if len(from) > 0 {
		ptrs := C.malloc(C.sizeof_rct_rangeSig_t * C.ulong(len(from)))
		var ps []C.rct_rangeSig_t
		toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), len(from))

		for index := 0; index < len(from); index++ {
			toRctKey64(&from[index].Ci, &ps[index].Ci)
			toRctKey64(&from[index].Asig.S0, &ps[index].asig.s0)
			toRctKey64(&from[index].Asig.S1, &ps[index].asig.s1)
			ps[index].asig.ee = C.p_rct_key_t(unsafe.Pointer(&from[index].Asig.Ee[0]))
		}
		to.v = C.p_rct_rangeSig_t(ptrs)
	}
}

func fromRctRangeSigV(from *C.struct_rct_rangeSigV) []types.RangeSig {
	v := make([]types.RangeSig, from.nums)
	if len(v) > 0 {
		var ps []C.rct_rangeSig_t
		toSlice(unsafe.Pointer(from.v), unsafe.Pointer(&ps), len(v))

		var data []byte
		sh := (*reflect.SliceHeader)((unsafe.Pointer(&data)))
		sh.Cap = types.HASH_SIZE
		sh.Len = types.HASH_SIZE
		for index := 0; index < len(v); index++ {
			fromRctKey64(&ps[index].Ci, &v[index].Ci)
			fromRctKey64(&ps[index].asig.s0, &v[index].Asig.S0)
			fromRctKey64(&ps[index].asig.s1, &v[index].Asig.S1)
			sh.Data = uintptr(unsafe.Pointer(ps[index].asig.ee))
			copy(v[index].Asig.Ee[:], data)
		}
	}
	return v
}

func toRctBulletproofV(from []types.Bulletproof, to *C.struct_rct_bulletproofV) {
	to.nums = C.int(len(from))
	if len(from) > 0 {
		ptrs := C.malloc(C.sizeof_rct_bulletproof_t * C.ulong(len(from)))
		var ps []C.rct_bulletproof_t
		toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), len(from))

		for index := 0; index < len(from); index++ {
			toRctKeyV(from[index].V, &ps[index].V)
			ps[index].A = C.p_rct_key_t(unsafe.Pointer(&from[index].A[0]))
			ps[index].S = C.p_rct_key_t(unsafe.Pointer(&from[index].S[0]))
			ps[index].T1 = C.p_rct_key_t(unsafe.Pointer(&from[index].T1[0]))
			ps[index].T2 = C.p_rct_key_t(unsafe.Pointer(&from[index].T2[0]))
			ps[index].taux = C.p_rct_key_t(unsafe.Pointer(&from[index].Taux[0]))
			ps[index].mu = C.p_rct_key_t(unsafe.Pointer(&from[index].Mu[0]))
			toRctKeyV(from[index].L, &ps[index].L)
			toRctKeyV(from[index].R, &ps[index].R)
			ps[index].a = C.p_rct_key_t(unsafe.Pointer(&from[index].Aa[0]))
			ps[index].b = C.p_rct_key_t(unsafe.Pointer(&from[index].B[0]))
			ps[index].t = C.p_rct_key_t(unsafe.Pointer(&from[index].T[0]))
		}
		to.v = C.p_rct_bulletproof_t(ptrs)
	}
}

func fromRctBulletproofV(from *C.struct_rct_bulletproofV) []types.Bulletproof {
	v := make([]types.Bulletproof, from.nums)
	if len(v) > 0 {
		var ps []C.rct_bulletproof_t
		toSlice(unsafe.Pointer(from.v), unsafe.Pointer(&ps), len(v))

		var data []byte
		sh := (*reflect.SliceHeader)((unsafe.Pointer(&data)))
		sh.Cap = types.HASH_SIZE
		sh.Len = types.HASH_SIZE
		for index := 0; index < len(v); index++ {
			v[index].V = fromRctKeyV(&ps[index].V)
			sh.Data = uintptr(unsafe.Pointer(ps[index].A))
			copy(v[index].A[:], data)
			sh.Data = uintptr(unsafe.Pointer(ps[index].S))
			copy(v[index].S[:], data)
			sh.Data = uintptr(unsafe.Pointer(ps[index].T1))
			copy(v[index].T1[:], data)
			sh.Data = uintptr(unsafe.Pointer(ps[index].T2))
			copy(v[index].T2[:], data)
			sh.Data = uintptr(unsafe.Pointer(ps[index].taux))
			copy(v[index].Taux[:], data)
			sh.Data = uintptr(unsafe.Pointer(ps[index].mu))
			copy(v[index].Mu[:], data)
			v[index].L = fromRctKeyV(&ps[index].L)
			v[index].R = fromRctKeyV(&ps[index].R)

			sh.Data = uintptr(unsafe.Pointer(ps[index].a))
			copy(v[index].Aa[:], data)
			sh.Data = uintptr(unsafe.Pointer(ps[index].b))
			copy(v[index].B[:], data)
			sh.Data = uintptr(unsafe.Pointer(ps[index].t))
			copy(v[index].T[:], data)
		}
	}
	return v
}

func toRctMgSigV(from []types.MgSig, to *C.struct_rct_mgSigV) {
	to.nums = C.int(len(from))
	if len(from) > 0 {
		pstr := C.malloc(C.sizeof_rct_mgSig_t * C.ulong(len(from)))
		var ps []C.rct_mgSig_t
		sh := (*reflect.SliceHeader)(unsafe.Pointer(&ps))
		sh.Len = len(from)
		sh.Cap = len(from)
		sh.Data = uintptr(unsafe.Pointer(pstr))

		for index := 0; index < len(from); index++ {
			toRctKeyM(from[index].Ss, &ps[index].ss)
			toRctKeyV(from[index].II, &ps[index].II)
			ps[index].cc = C.p_rct_key_t(unsafe.Pointer(&from[index].Cc[0]))
		}
		to.v = C.p_rct_mgSig_t(pstr)
	}
}

func fromRctMgSigV(from *C.struct_rct_mgSigV) []types.MgSig {
	v := make([]types.MgSig, from.nums)
	if len(v) > 0 {
		var ps []C.rct_mgSig_t
		toSlice(unsafe.Pointer(from.v), unsafe.Pointer(&ps), len(v))

		var data []byte
		sh := (*reflect.SliceHeader)(unsafe.Pointer(&data))
		sh.Len = types.HASH_SIZE
		sh.Cap = types.HASH_SIZE
		for index := 0; index < len(v); index++ {
			v[index].Ss = fromRctKeyM(&ps[index].ss)
			v[index].II = fromRctKeyV(&ps[index].II)
			sh.Data = uintptr(unsafe.Pointer(ps[index].cc))
			copy(v[index].Cc[:], data)
		}
	}
	return v
}

func fromRctSigbase(from *C.struct_rct_sigbase, to *types.RctSigBase) {
	to.Type = uint8(from.Type)
	to.MixRing = fromRctCtKeyM(&from.mixRing)
	to.PseudoOuts = fromRctKeyV(&from.pseudoOuts)
	to.EcdhInfo = fromRctEcdhTupleV(&from.ecdhInfo)
	to.OutPk = fromRctCtKeyV(&from.outPk)
	to.TxnFee = types.Lk_amount(from.txnFee)

	var data []byte
	toSlice(unsafe.Pointer(from.message), unsafe.Pointer(&data), types.HASH_SIZE)
	copy(to.Message[:], data)
}

func toRctSigbase(from *types.RctSigBase, to *C.struct_rct_sigbase) {
	to.Type = C.uint8_t(from.Type)
	to.message = (C.p_rct_key_t)(unsafe.Pointer(&from.Message[0]))
	toRctCtKeyM(from.MixRing, &to.mixRing)
	toRctKeyV(from.PseudoOuts, &to.pseudoOuts)
	toRctEcdhTupleV(from.EcdhInfo, &to.ecdhInfo)
	toRctCtkeyV(from.OutPk, &to.outPk)
	to.txnFee = C.uint64_t(from.TxnFee)
}

func fromRctSigPrunable(from *C.struct_rct_sig_prunable, to *types.RctSigPrunable) {
	to.RangeSigs = fromRctRangeSigV(&from.rangeSigs)
	to.Bulletproofs = fromRctBulletproofV(&from.bulletproofs)
	to.MGs = fromRctMgSigV(&from.MGs)
	to.PseudoOuts = fromRctKeyV(&from.pseudoOuts)
}

func toRctSigPrunable(from *types.RctSigPrunable, to *C.struct_rct_sig_prunable) {
	toRctRangeSigV(from.RangeSigs, &to.rangeSigs)
	toRctBulletproofV(from.Bulletproofs, &to.bulletproofs)
	toRctMgSigV(from.MGs, &to.MGs)
	toRctKeyV(from.PseudoOuts, &to.pseudoOuts)
}

func toRctSig(from *types.RctSig, to *C.struct_rct_sig) {
	toRctSigbase(&from.RctSigBase, &to.base)
	toRctSigPrunable(&from.P, &to.p)
}

//export GoFromRctSig
func GoFromRctSig(cRctSig *C.struct_rct_sig) {
	data := new(types.RctSig)
	fromRctSigbase(&cRctSig.base, &data.RctSigBase)
	fromRctSigPrunable(&cRctSig.p, &data.P)

	id := put(data)
	cRctSig.id = C.int32_t(id)
}

// -------------------------------------------------------------
// just for conversion test

func ccRctSig(in *types.RctSig) *types.RctSig {
	cIn := C.malloc_rct_sig_t()
	toRctSig(in, cIn)

	cOut := C.struct_rct_sig{}
	C.init_rct_sig_callback(&cOut)
	C.testc_rct_sig(cIn, &cOut)

	out := free(objectID(cOut.id)).(*types.RctSig)
	return out
}

func ccRctPublicKeyV(pksIn []types.PublicKey) (pksOut []types.PublicKey) {
	cKeyV := C.struct_rct_keyV{}
	if len(pksIn) > 0 {
		ptrs := C.malloc(C.sizeof_p_rct_key_t * C.ulong(len(pksIn)))
		var ps []C.p_rct_key_t
		toSlice(unsafe.Pointer(ptrs), unsafe.Pointer(&ps), len(pksIn))

		for index := 0; index < len(pksIn); index++ {
			ps[index] = C.p_rct_key_t(unsafe.Pointer(&pksIn[index][0]))
		}
		C.setn_rct_keyV_t(&cKeyV, ptrs, C.int(len(pksIn)))
		defer C.free(ptrs)
	} else {
		C.setn_rct_keyV_t(&cKeyV, C.NULL, C.int(len(pksIn)))
	}

	pksOut = make([]types.PublicKey, cKeyV.nums)
	var ps []C.p_rct_key_t
	toSlice(unsafe.Pointer(cKeyV.v), unsafe.Pointer(&ps), int(len(pksOut)))

	var data []byte
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	sh.Len = types.HASH_SIZE
	sh.Cap = types.HASH_SIZE
	for index := 0; index < len(pksIn); index++ {
		sh.Data = uintptr(unsafe.Pointer(ps[index]))
		copy(pksOut[index][:], data)
	}

	return pksOut
}

func ccSignature(sigIn types.Signature) (sigOut types.Signature) {
	cSig := C.struct_signature{}
	C.fill_signature(&cSig, unsafe.Pointer(&sigIn.C[0]), unsafe.Pointer(&sigIn.R[0]))

	C.extract_signature(&cSig, unsafe.Pointer(&sigOut.C[0]), unsafe.Pointer(&sigOut.R[0]))
	return sigOut
}

//export GoFromRctKey
func GoFromRctKey(v *C.struct_test_rct_key) {
	data := make([]byte, types.HASH_SIZE)
	var tmp []byte
	toSlice(unsafe.Pointer(v.data), unsafe.Pointer(&tmp), types.HASH_SIZE)
	copy(data, tmp)
	id := put(data)
	v.id = C.int32_t(id)
}
func ccRctKey(in types.Key) (k types.Key) {
	cIn := C.p_rct_key_t(unsafe.Pointer(&in[0]))

	cOut := C.struct_test_rct_key{}
	C.init_test_rct_key_callback(&cOut)
	C.testc_rct_key(cIn, &cOut)

	out := free(objectID(cOut.id)).([]byte)
	copy(k[:], out[:])
	return k
}

//export GoFromRctKeyM
func GoFromRctKeyM(v *C.struct_test_rct_keyM) {
	out := fromRctKeyM(&v.data)
	id := put(out)
	v.id = C.int32_t(id)
}
func ccRctKeyM(in types.KeyM) types.KeyM {
	cIn := C.struct_rct_keyM{}
	toRctKeyM(in, &cIn)

	cOut := C.struct_test_rct_keyM{}
	C.init_test_rct_keyM_callback(&cOut)
	C.testc_rct_keyM(&cIn, &cOut)

	out := free(objectID(cOut.id)).(types.KeyM)
	return out
}

//export GoFromRctKeyV
func GoFromRctKeyV(v *C.struct_test_rct_keyV) {
	out := fromRctKeyV(&v.data)
	id := put(out)
	v.id = C.int32_t(id)
}
func ccRctKeyV(in types.KeyV) types.KeyV {
	cIn := C.struct_rct_keyV{}
	toRctKeyV(in, &cIn)

	cOut := C.struct_test_rct_keyV{}
	C.init_test_rct_keyV_callback(&cOut)
	C.testc_rct_keyV(&cIn, &cOut)

	out := free(objectID(cOut.id)).(types.KeyV)
	return out
}

//export GoFromRctCtKeyV
func GoFromRctCtKeyV(v *C.struct_test_rct_ctkeyV) {
	out := fromRctCtKeyV(&v.data)
	id := put(out)
	v.id = C.int32_t(id)
}
func ccRctCtKeyV(in types.CtkeyV) types.CtkeyV {
	cIn := C.struct_rct_ctkeyV{}
	toRctCtkeyV(in, &cIn)

	cOut := C.struct_test_rct_ctkeyV{}
	C.init_test_rct_ctkeyV_callback(&cOut)
	C.testc_rct_ctkeyV(&cIn, &cOut)

	out := free(objectID(cOut.id)).(types.CtkeyV)
	return out
}

//export GoFromRctCtKeyM
func GoFromRctCtKeyM(v *C.struct_test_rct_ctkeyM) {
	out := fromRctCtKeyM(&v.data)
	id := put(out)
	v.id = C.int32_t(id)
}
func ccRctCtKeyM(in types.CtkeyM) types.CtkeyM {
	cIn := C.struct_rct_ctkeyM{}
	toRctCtKeyM(in, &cIn)

	cOut := C.struct_test_rct_ctkeyM{}
	C.init_test_rct_ctkeyM_callback(&cOut)
	C.testc_rct_ctkeyM(&cIn, &cOut)

	out := free(objectID(cOut.id)).(types.CtkeyM)
	return out
}

//export GoFromRctSigbase
func GoFromRctSigbase(v *C.struct_test_rct_sigbase) {
	out := new(types.RctSigBase)
	fromRctSigbase(&v.data, out)
	id := put(out)
	v.id = C.int32_t(id)
}
func ccRctSigbase(in *types.RctSigBase) *types.RctSigBase {
	cIn := C.malloc_rct_sigbase_t()
	toRctSigbase(in, cIn)

	cOut := C.struct_test_rct_sigbase{}
	C.init_test_rct_sigbase_callback(&cOut)
	C.testc_rct_sigbase(cIn, &cOut)

	out := free(objectID(cOut.id)).(*types.RctSigBase)
	return out
}

/*
//export GoFromRctSigprunable
func GoFromRctSigprunable(v *C.struct_test_rct_sig_prunable_t) {
	out := new(types.RctSigPrunable)
	fromRctSigPrunable(&v.data, out)
	id := put(out)
	v.id = C.int32_t(id)
}
*/
