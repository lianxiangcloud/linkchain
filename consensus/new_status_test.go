package consensus

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/common"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	crypto "github.com/lianxiangcloud/linkchain/libs/crypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cfg "github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/types"
)

// setupTestCase does setup common to all test cases
func setupTestCase(t *testing.T) (func(t *testing.T), dbm.DB, NewStatus) {
	config := cfg.ResetTestRoot("state_")
	dbType := dbm.DBBackendType(config.DBBackend)
	stateDB := dbm.NewDB("state", dbType, config.DBDir(), config.DBCounts)
	state, err := CreateStatusFromGenesisFile(stateDB, config.GenesisFile())
	assert.NoError(t, err, "expected no error on CreateStatusFromGenesisFile")

	tearDown := func(t *testing.T) {}

	return tearDown, stateDB, state
}

// TestStateCopy tests the correct copying behaviour of State.
func TestStateCopy(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)
	// nolint: vetshadow
	assert := assert.New(t)

	stateCopy := state.Copy()

	assert.True(state.Equals(stateCopy),
		cmn.Fmt("expected state and its copy to be identical.\ngot: %v\nexpected: %v\n",
			stateCopy, state))

	stateCopy.LastBlockHeight++
	assert.False(state.Equals(stateCopy), cmn.Fmt(`expected states to be different. got same
        %v`, state))
}

// TestStateSaveLoad tests saving and loading State from a db.
func TestStateSaveLoad(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)
	// nolint: vetshadow
	assert := assert.New(t)

	state.LastBlockHeight++
	SaveStatus(stateDB, state)

	loadedState, _ := LoadStatus(stateDB)
	assert.True(state.Equals(loadedState),
		cmn.Fmt("expected state and its copy to be identical.\ngot: %v\nexpected: %v\n",
			loadedState, state))
}

// TestValidatorSimpleSaveLoad tests saving and loading validators.
func TestValidatorSimpleSaveLoad(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)
	// nolint: vetshadow
	assert := assert.New(t)

	// can't load anything for height 0
	v, _, err := LoadValidators(stateDB, 0)
	assert.IsType(ErrNoValSetForHeight{}, err, "expected err at height 0")

	// should be able to load for height 1
	v, _, err = LoadValidators(stateDB, 1)
	assert.Nil(err, "expected no err at height 1")
	assert.Equal(v.Hash(), state.Validators.Hash(), "expected validator hashes to match")

	// increment height, save; should be able to load for next height
	state.LastBlockHeight++
	nextHeight := state.LastBlockHeight + 1
	saveValidatorsInfo(stateDB, nextHeight, state.LastHeightValidatorsChanged, state.Validators)
	v, _, err = LoadValidators(stateDB, nextHeight)
	assert.Nil(err, "expected no err")
	assert.Equal(v.Hash(), state.Validators.Hash(), "expected validator hashes to match")

	// increment height, save; should be able to load for next height
	state.LastBlockHeight += 10
	nextHeight = state.LastBlockHeight + 1
	saveValidatorsInfo(stateDB, nextHeight, state.LastHeightValidatorsChanged, state.Validators)
	v, _, err = LoadValidators(stateDB, nextHeight)
	assert.Nil(err, "expected no err")
	assert.Equal(v.Hash(), state.Validators.Hash(), "expected validator hashes to match")

	// should be able to load for next next height
	_, _, err = LoadValidators(stateDB, state.LastBlockHeight+2)
	assert.IsType(ErrNoValSetForHeight{}, err, "expected err at unknown height")
}

