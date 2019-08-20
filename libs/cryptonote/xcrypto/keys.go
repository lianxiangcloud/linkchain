package xcrypto

/*
#cgo CFLAGS: -O3 -I./
#include "xcrypto.h"
*/
import "C"

import (
	"fmt"
	"reflect"
	"unsafe"

	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
)

// WordsToBytes convert words to recover_key
func WordsToBytes(words string) (sec types.SecretKey, err error) {
	cs := C.CString(words)
	ret := C.x_words_to_bytes(cs, C.p_secret_key_t(unsafe.Pointer(&sec[0])))
	C.free(unsafe.Pointer(cs))

	if ret < 0 {
		return sec, fmt.Errorf("CGO x_words_to_bytes fail")
	}
	return sec, nil
}

// BytesToWords convert recover_key to words
func BytesToWords(sec types.SecretKey, lang string) (words string, err error) {
	cLang := C.CString(lang)
	var p *C.char

	ret := C.x_bytes_to_words(C.p_secret_key_t(unsafe.Pointer(&sec[0])), &p, cLang)
	C.free(unsafe.Pointer(cLang))

	if ret < 0 {
		return "", fmt.Errorf("CGO x_bytes_to_words fail")
	}

	words = C.GoString(p)
	C.free(unsafe.Pointer(p))
	return words, nil
}

// GenerateKeys generate secret_key and public_key
func GenerateKeys(recoverKey types.SecretKey) (sk types.SecretKey, pk types.PublicKey) {
	C.x_generate_keys(C.p_public_key_t(unsafe.Pointer(&pk[0])), C.p_secret_key_t(unsafe.Pointer(&sk[0])), C.p_secret_key_t(unsafe.Pointer(&recoverKey[0])))
	return sk, pk
}

// SecretAdd : r = a+b
func SecretAdd(a, b types.SecretKey) (r types.SecretKey) {
	C.x_sc_secret_add(C.p_secret_key_t(unsafe.Pointer(&r[0])), C.p_secret_key_t(unsafe.Pointer(&a[0])), C.p_secret_key_t(unsafe.Pointer(&b[0])))
	return r
}

// GetSubaddressSecretKey get sub secret_key by index.
func GetSubaddressSecretKey(main types.SecretKey, index uint32) (sub types.SecretKey) {
	C.x_get_subaddress_secret_key(C.p_secret_key_t(unsafe.Pointer(&main[0])), C.uint32_t(index), C.p_secret_key_t(unsafe.Pointer(&sub[0])))
	return sub
}

// GetSubaddress get sub public_key pair by index
func GetSubaddress(keys *types.AccountKey, index uint32) (addr types.AccountAddress) {
	addr, err := TlvGetSubaddress(keys, index)
	if err != nil {
		panic(err)
	}
	return addr
	//cAccountKey := (*C.account_keys_t)(C.malloc(C.sizeof_account_keys_t))
	//toAccountKeys(keys, cAccountKey)
	//cAccountAddr := (*C.account_public_address_t)(C.malloc(C.sizeof_account_public_address_t))
	//toAccountAddress(&addr, cAccountAddr)
	//
	//C.x_get_subaddress(cAccountKey, C.uint32_t(index), cAccountAddr)
	//C.free(unsafe.Pointer(cAccountAddr))
	//C.free(unsafe.Pointer(cAccountKey))
	//return addr
}

// GetSubaddressSpendPublicKeys get sub spend_key by [begin, end) index.
func GetSubaddressSpendPublicKeys(keys *types.AccountKey, begin, end uint32) ([]types.PublicKey, error) {
	if begin <= end {
		return nil, fmt.Errorf("invalid params: begin <= end")
	}

	spendPubs := make([]types.PublicKey, (end - begin))
	ptrs := C.malloc(C.sizeof_p_public_key_t * C.ulong(len(spendPubs)))
	var ps []C.p_public_key_t
	toSlice(ptrs, unsafe.Pointer(&ps), len(spendPubs))
	for i := 0; i < len(spendPubs); i++ {
		ps[i] = C.p_public_key_t(unsafe.Pointer(&spendPubs[i][0]))
	}

	cAccountKey := (*C.account_keys_t)(C.malloc(C.sizeof_account_keys_t))
	toAccountKeys(keys, cAccountKey)

	_, err := C.x_get_subaddress_spend_public_keys(cAccountKey, C.uint32_t(begin), C.uint32_t(end), (*C.p_public_key_t)(ptrs))
	C.free(unsafe.Pointer(ptrs))
	C.free(unsafe.Pointer(cAccountKey))

	return spendPubs, err
}

// GenerateKeyDerivation generate KeyDerivation
func GenerateKeyDerivation(pub types.PublicKey, sec types.SecretKey) (der types.KeyDerivation, err error) {
	ret := C.x_generate_key_derivation(C.p_public_key_t(unsafe.Pointer(&pub[0])), C.p_secret_key_t(unsafe.Pointer(&sec[0])), C.p_ec_point_t(unsafe.Pointer(&der[0])))
	if ret < 0 {
		return der, fmt.Errorf("CGO x_generate_key_derivation fail")
	}
	return der, nil
}

