package xcrypto

/*
#cgo CFLAGS: -O3 -I./
#include "xcrypto.h"
*/
import "C"

import (
	"fmt"
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
func BytesToWords(sec types.SecretKey, lang string) (string, error) {
	cLang := C.CString(lang)
	var p *C.char

	ret := C.x_bytes_to_words(C.p_secret_key_t(unsafe.Pointer(&sec[0])), &p, cLang)
	C.free(unsafe.Pointer(cLang))

	if ret < 0 {
		return "", fmt.Errorf("CGO x_bytes_to_words fail")
	}

	words := C.GoString(p)
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
}

// GenerateKeyDerivation generate KeyDerivation
func GenerateKeyDerivation(pub types.PublicKey, sec types.SecretKey) (der types.KeyDerivation, err error) {
	ret := C.x_generate_key_derivation(C.p_public_key_t(unsafe.Pointer(&pub[0])),
		C.p_secret_key_t(unsafe.Pointer(&sec[0])), C.p_ec_point_t(unsafe.Pointer(&der[0])))
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
