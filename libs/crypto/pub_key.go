package crypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/ripemd160"

	secp256k1 "github.com/btcsuite/btcd/btcec"
	"golang.org/x/crypto/ed25519"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

// An address is a []byte, but hex-encoded even in JSON.
// []byte leaves us the option to change the address length.
// Use an alias so Unmarshal methods (with ptr receivers) are available too.
type Address = cmn.HexBytes

func PubKeyFromBytes(pubKeyBytes []byte) (pubKey PubKey, err error) {
	err = ser.DecodeBytesWithType(pubKeyBytes, &pubKey)
	return
}

//----------------------------------------

type PubKey interface {
	Address() Address
	Bytes() []byte
	VerifyBytes(msg []byte, sig Signature) bool
	Equals(PubKey) bool
}

//-------------------------------------

var _ PubKey = PubKeyEd25519{}

const PubKeyEd25519Size = 32

// Implements PubKeyInner
type PubKeyEd25519 [PubKeyEd25519Size]byte

// Address is the Ripemd160 of the raw pubkey bytes.
func (pubKey PubKeyEd25519) Address() Address {
	return Address(Ripemd160(pubKey[:]))
}

func (pubKey PubKeyEd25519) Bytes() []byte {
	bz, err := ser.EncodeToBytesWithType(pubKey)
	if err != nil {
		panic(err)
	}
	return bz
}

func serEncodeFroJSON(v interface{}) ([]byte, error) {
	b, err := ser.EncodeToBytesWithType(v)
	if err != nil {
		return nil, err
	}
	enc := make([]byte, len(b)*2+4)
	copy(enc, `"0x`)
	hex.Encode(enc[3:], b)
	enc[len(enc)-1] = '"'
	return enc, err
}

func serDecodeForJSON(v interface{}, input []byte) error {
	if len(input) < 4 {
		return fmt.Errorf("%s is not a hex string", input)
	}
	input = input[3 : len(input)-1]
	dec := make([]byte, len(input)/2)
	if _, err := hex.Decode(dec, input); err != nil {
		return err
	}
	return ser.DecodeBytesWithType(dec, v)
}

func (pubKey PubKeyEd25519) MarshalJSON() ([]byte, error) {
	return serEncodeFroJSON(pubKey)
}

func (pubKey *PubKeyEd25519) UnmarshalJSON(input []byte) error {
	return serDecodeForJSON(pubKey, input)
}

func (pubKey PubKeyEd25519) VerifyBytes(msg []byte, sig_ Signature) bool {
	// make sure we use the same algorithm to sign
	sig, ok := sig_.(SignatureEd25519)
	if !ok {
		return false
	}
	pubKeyBytes := [PubKeyEd25519Size]byte(pubKey)
	sigBytes := [SignatureEd25519Size]byte(sig)
	return ed25519.Verify(pubKeyBytes[:], msg, sigBytes[:])
}

func (pubKey PubKeyEd25519) String() string {
	return fmt.Sprintf("PubKeyEd25519{%v}", hexutil.Encode(pubKey.Bytes()))
}

func (pubKey PubKeyEd25519) Equals(other PubKey) bool {
	if otherEd, ok := other.(PubKeyEd25519); ok {
		return bytes.Equal(pubKey[:], otherEd[:])
	} else {
		return false
	}
}

//-------------------------------------

var _ PubKey = PubKeySecp256k1{}

const PubKeySecp256k1Size = 33

// Implements PubKey.
// Compressed pubkey (just the x-cord),
// prefixed with 0x02 or 0x03, depending on the y-cord.
type PubKeySecp256k1 [PubKeySecp256k1Size]byte

// Implements Bitcoin style addresses: RIPEMD160(SHA256(pubkey))
func (pubKey PubKeySecp256k1) Address() Address {
	hasherSHA256 := sha256.New()
	hasherSHA256.Write(pubKey[:]) // does not error
	sha := hasherSHA256.Sum(nil)

	hasherRIPEMD160 := ripemd160.New()
	hasherRIPEMD160.Write(sha) // does not error
	return Address(hasherRIPEMD160.Sum(nil))
}

func (pubKey PubKeySecp256k1) Bytes() []byte {
	bz, err := ser.EncodeToBytesWithType(pubKey)
	if err != nil {
		panic(err)
	}
	return bz
}

func (pubKey PubKeySecp256k1) VerifyBytes(msg []byte, sig_ Signature) bool {
	// and assert same algorithm to sign and verify
	sig, ok := sig_.(SignatureSecp256k1)
	if !ok {
		return false
	}

	pub__, err := secp256k1.ParsePubKey(pubKey[:], secp256k1.S256())
	if err != nil {
		return false
	}
	sig__, err := secp256k1.ParseDERSignature(sig[:], secp256k1.S256())
	if err != nil {
		return false
	}
	return sig__.Verify(Sha256(msg), pub__)
}

func (pubKey PubKeySecp256k1) String() string {
	return fmt.Sprintf("PubKeySecp256k1{%X}", pubKey[:])
}

func (pubKey PubKeySecp256k1) Equals(other PubKey) bool {
	if otherSecp, ok := other.(PubKeySecp256k1); ok {
		return bytes.Equal(pubKey[:], otherSecp[:])
	} else {
		return false
	}
}
