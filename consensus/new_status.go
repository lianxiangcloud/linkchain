package consensus

import (
	"bytes"
	"encoding/binary"
	"fmt"

	//"io/ioutil"
	"time"

	//"github.com/lianxiangcloud/linkchain/libs/common"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
)

// NewStatus is a short description of the latest committed block of the consensus.
// It keeps all information necessary to validate new blocks,
// including the last validator set and the consensus params.
// All fields are exposed so the struct can be easily serialized,
// but none of them should be mutated directly.
// NOTE: not goroutine-safe.
type NewStatus struct {
	// Immutable
	ChainID string

	// LastBlockHeight=0 at genesis (ie. block(H=0) does not exist)
	LastBlockHeight  uint64
	LastBlockTotalTx uint64
	LastBlockID      types.BlockID
	LastBlockTime    uint64

	// LastValidators is used to validate block.LastCommit.
	// Validators are persisted to the database separately every time they change,
	// so we can query for historical validator sets.
	// Note that if s.LastBlockHeight causes a valset change,
	// we set s.LastHeightValidatorsChanged = s.LastBlockHeight + 1
	Validators                  *types.ValidatorSet
	LastValidators              *types.ValidatorSet
	LastHeightValidatorsChanged uint64

	//Indicate lastBlock is recoverBlock
	LastRecover bool

	// Consensus parameters used for validating blocks.
	// Changes returned by EndBlock and updated after Commit.
	ConsensusParams                  types.ConsensusParams
	LastHeightConsensusParamsChanged uint64
}

// Copy makes a copy of the NewStatus for mutating.
func (status NewStatus) Copy() NewStatus {
	return NewStatus{
		ChainID: status.ChainID,

		LastBlockHeight:  status.LastBlockHeight,
		LastBlockTotalTx: status.LastBlockTotalTx,
		LastBlockID:      status.LastBlockID,
		LastBlockTime:    status.LastBlockTime,
		LastRecover:      status.LastRecover,

		Validators:                  status.Validators.Copy(),
		LastValidators:              status.LastValidators.Copy(),
		LastHeightValidatorsChanged: status.LastHeightValidatorsChanged,

		ConsensusParams:                  status.ConsensusParams,
		LastHeightConsensusParamsChanged: status.LastHeightConsensusParamsChanged,
	}
}

// Equals returns true if the NewStatus are identical.
func (status NewStatus) Equals(status2 NewStatus) bool {
	sbz, s2bz := status.Bytes(), status2.Bytes()
	return bytes.Equal(sbz, s2bz)
}

// Bytes serializes the NewStatus using go-amino.
func (status NewStatus) Bytes() []byte {
	return ser.MustEncodeToBytes(status)
}

// IsEmpty returns true if the NewStatus is equal to the empty status.
func (status NewStatus) IsEmpty() bool {
	return status.Validators == nil // XXX can't compare to Empty
}

// GetValidators returns the last and current validator sets.
func (status NewStatus) GetValidators() (last *types.ValidatorSet, current *types.ValidatorSet) {
	return status.LastValidators, status.Validators
}

//------------------------------------------------------------------------
// Genesis

// MakeGenesisStatusFromFile reads and unmarshals NewStatus from the given
// file.
//
// Used during replay and in tests.
func MakeGenesisStatusFromFile(genDocFile string) (NewStatus, error) {
	genDoc, err := types.GenesisDocFromFile(genDocFile)
	if err != nil {
		return NewStatus{}, err
	}
	return MakeGenesisStatus(genDoc)
}

// MakeGenesisStatus creates NewStatus from types.GenesisDoc.
func MakeGenesisStatus(genDoc *types.GenesisDoc) (NewStatus, error) {
	err := genDoc.ValidateAndComplete()
	if err != nil {
		return NewStatus{}, fmt.Errorf("Error in genesis file: %v", err)
	}

	// Make validators slice
	validators := make([]*types.Validator, len(genDoc.Validators))
	for i, val := range genDoc.Validators {
		pubKey := val.PubKey
		address := pubKey.Address()

		// Make validator
		validators[i] = &types.Validator{
			Address:     address,
			PubKey:      pubKey,
			VotingPower: val.Power,
			CoinBase:    val.CoinBase,
		}
	}

	return NewStatus{

		ChainID: genDoc.ChainID,

		LastBlockHeight: types.BlockHeightZero,
		LastBlockID:     types.BlockID{},
		LastBlockTime:   uint64(time.Now().Unix()),

		Validators:                  types.NewValidatorSet(validators),
		LastValidators:              types.NewValidatorSet(nil),
		LastHeightValidatorsChanged: types.BlockHeightOne,

		ConsensusParams:                  *genDoc.ConsensusParams,
		LastHeightConsensusParamsChanged: types.BlockHeightOne,
	}, nil
}

