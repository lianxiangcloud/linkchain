package types

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/common"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenLoadValidator(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	privVal := GenFilePV(tempFilePath)

	height := uint64(100)
	privVal.LastHeight = height
	privVal.Save()
	addr := privVal.GetAddress()
	assert.Equal(uint64(0), privVal.pv.LastHeight, "expected privval.pv.LastHeight to be 0")

	privVal = LoadFilePV(tempFilePath)
	assert.Equal(addr, privVal.GetAddress(), "expected privval addr to be the same")
	assert.Equal(height, privVal.LastHeight, "expected privval.LastHeight to have been saved")
	assert.Equal(height, privVal.pv.LastHeight, "expected privval.pv.LastHeight to be the same")

}

func TestValidatorUpdate(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	if err := os.Remove(tempFilePath); err != nil {
		t.Error(err)
	}
	privVal := LoadOrGenFilePV(tempFilePath)
	oldPriv := privVal.GetPrikey()
	privKey := crypto.GenPrivKeyEd25519()
	privVal.UpdatePrikey(privKey)

	assert.EqualValues(privVal.GetPrikey(), privKey)
	assert.EqualValues(privVal.pv.GetPrikey(), oldPriv)

	block1 := BlockID{common.BytesToHash([]byte{1, 2, 3}), PartSetHeader{}}
	height, round, voteType := uint64(10), 1, VoteTypePrevote

	// sign a vote for first time
	vote := newVote(privVal.GetAddress(), 0, height, round, voteType, block1)
	err := privVal.SignVote("mychainid", vote)
	assert.NoError(err, "expected no error signing vote")

	privVal = LoadOrGenFilePV(tempFilePath)
	assert.EqualValues(privVal.GetPrikey(), oldPriv)
	assert.EqualValues(privVal.LastHeight, height)
	assert.EqualValues(privVal.LastRound, round)
}

func TestLoadOrGenValidator(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	if err := os.Remove(tempFilePath); err != nil {
		t.Error(err)
	}
	privVal := LoadOrGenFilePV(tempFilePath)
	addr := privVal.GetAddress()
	privVal = LoadOrGenFilePV(tempFilePath)
	assert.Equal(addr, privVal.GetAddress(), "expected privval addr to be the same")
}

func TestUnmarshalValidator(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// create some fixed values
	privKey := crypto.GenPrivKeyEd25519()
	pubKey := privKey.PubKey()
	addr := pubKey.Address()
	pubBytes := pubKey.Bytes()
	privBytes := privKey.Bytes()

	serialized := fmt.Sprintf(`{
		"address": "%s",
		"pub_key": {
			"type": "PubKeyEd25519",
			"value": "0x%x"
		},
		"last_height": "0",
		"last_round": "0",
		"last_step": 0,
		"priv_key": {
			"type": "PrivKeyEd25519",
			"value": "0x%x"
		}
	}`, addr, pubBytes, privBytes)

	val := FilePV{}
	err := ser.UnmarshalJSON([]byte(serialized), &val)
	require.Nil(err, "%+v", err)

	// make sure the values match
	assert.EqualValues(addr, val.GetAddress())
	assert.EqualValues(pubKey, val.GetPubKey())
	assert.EqualValues(privKey, val.PrivKey)

	// export it and make sure it is the same
	out, err := ser.MarshalJSON(val)
	require.Nil(err, "%+v", err)
	assert.JSONEq(serialized, string(out))
}

func TestSignVote(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	privVal := GenFilePV(tempFilePath)

	block1 := BlockID{common.BytesToHash([]byte{1, 2, 3}), PartSetHeader{}}
	block2 := BlockID{common.BytesToHash([]byte{3, 2, 1}), PartSetHeader{}}
	height, round := uint64(10), 1
	voteType := VoteTypePrevote

	// sign a vote for first time
	vote := newVote(privVal.Address, 0, height, round, voteType, block1)
	err := privVal.SignVote("mychainid", vote)
	assert.NoError(err, "expected no error signing vote")

	// try to sign the same vote again; should be fine
	err = privVal.SignVote("mychainid", vote)
	assert.NoError(err, "expected no error on signing same vote")

	// now try some bad votes
	cases := []*Vote{
		newVote(privVal.Address, 0, height, round-1, voteType, block1),   // round regression
		newVote(privVal.Address, 0, height-1, round, voteType, block1),   // height regression
		newVote(privVal.Address, 0, height-2, round+4, voteType, block1), // height regression and different round
		newVote(privVal.Address, 0, height, round, voteType, block2),     // different block
	}

	for _, c := range cases {
		err = privVal.SignVote("mychainid", c)
		assert.Error(err, "expected error on signing conflicting vote")
	}

	// try signing a vote with a different time stamp
	sig := vote.Signature
	vote.Timestamp = vote.Timestamp.Add(time.Duration(1000))
	err = privVal.SignVote("mychainid", vote)
	assert.NoError(err)
	assert.Equal(sig, vote.Signature)
}

