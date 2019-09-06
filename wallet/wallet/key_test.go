package wallet

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	lktypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
)

var (
	keyTests = make(map[string][]KeyTest)
)

type KeyTest struct {
	val           interface{}
	output, error string
}

func runKeyTests(t *testing.T, id string, f func(val interface{}) ([]byte, error)) {
	if tests, exist := keyTests[id]; exist {
		for i, test := range tests {
			output, err := f(test.val)
			if err != nil && test.error == "" {
				t.Errorf("test %s-%d: unexpected error: %v\nvalue %#v\ntype %T",
					id, i, err, test.val, test.val)
				continue
			}
			if test.error != "" && fmt.Sprint(err) != test.error {
				t.Errorf("test %s-%d: error mismatch\ngot   %v\nwant  %v\nvalue %#v\ntype  %T",
					id, i, err, test.error, test.val, test.val)
				continue
			}
			b, err := hex.DecodeString(strings.Replace(test.output, " ", "", -1))
			if err != nil {
				panic(fmt.Sprintf("invalid hex string: %q", test.output))
			}
			if err == nil && !bytes.Equal(output, b) {
				t.Errorf("test %s-%d: output mismatch:\ngot   %X\nwant  %s\nvalue %#v\ntype  %T",
					id, i, output, test.output, test.val, test.val)
			}
		}
	}
}

func TestKeyFromAccount(t *testing.T) {
	type keyTest struct {
		Keystore string
		Passwd   string
	}
	keystore := `{"address":"622bc0938fae8b028fcf124f9ba8580719009fdc","crypto":{"cipher":"aes-128-ctr","ciphertext":"442cbf32faee334188b050225e51c82fce94a0cb5870c5bbedcabbdc5b4e0828","cipherparams":{"iv":"d18fed89eae428c55b8b5f2cde1b56c7"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":262144,"p":1,"r":8,"salt":"9b3e1afa3b16648fa2814d3a5e77f30ff86180570237bbeada150236291a2c29"},"mac":"36ce132aceffae6c751fd2572685546c630c7f05dde5004608871645a3b9578b"},"id":"070808d2-957a-4c2c-b856-b6c0d09ec129","version":3}`
	passwd := "12345678"
	keyTests["TestKeyFromAccount"] = []KeyTest{
		{
			val: keyTest{
				Keystore: keystore,
				Passwd:   passwd,
			},
			output: "4D4CA6830D1378F7923A5116174657CD79FB312F881FCD557A495A8202CA08B1",
			error:  "",
		},
	}
	runKeyTests(t, "TestKeyFromAccount", func(val interface{}) ([]byte, error) {
		keytest := (val.(keyTest))
		key, err := KeyFromAccount([]byte(keytest.Keystore), keytest.Passwd)
		return []byte(key[:]), err
	})
}

func TestStrToAddress(t *testing.T) {
	keyTests["TestStrToAddress"] = []KeyTest{
		{
			val:    "byF93XiVh8tP7CVsDS1Jt91sgCkhWzRBrqQ1UaygKuYE4pXM8HxnLMEXz2H9PdjFzqX7ozBJ6i2exvJdsMoKsU9zoMTG9V",
			output: "8F138423C4219965128858A11ACA4A122B82D47C15BCA6E1506A22A1C14B3856",
			error:  "",
		},
	}
	runKeyTests(t, "TestStrToAddress", func(val interface{}) ([]byte, error) {
		str := (val.(string))
		addr, err := StrToAddress(str)
		if err != nil {
			return nil, err
		}
		return []byte(addr.SpendPublicKey[:]), err
	})
}

func TestWordsToAccount(t *testing.T) {
	keyTests["TestWordsToAccount"] = []KeyTest{
		{
			val:    "sequence atlas unveil summon pebbles tuesday beer rudely snake rockets different fuselage woven tagged bested dented vegan hover rapid fawns obvious muppet randomly seasons randomly",
			output: "42BA20ADB337E5ECA797565BE11C9ADB0A8BEF8C830BCCC2DF712535D3B8F608",
			error:  "",
		},
	}
	runKeyTests(t, "TestWordsToAccount", func(val interface{}) ([]byte, error) {
		str := (val.(string))
		acc, err := WordsToAccount(str)
		if err != nil {
			return nil, nil
		}
		return []byte(acc.GetKeys().ViewSKey[:]), nil
	})
}

func TestWordsToKey(t *testing.T) {
	keyTests["TestWordsToKey"] = []KeyTest{
		{
			val:    "sequence atlas unveil summon pebbles tuesday beer rudely snake rockets different fuselage woven tagged bested dented vegan hover rapid fawns obvious muppet randomly seasons randomly",
			output: "B0EF6BD527B9B23B9CEEF70DC8B4CD1EE83CA14541964E764AD23F5151204F0F",
			error:  "",
		},
	}
	runKeyTests(t, "TestWordsToKey", func(val interface{}) ([]byte, error) {
		str := (val.(string))
		key, err := WordsToKey(str)
		if err != nil {
			return nil, nil
		}
		return []byte(key[:]), nil
	})
}

func TestKeyToWords(t *testing.T) {
	output := []byte("sequence atlas unveil summon pebbles tuesday beer rudely snake rockets different fuselage woven tagged bested dented vegan hover rapid fawns obvious muppet randomly seasons randomly")
	keyTests["TestKeyToWords"] = []KeyTest{
		{
			val:    "B0EF6BD527B9B23B9CEEF70DC8B4CD1EE83CA14541964E764AD23F5151204F0F",
			output: fmt.Sprintf("%x", output),
			error:  "",
		},
	}
	runKeyTests(t, "TestKeyToWords_NotExec", func(val interface{}) ([]byte, error) {
		str := (val.(string))
		b, err := hex.DecodeString(str)
		if err != nil {
			return nil, err
		}
		var key lktypes.SecretKey
		copy(key[:], b)
		words, err := KeyToWords(key)
		if err != nil {
			return nil, nil
		}
		return []byte(words), nil
	})
}

func TestGetSubaddr(t *testing.T) {
	addr := []byte("oHXav7gNves6vewvdMoBtwd3nNeM1sNBBQUdSggUVVB2XB7vg7JUXjntBpGx5LEkahq9yzRb25UuymoW8oYmnycV2hgYsb")
	keyTests["TestGetSubaddr"] = []KeyTest{
		{
			val:    "sequence atlas unveil summon pebbles tuesday beer rudely snake rockets different fuselage woven tagged bested dented vegan hover rapid fawns obvious muppet randomly seasons randomly",
			output: fmt.Sprintf("%x", addr),
			error:  "",
		},
	}
	runKeyTests(t, "TestGetSubaddr", func(val interface{}) ([]byte, error) {
		str := (val.(string))
		acc, err := WordsToAccount(str)
		if err != nil {
			return nil, err
		}
		addr := GetSubaddr(acc.GetKeys(), 1)
		return []byte(addr), nil
	})
}
