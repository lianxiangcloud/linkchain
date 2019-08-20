package crypto

import (
	. "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/xcrypto"
)

//func GenerateKeys(pub PublicKey, sec, RecoveryKey SecretKey, recover bool) (*SecretKey) {
//	return nil
//}

func CheckKey(pub *PublicKey) bool {
	return xcrypto.CheckKey(*pub)
}

func SecretKeyToKey(secretKey SecretKey) (key Key) {
	copy(key[:], secretKey[:])
	return key
}
func KeyToSecretKey(key Key) (secretKey SecretKey) {
	copy(secretKey[:], key[:])
	return secretKey
}
func PublicKeyToKey(publicKey PublicKey) (key Key) {
	copy(key[:], publicKey[:])
	return key
}
func KeyToPublicKey(key Key) (publicKey PublicKey) {
	copy(publicKey[:], key[:])
	return publicKey
}

//
//func GenerateKeyDerivation(pub PublicKey, secretKey SecretKey) (bool, *KeyDerivation) {
//	return false, nil
//}
//
////derive_xxx
//func DerivationToScalar(derivation KeyDerivation, outputIndex uint) (*EcScalar) {
//	return nil
//}
//
//func DerivePublicKey(derivation KeyDerivation, outputIndex uint, publicKey PublicKey) (*PublicKey) {
//	return nil
//}
//
//func DeriveSecretKey(derivation KeyDerivation, outputIndex uint, secretKey SecretKey) (*SecretKey) {
//	return nil
//}
//
//func DeriveSubaddressPublicKey(publicKey PublicKey, derivation KeyDerivation, outputIndex uint) (*PublicKey) {
//	return nil
//}
//
//func GenerateSignature(prefixHash Hash, publicKey PublicKey, secretKey SecretKey) (*Signature) {
//	return nil
//}
//func CheckSignature(prefixHash Hash, publicKey PublicKey, signature Signature) (bool) {
//	return false
//}
//func GenerateTxProof(prefixHash Hash, R, A PublicKey, B *PublicKey, D PublicKey, secretKey SecretKey) (*Signature) {
//	return nil
//}
//func CheckTxProof(prefixHash Hash, R, A PublicKey, B *PublicKey, D PublicKey, signature *Signature) (bool) {
//	return false
//}
//
//func GenerateKeyImage(publicKey PublicKey, secretKey SecretKey) (*KeyImage) {
//	return nil
//}

//GenerateRingSignature --
func GenerateRingSignature(prefixHash Hash, keyImage KeyImage, pubs []PublicKey, secretKey SecretKey, secIndex uint) (*Signature, error) {
	return xcrypto.GenerateRingSignature(prefixHash, keyImage, pubs, secretKey, secIndex)
}

//CheckRingSignature --
func CheckRingSignature(prefixHash Hash, keyImage KeyImage, pubs []PublicKey, signature *Signature) bool {
	return xcrypto.CheckRingSignature(prefixHash, keyImage, pubs, signature)
}
