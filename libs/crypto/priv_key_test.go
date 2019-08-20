package crypto

import (
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/stretchr/testify/assert"
)

func TestGeneratePrivKey(t *testing.T) {
	testPriv := GenPrivKeyEd25519()
	testGenerate := testPriv.Generate(1)
	signBytes := []byte("something to sign")
	pub := testGenerate.PubKey()
	sig, err := testGenerate.Sign(signBytes)
	assert.NoError(t, err)
	assert.True(t, pub.VerifyBytes(signBytes, sig))
}

func TestGenPrivKeyEd25519FromSecret(t *testing.T) {
	secret := []byte("hello")
	privBytes := common.FromHex("9e5e70a1b9af8fb8402cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824d16198cd553243dae7d8e421107d9887e270eb1cc6e4c072adea0f0442b65ace")
	expectPriv, err := PrivKeyFromBytes(privBytes)
	assert.Nil(t, err)

	pubBytes := common.FromHex("724c2517228e6aa0d16198cd553243dae7d8e421107d9887e270eb1cc6e4c072adea0f0442b65ace")
	expectPub, err := PubKeyFromBytes(pubBytes)
	assert.Nil(t, err)

	priv := GenPrivKeyEd25519FromSecret(secret)
	assert.True(t, priv.Equals(expectPriv))
	assert.True(t, priv.PubKey().Equals(expectPub))

	expectSig := common.FromHex("c2ecc12ed3ead6b840966ce54dfa5d3de320a1cc0af59655e49fe52674b74e33b31a8d8289bd300bb63486ee4a86164f25e388c1f496a75c4f101bc67bd81c70a1f2553a20ee68be07")
	sig, err := priv.Sign(secret)
	assert.Nil(t, err)
	assert.Equal(t, expectSig, sig.Bytes())

	assert.True(t, expectPub.VerifyBytes(secret, sig))
}

func BenchmarkEd25519VerifyBytes(b *testing.B) {
	secret := []byte("hello")
	priv := GenPrivKeyEd25519FromSecret(secret)
	sig, _ := priv.Sign(secret)
	pub := priv.PubKey()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pub.VerifyBytes(secret, sig)
	}
}

func BenchmarkEd25519Sign(b *testing.B) {
	secret := []byte("hello")
	priv := GenPrivKeyEd25519FromSecret(secret)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		priv.Sign(secret)
	}
}
