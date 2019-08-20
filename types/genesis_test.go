package types

import (
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/stretchr/testify/assert"
)

func TestGenesisBad(t *testing.T) {
	// test some bad ones from raw json
	testCases := [][]byte{
		[]byte{},                         // empty
		[]byte{1, 1, 1, 1, 1},            // junk
		[]byte(`{}`),                     // empty
		[]byte(`{"chain_id":"mychain"}`), // missing validators
		[]byte(`{"chain_id":"mychain","validators":[]}`),   // missing validators
		[]byte(`{"chain_id":"mychain","validators":[{}]}`), // missing validators
		[]byte(`{"chain_id":"mychain","validators":null}`), // missing validators
		[]byte(`{"chain_id":"mychain"}`),                   // missing validators
		[]byte(`{"validators":[{"pub_key":{"type":"PubKeyEd25519","value":"0x724c2517228e6aa0fff68bf4020740055e2706df23da4fc9089f40b0b4452b721a95248adb1cc291"},"power":"10","name":""}]}`), // missing chain_id
	}

	for _, testCase := range testCases {
		_, err := GenesisDocFromJSON(testCase)
		assert.Error(t, err, "expected error for empty genDoc json")
	}
}

func TestGenesisGood(t *testing.T) {
	// test a good one by raw json
	genDocBytes := []byte(`{"genesis_time":"0001-01-01T00:00:00Z","chain_id":"test-chain-QDKdJr","consensus_params":null,"validators":[{"pub_key":{"type":"PubKeyEd25519","value":"0x724c2517228e6aa0fff68bf4020740055e2706df23da4fc9089f40b0b4452b721a95248adb1cc291"},"power":"10","name":""}],"app_hash":"","app_state":{"account_owner": "Bob"}}`)
	_, err := GenesisDocFromJSON(genDocBytes)
	assert.NoError(t, err, "expected no error for good genDoc json")

	// create a base gendoc from struct
	baseGenDoc := &GenesisDoc{
		ChainID:    "abc",
		Validators: []GenesisValidator{{crypto.GenPrivKeyEd25519().PubKey(), common.EmptyAddress, 10, "myval"}},
	}
	genDocBytes, err = ser.MarshalJSON(baseGenDoc)
	assert.NoError(t, err, "error marshalling genDoc")

	// test base gendoc and check consensus params were filled
	genDoc, err := GenesisDocFromJSON(genDocBytes)
	assert.NoError(t, err, "expected no error for valid genDoc json")
	assert.NotNil(t, genDoc.ConsensusParams, "expected consensus params to be filled in")

	// create json with consensus params filled
	genDocBytes, err = ser.MarshalJSON(genDoc)
	assert.NoError(t, err, "error marshalling genDoc")
	genDoc, err = GenesisDocFromJSON(genDocBytes)
	assert.NoError(t, err, "expected no error for valid genDoc json")

	// test with invalid consensus params
	genDoc.ConsensusParams.BlockSize.MaxBytes = 0
	genDocBytes, err = ser.MarshalJSON(genDoc)
	assert.NoError(t, err, "error marshalling genDoc")
	genDoc, err = GenesisDocFromJSON(genDocBytes)
	assert.Error(t, err, "expected error for genDoc json with block size of 0")
}