func TestSignProposal(t *testing.T) {
	assert := assert.New(t)

	_, tempFilePath := cmn.Tempfile("priv_validator_")
	privVal := GenFilePV(tempFilePath)

	block1 := PartSetHeader{5, []byte{1, 2, 3}}
	block2 := PartSetHeader{10, []byte{3, 2, 1}}
	height, round := uint64(10), 1

	// sign a proposal for first time
	proposal := newProposal(height, round, block1)
	err := privVal.SignProposal("mychainid", proposal)
	assert.NoError(err, "expected no error signing proposal")

	// try to sign the same proposal again; should be fine
	err = privVal.SignProposal("mychainid", proposal)
	assert.NoError(err, "expected no error on signing same proposal")

	// now try some bad Proposals
	cases := []*Proposal{
		newProposal(height, round-1, block1),   // round regression
		newProposal(height-1, round, block1),   // height regression
		newProposal(height-2, round+4, block1), // height regression and different round
		newProposal(height, round, block2),     // different block
	}

	for _, c := range cases {
		err = privVal.SignProposal("mychainid", c)
		assert.Error(err, "expected error on signing conflicting proposal")
	}

	// try signing a proposal with a different time stamp
	sig := proposal.Signature
	proposal.Timestamp = proposal.Timestamp.Add(time.Duration(1000))
	err = privVal.SignProposal("mychainid", proposal)
	assert.NoError(err)
	assert.Equal(sig, proposal.Signature)
}

func TestDifferByTimestamp(t *testing.T) {
	_, tempFilePath := cmn.Tempfile("priv_validator_")
	privVal := GenFilePV(tempFilePath)

	block1 := PartSetHeader{5, []byte{1, 2, 3}}
	height, round := uint64(10), 1
	chainID := "mychainid"

	// test proposal
	{
		proposal := newProposal(height, round, block1)
		err := privVal.SignProposal(chainID, proposal)
		assert.NoError(t, err, "expected no error signing proposal")
		signBytes := proposal.SignBytes(chainID)
		sig := proposal.Signature
		timeStamp := clipToMS(proposal.Timestamp)

		// manipulate the timestamp. should get changed back
		proposal.Timestamp = proposal.Timestamp.Add(time.Millisecond)
		var emptySig crypto.Signature
		proposal.Signature = emptySig
		err = privVal.SignProposal("mychainid", proposal)
		assert.NoError(t, err, "expected no error on signing same proposal")

		assert.Equal(t, timeStamp, proposal.Timestamp)
		assert.Equal(t, signBytes, proposal.SignBytes(chainID))
		assert.Equal(t, sig, proposal.Signature)
	}

	// test vote
	{
		voteType := VoteTypePrevote
		blockID := BlockID{common.BytesToHash([]byte{1, 2, 3}), PartSetHeader{}}
		vote := newVote(privVal.Address, 0, height, round, voteType, blockID)
		err := privVal.SignVote("mychainid", vote)
		assert.NoError(t, err, "expected no error signing vote")

		signBytes := vote.SignBytes(chainID)
		sig := vote.Signature
		timeStamp := clipToMS(vote.Timestamp)

		// manipulate the timestamp. should get changed back
		vote.Timestamp = vote.Timestamp.Add(time.Millisecond)
		var emptySig crypto.Signature
		vote.Signature = emptySig
		err = privVal.SignVote("mychainid", vote)
		assert.NoError(t, err, "expected no error on signing same vote")

		assert.Equal(t, timeStamp, vote.Timestamp)
		assert.Equal(t, signBytes, vote.SignBytes(chainID))
		assert.Equal(t, sig, vote.Signature)
	}
}

func newVote(addr crypto.Address, idx int, height uint64, round int, typ byte, blockID BlockID) *Vote {
	return &Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           height,
		Round:            round,
		Type:             typ,
		Timestamp:        time.Now().UTC(),
		BlockID:          blockID,
	}
}

func newProposal(height uint64, round int, partsHeader PartSetHeader) *Proposal {
	return &Proposal{
		Height:           height,
		Round:            round,
		BlockPartsHeader: partsHeader,
		Timestamp:        time.Now().UTC(),
	}
}

func clipToMS(t time.Time) time.Time {
	nano := t.UnixNano()
	million := int64(1000000)
	nano = (nano / million) * million
	return time.Unix(0, nano).UTC()
}
