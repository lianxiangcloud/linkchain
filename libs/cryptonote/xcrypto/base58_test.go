package xcrypto

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

const (
	testHexAddr    = "3f2771523a6a4b83f25cb65192b8db29b4d68b6f7a5ae424f09bd312e3ab123c91e23f3e2da4093f82ef9e43c07602f9efca833063f0597ba3a1a3b3cdb28401b40d906bc8"
	testBase58Addr = "BZg53n1EgLJhYDZNCi3VvxXFMdmmgk6HhhFCvvw9sMf1RQFp7LyjGvrNuF7TzukfaGh7Gsin2bEDpUNRv9oc8qSGMKCnktw"

	testEncodeAddrRet="jesAdEtDede9u4zMGNadXoAQDeBFbPBsj9u2qmQWEQ9X9QEkzM9YykmA6jzLrorwKZ9Q3oa4B7QwC9EGj6ysnQem9bXPmCPS4NH9bbjLWkjVrG9bdyi3CfcwW9jQvd9kpR2HJ6hmJcQn2oC9ZbGmPWbphnAGJEG57ogZgHRfsNAnsgYs94Xq7kM8XZ1A6baFa4yVpE"
	tag = uint64(0xff)
)

func TestBase58Encode(t *testing.T) {
	s, _ := hex.DecodeString(testHexAddr)
	addr := Base58Encode(s)
	if !strings.EqualFold(addr, testBase58Addr) {
		t.Fatalf("addr not match: wanted=%s, goted=%s", testBase58Addr, addr)
	} else {
		t.Logf("base58 encode addr: %s", addr)
	}
}

func TestBase58Decode(t *testing.T) {
	addr := Base58Decode(testBase58Addr)
	if addr == nil {
		t.Fatalf("Base58Decode fail")
	}
	if !strings.EqualFold(hex.EncodeToString(addr), testHexAddr) {
		t.Fatalf("addr not match: wanted=%s, goted=%s", testHexAddr, hex.EncodeToString(addr))
	} else {
		t.Logf("base58 decode addr: %s, hex: %s", string(addr), hex.EncodeToString(addr))
	}
}
func TestBase58EncodeAddr(t *testing.T) {
	addr := Base58EncodeAddr(tag,testHexAddr)
	if !strings.EqualFold(addr, testEncodeAddrRet) {
		t.Fatalf("addr not match: wanted=%s, goted=%s", testBase58Addr, addr)
	}
}

func TestBase58DecodeAddr(t *testing.T) {
	tag1 := tag
	dbyte := Base58DecodeAddr(&tag1,testEncodeAddrRet)
	fmt.Printf("%v %v\n",string(dbyte),tag1)
	if !strings.EqualFold(string(dbyte), testHexAddr) {
		t.Fatalf("addr not match: wanted=%s, goted=%s", testHexAddr, hex.EncodeToString(dbyte))
	}
}