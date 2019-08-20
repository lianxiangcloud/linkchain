package xcrypto

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
)

const (
	testWords              = "sequence atlas unveil summon pebbles tuesday beer rudely snake rockets different fuselage woven tagged bested dented vegan hover rapid fawns obvious muppet randomly seasons randomly"
	testMainSpendSecretKey = "b0ef6bd527b9b23b9ceef70dc8b4cd1ee83ca14541964e764ad23f5151204f0f"
	testMainSpendPublicKey = "7d996b0f2db6dbb5f2a086211f2399a4a7479b2c911af307fdc3f7f61a88cb0e"
	testMainViewSecretKey  = "42ba20adb337e5eca797565be11c9adb0a8bef8c830bccc2df712535d3b8f608"
	testMainViewPublicKey  = "1c06bcac7082f73af10460b5f2849aded79374b2fbdaae5d9384b9b6514fddcb"
)

func TestWordsToBytes(t *testing.T) {
	sec, err := WordsToBytes(testWords)
	if err != nil {
		t.Fatalf("%s", err)
	}

	if !strings.EqualFold(testMainSpendSecretKey, hex.EncodeToString(sec[:])) {
		t.Fatalf("spend secret key not match: wanted=%s, goted=%s", testMainSpendSecretKey, hex.EncodeToString(sec[:]))
	} else {
		t.Logf("secret key: %s", hex.EncodeToString(sec[:]))
	}
}

func TestBytesToWords(t *testing.T) {
	secKey, _ := hex.DecodeString(testMainSpendSecretKey)

	var spendSecKey types.SecretKey
	copy(spendSecKey[:], secKey)
	words, err := BytesToWords(spendSecKey, "English")
	if err != nil {
		t.Fatalf("%s", err)
	}

	if !strings.EqualFold(testWords, words) {
		t.Fatalf("words not match: wanted=%s, goted=%s", testWords, words)
	} else {
		t.Logf("words: %s", words)
	}
}

func TestGenerateKeys(t *testing.T) {
	recoverKey, err := WordsToBytes(testWords)
	if err != nil {
		t.Fatalf("%s", err)
	}

	sk, pk := GenerateKeys(recoverKey)
	if !strings.EqualFold(hex.EncodeToString(sk[:]), testMainSpendSecretKey) {
		t.Fatalf("spend secret key not match: wanted=%s, goted=%s", testMainSpendSecretKey, hex.EncodeToString(sk[:]))
	}
	if !strings.EqualFold(hex.EncodeToString(pk[:]), testMainSpendPublicKey) {
		t.Fatalf("spend public key not match: wanted=%s, goted=%s", testMainSpendSecretKey, hex.EncodeToString(pk[:]))
	}
}
