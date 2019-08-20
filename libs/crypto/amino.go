package crypto

import (
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

func init() {
	// NOTE: It's important that there be no conflicts here,
	// as that would change the canonical representations,
	// and therefore change the address.
	// TODO: Add feature to go-amino to ensure that there
	// are no conflicts.
	RegisterAmino()
}

// RegisterAmino registers all crypto related types in the given (amino) codec.
func RegisterAmino() {
	ser.RegisterInterface((*PubKey)(nil), nil)
	ser.RegisterConcrete(PubKeyEd25519{}, "PubKeyEd25519", nil)
	ser.RegisterConcrete(PubKeySecp256k1{}, "PubKeySecp256k1", nil)

	ser.RegisterInterface((*PrivKey)(nil), nil)
	ser.RegisterConcrete(PrivKeyEd25519{}, "PrivKeyEd25519", nil)
	ser.RegisterConcrete(PrivKeySecp256k1{}, "PrivKeySecp256k1", nil)

	ser.RegisterInterface((*Signature)(nil), nil)
	ser.RegisterConcrete(SignatureEd25519{}, "SignEd25519", nil)
	ser.RegisterConcrete(SignatureSecp256k1{}, "SignSecp256k1", nil)
}