//------------------------------------------------------------------------

var (
	statusKey = []byte("statusKey")
)

func calcValidatorsKey(height uint64) []byte {
	return []byte(cmn.Fmt("VALDK:%v", height))
}

func calcConsensusParamsKey(height uint64) []byte {
	return []byte(cmn.Fmt("CSPK:%v", height))
}

// CreateStatusFromGenesisFile creates a new one from the given genesisFilePath
// and persists the result to the database.
func CreateStatusFromGenesisFile(statusDB dbm.DB, genesisFilePath string) (NewStatus, error) {
	status, err := MakeGenesisStatusFromFile(genesisFilePath)
	if err != nil {
		return status, err
	}
	SaveStatus(statusDB, status)
	return status, nil
}

// CreateStatusFromGenesisFile creates a new one from the given genesisDoc
// and persists the result to the database.
func CreateStatusFromGenesisDoc(statusDB dbm.DB, genesisDoc *types.GenesisDoc) (NewStatus, error) {
	status, err := MakeGenesisStatus(genesisDoc)
	if err != nil {
		return status, err
	}
	SaveStatus(statusDB, status)
	return status, nil
}

// LoadStatus loads the NewStatus from the database.
func LoadStatus(db dbm.DB) (NewStatus, error) {
	return loadStatus(db, statusKey)
}

// LoadStatusByHeight from the database
func LoadStatusByHeight(db dbm.DB, height uint64) (NewStatus, error) {
	currStatusKey := fmt.Sprintf("%s_%d", statusKey, height)
	return loadStatus(db, []byte(currStatusKey))
}

func loadStatus(db dbm.DB, key []byte) (NewStatus, error) {
	status := NewStatus{}
	buf := db.Get(key)
	if len(buf) == 0 {
		return status, fmt.Errorf("Error get consensus status from db")
	}

	err := ser.DecodeBytes(buf, &status)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		cmn.Exit(cmn.Fmt(`LoadStatus: Data has been corrupted or its spec has changed:%v\n`, err))
	}
	// TODO: ensure that buf is completely read.

	return status, err
}

// SaveStatus persists the NewStatus, the ValidatorsInfo, and the ConsensusParamsInfo to the database.
func SaveStatus(db dbm.DB, status NewStatus) {
	saveStatus(db, status)
}

func saveStatus(db dbm.DB, status NewStatus) {
	nextHeight := status.LastBlockHeight + 1
	saveValidatorsInfo(db, nextHeight, status.LastHeightValidatorsChanged, status.Validators)
	saveConsensusParamsInfo(db, nextHeight, status.LastHeightConsensusParamsChanged, status.ConsensusParams)
	db.SetSync(statusKey, status.Bytes())
	saveLastTenStatus(db, status)
}

func saveLastTenStatus(db dbm.DB, status NewStatus) {
	currStatusKey := fmt.Sprintf("%s_%d", statusKey, status.LastBlockHeight)
	if status.LastBlockHeight > 10 {
		beforeTenStatusKey := fmt.Sprintf("%s_%d", statusKey, status.LastBlockHeight-10)
		//del key
		db.DeleteSync([]byte(beforeTenStatusKey))
	}

	db.SetSync([]byte(currStatusKey), status.Bytes())
}

//-----------------------------------------------------------------------------

// ValidatorsInfo represents the latest validator set, or the last height it changed
type ValidatorsInfo struct {
	ValidatorSet      *types.ValidatorSet
	LastHeightChanged uint64
}

// Bytes serializes the ValidatorsInfo using go-amino.
func (valInfo *ValidatorsInfo) Bytes() []byte {
	return ser.MustEncodeToBytes(valInfo)
}

// LoadValidators loads the ValidatorSet for a given height.
// Returns ErrNoValSetForHeight if the validator set can't be found for this height.
func LoadValidators(db dbm.DB, height uint64) (*types.ValidatorSet, uint64, error) {
	valInfo := loadValidatorsInfo(db, height)
	if valInfo == nil {
		return nil, 0, ErrNoValSetForHeight{height}
	}
	lastHeightChanged := valInfo.LastHeightChanged
	if valInfo.ValidatorSet == nil {
		valInfo2 := loadValidatorsInfo(db, lastHeightChanged)
		if valInfo2 == nil {
			cmn.PanicSanity(fmt.Sprintf(`Couldn't find validators at height %d as
                        last changed from height %d`, valInfo.LastHeightChanged, height))
		}
		valInfo = valInfo2
	}

	return valInfo.ValidatorSet, lastHeightChanged, nil
}