// DeriveSubaddressPublicKey derive public-key for subaddress.
func DeriveSubaddressPublicKey(pub types.PublicKey, derivation types.KeyDerivation, outIndex int) (derPub types.PublicKey, err error) {
	ret := C.x_derive_subaddress_public_key(C.p_public_key_t(unsafe.Pointer(&pub[0])), C.p_ec_point_t(unsafe.Pointer(&derivation[0])),
		C.size_t(outIndex), C.p_public_key_t(unsafe.Pointer(&derPub[0])))
	if ret < 0 {
		return derPub, fmt.Errorf("CGO x_derive_subadress_public_key fail")
	}
	return derPub, nil
}

// DeriveSecretKey derive sub secret-key
func DeriveSecretKey(derivation types.KeyDerivation, outIndex int, sec types.SecretKey) (derSec types.SecretKey, err error) {
	ret := C.x_derive_secret_key(C.p_ec_point_t(unsafe.Pointer(&derivation[0])), C.size_t(outIndex),
		C.p_secret_key_t(unsafe.Pointer(&sec[0])), C.p_secret_key_t(unsafe.Pointer(&derSec[0])))
	if ret < 0 {
		return derSec, fmt.Errorf("CGO x_derive_secret_key fail")
	}
	return derSec, nil
}

// DerivePublicKey derive sub public-key
func DerivePublicKey(derivation types.KeyDerivation, outIndex int, pub types.PublicKey) (derPub types.PublicKey, err error) {
	ret := C.x_derive_public_key(C.p_ec_point_t(unsafe.Pointer(&derivation[0])), C.size_t(outIndex),
		C.p_public_key_t(unsafe.Pointer(&pub[0])), C.p_public_key_t(unsafe.Pointer(&derPub[0])))
	if ret < 0 {
		return derPub, fmt.Errorf("CGO x_derive_public_key fail")
	}
	return derPub, nil
}

// SecretKeyToPublicKey get public-key from secret-key
func SecretKeyToPublicKey(sec types.SecretKey) (pub types.PublicKey, err error) {
	ret := C.x_secret_key_to_public_key(C.p_secret_key_t(unsafe.Pointer(&sec[0])), C.p_public_key_t(unsafe.Pointer(&pub[0])))
	if ret < 0 {
		return pub, fmt.Errorf("CGO x_secret_key_to_public_key fail")
	}
	return pub, nil
}

// GenerateKeyImage generate key_image
func GenerateKeyImage(pub types.PublicKey, sec types.SecretKey) (ki types.KeyImage, err error) {
	ret := C.x_generate_key_image(C.p_public_key_t(unsafe.Pointer(&pub[0])), C.p_secret_key_t(unsafe.Pointer(&sec[0])),
		C.p_key_image_t(unsafe.Pointer(&ki[0])))
	if ret < 0 {
		return ki, fmt.Errorf("CGO x_generate_key_image fail")
	}
	return ki, nil
}

// DerivationToScalar key-derivation to ec-scalar
func DerivationToScalar(derivation types.KeyDerivation, outIndex int) (res types.EcScalar, err error) {
	ret := C.x_derivation_to_scalar(C.p_ec_point_t(unsafe.Pointer(&derivation[0])), C.size_t(outIndex), C.p_ec_scalar_t(unsafe.Pointer(&res[0])))
	if ret < 0 {
		return res, fmt.Errorf("CGO x_derivation_to_scalar fail")
	}
	return res, nil
}

// -----------------------------------------------------------------

func toAccountAddress(from *types.AccountAddress, to *C.account_public_address_t) {
	to.spend = C.p_public_key_t(unsafe.Pointer(&from.SpendPublicKey[0]))
	to.view = C.p_public_key_t(unsafe.Pointer(&from.ViewPublicKey[0]))
}

func fromAccountAddress(from *C.account_public_address_t, to *types.AccountAddress) {
	var data []byte
	sh := (*reflect.SliceHeader)((unsafe.Pointer(&data)))
	sh.Cap = types.HASH_SIZE
	sh.Len = types.HASH_SIZE

	sh.Data = uintptr(unsafe.Pointer(from.spend))
	copy(to.SpendPublicKey[:], data)

	sh.Data = uintptr(unsafe.Pointer(from.view))
	copy(to.ViewPublicKey[:], data)
}

func toAccountKeys(from *types.AccountKey, to *C.account_keys_t) {
	toAccountAddress(&from.Addr, &to.address)
	to.spend = C.p_secret_key_t(unsafe.Pointer(&from.SpendSKey[0]))
	to.view = C.p_secret_key_t(unsafe.Pointer(&from.ViewSKey[0]))
}
