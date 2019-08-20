package crypto

import (
	"bytes"
	"crypto/subtle"

	secp256k1 "github.com/btcsuite/btcd/btcec"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"golang.org/x/crypto/ed25519"
)

func PrivKeyFromBytes(privKeyBytes []byte) (privKey PrivKey, err error) {
	err = ser.DecodeBytesWithType(privKeyBytes, &privKey)
	return
}

//----------------------------------------

type PrivKey interface {
	Bytes() []byte
	Sign(msg []byte) (Signature, error)
	PubKey() PubKey
	Equals(PrivKey) bool
}

//-------------------------------------

var _ PrivKey = PrivKeyEd25519{}

// Implements PrivKey
type PrivKeyEd25519 [64]byte

func (privKey PrivKeyEd25519) Bytes() []byte {
	return ser.MustEncodeToBytesWithType(privKey)
}

func (privKey PrivKeyEd25519) Sign(msg []byte) (Signature, error) {
	privKeyBytes := privKey[:]
	sigBytes := ed25519.Sign(privKeyBytes, msg)
	signatureBytes := new([SignatureEd25519Size]byte)
	copy(signatureBytes[:SignatureEd25519Size], sigBytes)
	return SignatureEd25519(*signatureBytes), nil
}

func (privKey PrivKeyEd25519) PubKey() PubKey {
	privKeyBytes := [64]byte(privKey)
	pub, _, err := ed25519.GenerateKey(bytes.NewReader(privKeyBytes[:]))
	if err != nil {
		panic(err)
	}
	newKey := new([PubKeyEd25519Size]byte)
	copy(newKey[:PubKeyEd25519Size], pub)
	return PubKeyEd25519(*newKey)
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the keys.
func (privKey PrivKeyEd25519) Equals(other PrivKey) bool {
	if otherEd, ok := other.(PrivKeyEd25519); ok {
		return subtle.ConstantTimeCompare(privKey[:], otherEd[:]) == 1
	} else {
		return false
	}
}

// Deterministically generates new priv-key bytes from key.
func (privKey PrivKeyEd25519) Generate(index int) PrivKeyEd25519 {
	bz, err := ser.EncodeToBytes(struct {
		PrivKey [64]byte
		Index   int
	}{privKey, index})
	if err != nil {
		panic(err)
	}
	newBytes := Sha256(bz)
	newKey := new([64]byte)
	copy(newKey[:32], newBytes)
	_, priv, err := ed25519.GenerateKey(bytes.NewReader(newKey[:]))
	if err != nil {
		panic(err)
	}
	copy(newKey[:64], priv)
	return PrivKeyEd25519(*newKey)
}

func (privKey PrivKeyEd25519) MarshalJSON() ([]byte, error) {
	return serEncodeFroJSON(privKey)
}

func (privKey *PrivKeyEd25519) UnmarshalJSON(input []byte) error {
	return serDecodeForJSON(privKey, input)
}

func GenPrivKeyEd25519() PrivKeyEd25519 {
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], CRandBytes(32))
	_, priv, err := ed25519.GenerateKey(bytes.NewReader(privKeyBytes[:]))
	if err != nil {
		panic(err)
	}
	copy(privKeyBytes[:64], priv)
	return PrivKeyEd25519(*privKeyBytes)
}

// NOTE: secret should be the output of a KDF like bcrypt,
// if it's derived from user input.
func GenPrivKeyEd25519FromSecret(secret []byte) PrivKeyEd25519 {
	privKey32 := Sha256(secret) // Not Ripemd160 because we want 32 bytes.
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], privKey32)
	_, priv, err := ed25519.GenerateKey(bytes.NewReader(privKeyBytes[:]))
	if err != nil {
		panic(err)
	}
	copy(privKeyBytes[:64], priv)
	return PrivKeyEd25519(*privKeyBytes)
}

//-------------------------------------

var _ PrivKey = PrivKeySecp256k1{}

// Implements PrivKey
type PrivKeySecp256k1 [32]byte

func (privKey PrivKeySecp256k1) Bytes() []byte {
	return ser.MustEncodeToBytesWithType(privKey)
}

func (privKey PrivKeySecp256k1) Sign(msg []byte) (Signature, error) {
	priv__, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey[:])
	sig__, err := priv__.Sign(Sha256(msg))
	if err != nil {
		return nil, err
	}
	return SignatureSecp256k1(sig__.Serialize()), nil
}

func (privKey PrivKeySecp256k1) PubKey() PubKey {
	_, pub__ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey[:])
	var pub PubKeySecp256k1
	copy(pub[:], pub__.SerializeCompressed())
	return pub
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the keys.
func (privKey PrivKeySecp256k1) Equals(other PrivKey) bool {
	if otherSecp, ok := other.(PrivKeySecp256k1); ok {
		return subtle.ConstantTimeCompare(privKey[:], otherSecp[:]) == 1
	} else {
		return false
	}
}

/*
// Deterministically generates new priv-key bytes from key.
func (key PrivKeySecp256k1) Generate(index int) PrivKeySecp256k1 {
	newBytes := cdc.BinarySha256(struct {
		PrivKey [64]byte
		Index   int
	}{key, index})
	var newKey [64]byte
	copy(newKey[:], newBytes)
	return PrivKeySecp256k1(newKey)
}
*/

func GenPrivKeySecp256k1() PrivKeySecp256k1 {
	privKeyBytes := [32]byte{}
	copy(privKeyBytes[:], CRandBytes(32))
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKeyBytes[:])
	copy(privKeyBytes[:], priv.Serialize())
	return PrivKeySecp256k1(privKeyBytes)
}

// NOTE: secret should be the output of a KDF like bcrypt,
// if it's derived from user input.
func GenPrivKeySecp256k1FromSecret(secret []byte) PrivKeySecp256k1 {
	privKey32 := Sha256(secret) // Not Ripemd160 because we want 32 bytes.
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey32)
	privKeyBytes := [32]byte{}
	copy(privKeyBytes[:], priv.Serialize())
	return PrivKeySecp256k1(privKeyBytes)
}