func loadValidatorsInfo(db dbm.DB, height uint64) *ValidatorsInfo {
	buf := db.Get(calcValidatorsKey(height))
	if len(buf) == 0 {
		return nil
	}

	v := new(ValidatorsInfo)
	err := ser.DecodeBytes(buf, v)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		cmn.Exit(cmn.Fmt(`LoadValidators: Data has been corrupted or its spec has changed:
                %v\n`, err))
	}
	// TODO: ensure that buf is completely read.

	return v
}

// saveValidatorsInfo persists the validator set for the next block to disk.
// It should be called from s.Save(), right before the status itself is persisted.
// If the validator set did not change after processing the latest block,
// only the last height for which the validators changed is persisted.
func saveValidatorsInfo(db dbm.DB, nextHeight, changeHeight uint64, valSet *types.ValidatorSet) {
	valInfo := &ValidatorsInfo{
		LastHeightChanged: changeHeight,
	}
	if changeHeight == nextHeight {
		valInfo.ValidatorSet = valSet
	}
	db.SetSync(calcValidatorsKey(nextHeight), valInfo.Bytes())
}

//-----------------------------------------------------------------------------

// ConsensusParamsInfo represents the latest consensus params, or the last height it changed
type ConsensusParamsInfo struct {
	ConsensusParams   types.ConsensusParams
	LastHeightChanged uint64
}

// Bytes serializes the ConsensusParamsInfo using go-amino.
func (params ConsensusParamsInfo) Bytes() []byte {
	return ser.MustEncodeToBytes(params)
}

// LoadConsensusParams loads the ConsensusParams for a given height.
func LoadConsensusParams(db dbm.DB, height uint64) (types.ConsensusParams, error) {
	empty := types.ConsensusParams{}

	paramsInfo := loadConsensusParamsInfo(db, height)
	if paramsInfo == nil {
		return empty, ErrNoConsensusParamsForHeight{height}
	}

	if paramsInfo.ConsensusParams == empty {
		paramsInfo = loadConsensusParamsInfo(db, paramsInfo.LastHeightChanged)
		if paramsInfo == nil {
			cmn.PanicSanity(fmt.Sprintf(`Couldn't find consensus params at height %d as
                        last changed from height %d`, paramsInfo.LastHeightChanged, height))
		}
	}

	return paramsInfo.ConsensusParams, nil
}

func loadConsensusParamsInfo(db dbm.DB, height uint64) *ConsensusParamsInfo {
	buf := db.Get(calcConsensusParamsKey(height))
	if len(buf) == 0 {
		return nil
	}

	paramsInfo := new(ConsensusParamsInfo)
	err := ser.DecodeBytes(buf, paramsInfo)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		cmn.Exit(cmn.Fmt(`LoadConsensusParams: Data has been corrupted or its spec has changed:
                %v\n`, err))
	}
	// TODO: ensure that buf is completely read.

	return paramsInfo
}

// saveConsensusParamsInfo persists the consensus params for the next block to disk.
// It should be called from s.Save(), right before the status itself is persisted.
// If the consensus params did not change after processing the latest block,
// only the last height for which they changed is persisted.
func saveConsensusParamsInfo(db dbm.DB, nextHeight, changeHeight uint64, params types.ConsensusParams) {
	paramsInfo := &ConsensusParamsInfo{
		LastHeightChanged: changeHeight,
	}
	if changeHeight == nextHeight {
		paramsInfo.ConsensusParams = params
	}
	db.SetSync(calcConsensusParamsKey(nextHeight), paramsInfo.Bytes())
}

var startDeleteHeight = []byte("CSSDH")
var heightBytes = make([]byte, 8)

func saveStartDeleteHeight(db dbm.DB, height uint64) {
	binary.BigEndian.PutUint64(heightBytes, height)
	db.Set(startDeleteHeight, heightBytes)
}

func loadStartDeleteHeight(db dbm.DB) uint64 {
	h, err := db.Load(startDeleteHeight)
	if err == nil {
		return binary.BigEndian.Uint64(h)
	}
	return 1
}
