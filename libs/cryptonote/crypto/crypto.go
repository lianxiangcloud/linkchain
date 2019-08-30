package crypto

import (
	. "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/xcrypto"
)

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

//GenerateRingSignature --
func GenerateRingSignature(prefixHash Hash, keyImage KeyImage, pubs []PublicKey, secretKey SecretKey, secIndex uint) (*Signature, error) {
	return xcrypto.GenerateRingSignature(prefixHash, keyImage, pubs, secretKey, secIndex)
}

//CheckRingSignature --
func CheckRingSignature(prefixHash Hash, keyImage KeyImage, pubs []PublicKey, signature *Signature) bool {
	return xcrypto.CheckRingSignature(prefixHash, keyImage, pubs, signature)
}
