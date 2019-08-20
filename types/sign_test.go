package types

import (
	"bytes"
	"crypto/ecdsa"
	"math/big"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

type ttxdata struct {
	Name string
	Bal  *big.Int
	signdata
}

type TTx struct {
	data ttxdata
	// caches
	hash atomic.Value
	size atomic.Value
}

func (tx *TTx) signFields() []interface{} {
	return []interface{}{tx.data.Name, tx.data.Bal}
}

func (tx *TTx) Hash() (h common.Hash) {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}

	hashFields := append(tx.signFields(), tx.data.signdata)
	v := rlpHash(hashFields)
	tx.hash.Store(v)
	return v
}

func (tx *TTx) Sign(signer STDSigner, prv *ecdsa.PrivateKey) error {
	tx.data.setSignFieldsFunc(tx.signFields)
	r, s, v, err := sign(signer, prv, tx.data.signFields())
	if err != nil {
		return err
	}
	cpy := &TTx{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	*tx = *cpy
	return nil
}

func (tx *TTx) Sender(signer STDSigner) (common.Address, error) {
	tx.data.setSignFieldsFunc(tx.signFields)
	return sender(signer, &tx.data)
}

func TestSignAndSender(t *testing.T) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	signer := NewSTDEIP155Signer(SignParam)
	tx := &TTx{
		data: ttxdata{
			Name: "ab",
			Bal:  big.NewInt(1),
		},
	}
	hashBefore := tx.Hash()
	err := tx.Sign(signer, key)
	if err != nil {
		t.Errorf("tx.Sign err %v", err)
	}

	hashAfter := tx.Hash()
	if hashBefore.Hex() == hashAfter.Hex() {
		t.Errorf("tx hash equal")
	}

	from, err := tx.Sender(signer)
	if err != nil {
		t.Errorf("tx.Sender err %v", err)
	}
	if from != addr {
		t.Errorf("expected %x got %x", addr, from)
	}

	tx = &TTx{
		data: ttxdata{
			Name: "ab",
			Bal:  big.NewInt(12),
		},
	}

	err = tx.Sign(signer, key)
	if err != nil {
		t.Errorf("tx.Sign err %v", err)
	}

	tx.data.Bal = big.NewInt(2)
	from, err = tx.Sender(signer)
	if err != nil {
		t.Errorf("tx.Sender err %v", err)
	}
	if from == addr {
		t.Errorf("expected not %x got %x", addr, from)
	}
}

func TestTxSign(t *testing.T) {
	var txs = []map[string]string{
		map[string]string{
			"type": "tx",
			"from": "0xc52935e8ccecfbc860a7c003fbc7459f67ee8670",
			"to":   "0xeabc7a5cac7dd15069b5aa834191b28bf38504d8",
			"hash": "0x887e85748400d652b819ecd6f8eaae708a781a873032d3bcfd3244711a33ca5d",
			"raw":  "0xf8678085174876e800830186a094eabc7a5cac7dd15069b5aa834191b28bf38504d8808082ec8da0743c98dc09528902783fb4315d42ab990e596d797e4aadfb7c0e4565d4bd6cc1a030723c340fb5fdb794b38082aca900359660ba02a37722a0a7f9c65651a8b679",
		},
		map[string]string{
			"type": "tx",
			"from": "0xe177369d6bec84ca3cdc7ae38dc6582322454691",
			"to":   "0x9851d2a7f9a2a42db3f0db6e55b67cd623d461ab",
			"hash": "0x96bd1c5850926d9ba788108be012f2dac68ff3a58edfb97b4bbf656d14dc9e2e",
			"raw":  "0xf8678085174876e800830186a0949851d2a7f9a2a42db3f0db6e55b67cd623d461ab808082ec8ea06bfbc6c00a95d8da7e109d347608ed16a57434f9d9af3a3d02c60eb172ff0b0ba007e5b951fa33defefcbf0c2adc39a55fd3da98cc4af3e1a48119532c91765f89",
		},
		map[string]string{
			"type": "tx",
			"from": "0x5907174292089e58092ea9909c4883e18968651e",
			"to":   "0x6fb94c5fbadb476a2486e41073a84393fe844aeb",
			"hash": "0x8cdbbfc8b7620dc9c55cc456f732153efd26714a90821940c9f35e133b6aecee",
			"raw":  "0xf8678085174876e800830186a0946fb94c5fbadb476a2486e41073a84393fe844aeb808082ec8ea0217797fd5ca883c9e0748d9f9dddcb8f0940ee9c8044b5fc919cfec2ce547fa7a04d7aedc135f9dc360253866ac35e0d0136212016864fd4142d08a04fc2ebf466",
		},
	}

	signer := MakeSTDSigner(big.NewInt(30261))

	var from common.Address
	var err error
	for _, txm := range txs {
		tx := new(Transaction)
		if err = ser.DecodeBytes(common.FromHex(txm["raw"]), tx); err != nil {
			t.Fatalf("ser.DecodeBytes err:%v", err)
		}

		if from, err = tx.Sender(signer); err != nil {
			t.Fatalf("tx.From:%v", err)
		}
		bs, err := ser.EncodeToBytes(tx)
		if err != nil {
			t.Fatalf("ser.EncodeToBytes err:%v", err)
		}

		assert.Equal(t, from.Hex(), txm["from"], "from not equal")
		assert.Equal(t, tx.To().Hex(), txm["to"], "to not equal")
		assert.Equal(t, tx.Hash().Hex(), txm["hash"], "hash not equal")
		assert.Equal(t, common.ToHex(bs), txm["raw"], "raw not equal")

		tx = new(Transaction)
		s := ser.NewStream(bytes.NewReader(common.FromHex(txm["raw"])), 0)
		if err = tx.DecodeSER(s); err != nil {
			t.Fatalf("ser.DecodeBytes err:%v", err)
		}

		if from, err = tx.Sender(signer); err != nil {
			t.Fatalf("tx.From:%v", err)
		}
		bf := bytes.NewBuffer(nil)
		err = tx.EncodeSER(bf)
		if err != nil {
			t.Fatalf("ser.EncodeToBytes err:%v", err)
		}
		bs = bf.Bytes()

		assert.Equal(t, from.Hex(), txm["from"], "from not equal")
		assert.Equal(t, tx.To().Hex(), txm["to"], "to not equal")
		assert.Equal(t, tx.Hash().Hex(), txm["hash"], "hash not equal")
		assert.Equal(t, common.ToHex(bs), txm["raw"], "raw not equal")
	}
}
