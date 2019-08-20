package main

import (
	"bytes"
	"io/ioutil"

	"github.com/lianxiangcloud/linkchain/accounts/abi"
	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	"github.com/lianxiangcloud/linkchain/libs/common"
)

type ks struct {
	key string
	pwd string
}

var (
	cabi  abi.ABI
	ccode []byte
	kss   = []ks{
		ks{
			key: `{"address":"54fb1c7d0f011dd63b08f85ed7b518ab82028100","crypto":{"cipher":"aes-128-ctr","ciphertext":"e77ec15da9bdec5488ce40b07a860fb5383dffce6950defeb80f6fcad4916b3a","cipherparams":{"iv":"5df504a561d39675b0f9ebcbafe5098c"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":262144,"p":1,"r":8,"salt":"908cd3b189fc8ceba599382cf28c772b735fb598c7dbbc59ef0772d2b851f57f"},"mac":"9bb92ffd436f5248b73a641a26ae73c0a7d673bb700064f388b2be0f35fedabd"},"id":"2e15f180-b4f1-4d9c-b401-59eeeab36c87","version":3}`,
			pwd: `1234`,
		},
	}
	accounts []*keystore.Key
)

func init() {

	for _, k := range kss {
		key, err := keystore.DecryptKey([]byte(k.key), k.pwd)
		if err != nil {
			panic(err)
		}
		accounts = append(accounts, key)
	}

	abiBytes, err := ioutil.ReadFile("sol/SimpleToken.abi")
	if err != nil {
		panic(err)
	}

	cabi, err = abi.JSON(bytes.NewReader(abiBytes))
	if err != nil {
		panic(err)
	}

	bin, err := ioutil.ReadFile("sol/SimpleToken.bin")
	if err != nil {
		panic(err)
	}

	ccode = common.Hex2Bytes(string(bin))
}

func main() {}