// TestValidatorChangesSaveLoad tests saving and loading a validator set with changes.
func TestOneValidatorChangesSaveLoad(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)

	// change vals at these heights
	changeHeights := []uint64{1, 2, 4, 5, 10, 15, 16, 17, 20}
	N := len(changeHeights)

	// build the validator history by running updateState
	// with the right validator set for each height
	highestHeight := changeHeights[N-1] + 5
	changeIndex := 0
	_, val := state.Validators.GetByIndex(0)
	power := val.VotingPower
	var err error
	for i := uint64(1); i < highestHeight; i++ {
		// when we get to a change height,
		// use the next pubkey
		if changeIndex < len(changeHeights) && i == changeHeights[changeIndex] {
			changeIndex++
			power++
		}
		header, blockID, responses := makeHeaderPartsResponsesValPowerChange(state, i, power)
		state, err = updateStatus(state, blockID, header, responses)
		assert.Nil(t, err)
		nextHeight := state.LastBlockHeight + 1
		saveValidatorsInfo(stateDB, nextHeight, state.LastHeightValidatorsChanged, state.Validators)
	}

	// on each change height, increment the power by one.
	testCases := make([]int64, highestHeight)
	changeIndex = 0
	power = val.VotingPower
	for i := uint64(1); i < highestHeight+1; i++ {
		// we we get to the height after a change height
		// use the next pubkey (note our counter starts at 0 this time)
		if changeIndex < len(changeHeights) && i == changeHeights[changeIndex]+1 {
			changeIndex++
			power++
		}
		testCases[i-1] = power
	}

	for i, power := range testCases {
		v, _, err := LoadValidators(stateDB, uint64(i+1))
		assert.Nil(t, err, fmt.Sprintf("expected no err at height %d", i))
		assert.Equal(t, v.Size(), 1, "validator set size is greater than 1: %d", v.Size())
		_, val := v.GetByIndex(0)

		assert.Equal(t, val.VotingPower, power, fmt.Sprintf(`unexpected powerat
                height %d`, i))
	}
}

// TestValidatorChangesSaveLoad tests saving and loading a validator set with
// changes.
func TestManyValidatorChangesSaveLoad(t *testing.T) {
	const valSetSize = 7
	tearDown, stateDB, state := setupTestCase(t)
	state.Validators = genValSet(valSetSize)
	SaveStatus(stateDB, state)
	defer tearDown(t)
	const height = 1
	pubkey := crypto.GenPrivKeyEd25519().PubKey()
	// swap the first validator with a new one ^^^ (validator set size stays the same)
	header, blockID, responses := makeHeaderPartsResponsesValPubKeyChange(state, height, pubkey)
	var err error
	state, err = updateStatus(state, blockID, header, responses)
	require.Nil(t, err)
	nextHeight := state.LastBlockHeight + 1
	saveValidatorsInfo(stateDB, nextHeight, state.LastHeightValidatorsChanged, state.Validators)

	v, _, err := LoadValidators(stateDB, height+1)
	assert.Nil(t, err)
	assert.Equal(t, valSetSize, v.Size())
	index, val := v.GetByAddress(pubkey.Address())
	assert.NotNil(t, val)
	if index < 0 {
		t.Fatal("expected to find newly added validator")
	}
}

func genValSet(size int) *types.ValidatorSet {
	vals := make([]*types.Validator, size)
	for i := 0; i < size; i++ {
		vals[i] = types.NewValidator(crypto.GenPrivKeyEd25519().PubKey(), common.EmptyAddress, 10)
	}
	return types.NewValidatorSet(vals)
}

func pk() []byte {
	return crypto.GenPrivKeyEd25519().PubKey().Bytes()
}

func makeHeaderPartsResponsesValPubKeyChange(state NewStatus, height uint64,
	pubkey crypto.PubKey) (*types.Header, types.BlockID, []*types.Validator) {

	block := makeBlock(state, height)
	validators := make([]*types.Validator, len(state.Validators.Validators))
	for i, val := range state.Validators.Validators {
		// NOTE: must copy, since IncrementAccum updates in place.
		validators[i] = val.Copy()
	}
	if len(validators) > 0 {
		if !bytes.Equal(pubkey.Bytes(), validators[0].PubKey.Bytes()) {
			validators[0].PubKey = pubkey
			validators[0].Address = pubkey.Address()
		}
	}

	return block.Header, types.BlockID{block.Hash(), types.PartSetHeader{}}, validators
}

func makeHeaderPartsResponsesValPowerChange(state NewStatus, height uint64,
	power int64) (*types.Header, types.BlockID, []*types.Validator) {

	block := makeBlock(state, height)

	validators := make([]*types.Validator, len(state.Validators.Validators))
	for i, val := range state.Validators.Validators {
		// NOTE: must copy, since IncrementAccum updates in place.
		validators[i] = val.Copy()
	}

	if len(validators) > 0 {
		if validators[0].VotingPower != power {
			validators[0].VotingPower = power
		}
	}
	return block.Header, types.BlockID{block.Hash(), types.PartSetHeader{}}, validators
}
