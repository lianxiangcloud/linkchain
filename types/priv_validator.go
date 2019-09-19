package types

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

const (
	stepNone      int8 = 0 // Used to distinguish the initial state
	stepPropose   int8 = 1
	stepPrevote   int8 = 2
	stepPrecommit int8 = 3
)

func voteToStep(vote *Vote) int8 {
	switch vote.Type {
	case VoteTypePrevote:
		return stepPrevote
	case VoteTypePrecommit:
		return stepPrecommit
	default:
		cmn.PanicSanity("Unknown vote type")
		return 0
	}
}

// PrivValidator defines the functionality of a local validator
// that signs votes, proposals, and heartbeats, and never double signs.
type PrivValidator interface {
	GetAddress() crypto.Address // redundant since .PubKey().Address()
	GetPubKey() crypto.PubKey

	UpdatePrikey(priv crypto.PrivKey)
	GetPrikey() crypto.PrivKey

	SignData(data []byte) ([]byte, error)
	SignVote(chainID string, vote *Vote) error
	SignVoteWithoutSave(chainID string, vote *Vote) error
	SignProposal(chainID string, proposal *Proposal) error
	SignHeartbeat(chainID string, heartbeat *Heartbeat) error
}

//----------------------------------------
// Misc.

type PrivValidatorsByAddress []PrivValidator

func (pvs PrivValidatorsByAddress) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(pvs[i].GetAddress(), pvs[j].GetAddress()) == -1
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	it := pvs[i]
	pvs[i] = pvs[j]
	pvs[j] = it
}

//----------------------------------------
//FilePV
//
// FilePV implements PrivValidator using data persisted to disk
// to prevent double signing.
// NOTE: the directory containing the pv.filePath must already exist.
type FilePV struct {
	pv            *FilePV
	Address       crypto.Address   `json:"address"`
	PubKey        crypto.PubKey    `json:"pub_key"`
	LastHeight    uint64           `json:"last_height"`
	LastRound     int              `json:"last_round"`
	LastStep      int8             `json:"last_step"`
	LastSignature crypto.Signature `json:"last_signature,omitempty"` // so we dont lose signatures XXX Why would we lose signatures?
	LastSignBytes cmn.HexBytes     `json:"last_signbytes,omitempty"` // so we dont lose signatures XXX Why would we lose signatures?
	PrivKey       crypto.PrivKey   `json:"priv_key"`

	// For persistence.
	// Overloaded for testing.
	filePath string
	mtx      sync.Mutex
}

// GetAddress returns the address of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetAddress() crypto.Address {
	return pv.Address
}

// GetPubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetPubKey() crypto.PubKey {
	return pv.PubKey
}

// UpdatePrikey update PrivKey with the given privkey
func (pv *FilePV) UpdatePrikey(priv crypto.PrivKey) {
	pv.Address = priv.PubKey().Address()
	pv.PubKey = priv.PubKey()
	pv.PrivKey = priv
}

func (pv *FilePV) GetPrikey() crypto.PrivKey {
	return pv.PrivKey
}

// GenFilePV generates a new validator with randomly generated private key
// and sets the filePath, but does not call Save().
func GenFilePV(filePath string) *FilePV {
	privKey := crypto.GenPrivKeyEd25519()
	pv := &FilePV{
		Address:  privKey.PubKey().Address(),
		PubKey:   privKey.PubKey(),
		PrivKey:  privKey,
		LastStep: stepNone,
		filePath: filePath,
	}
	pv.pv = pv.Copy()
	return pv
}

func LoadPVFromBytes(data []byte) (*FilePV, error) {
	pv := &FilePV{}
	if err := ser.UnmarshalJSON(data, &pv); err != nil {
		return nil, err
	}
	pv.Address = pv.PrivKey.PubKey().Address()
	pv.PubKey = pv.PrivKey.PubKey()
	pv.pv = pv.Copy()
	return pv, nil
}

// LoadFilePV loads a FilePV from the filePath.  The FilePV handles double
// signing prevention by persisting data to the filePath.  If the filePath does
// not exist, the FilePV must be created manually and saved.
func LoadFilePV(filePath string) *FilePV {
	pvJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		cmn.Exit(err.Error())
	}
	pv := &FilePV{}
	err = ser.UnmarshalJSON(pvJSONBytes, &pv)
	if err != nil {
		cmn.Exit(cmn.Fmt("Error reading PrivValidator from %v: %v\n", filePath, err))
	}

	pv.filePath = filePath
	pv.Address = pv.PrivKey.PubKey().Address()
	pv.PubKey = pv.PrivKey.PubKey()
	pv.pv = pv.Copy()
	return pv
}

// LoadOrGenFilePV loads a FilePV from the given filePath
// or else generates a new one and saves it to the filePath.
func LoadOrGenFilePV(filePath string) *FilePV {
	var pv *FilePV
	if cmn.FileExists(filePath) {
		pv = LoadFilePV(filePath)
	} else {
		pv = GenFilePV(filePath)
		pv.Save()
	}
	return pv
}

// Copy copy a FilePV
func (pv *FilePV) Copy() *FilePV {
	return &FilePV{
		Address:       pv.Address,
		PubKey:        pv.PubKey,
		LastHeight:    pv.LastHeight,
		LastRound:     pv.LastRound,
		LastStep:      pv.LastStep,
		LastSignature: pv.LastSignature,
		LastSignBytes: pv.LastSignBytes,
		PrivKey:       pv.PrivKey,
		filePath:      pv.filePath,
	}
}

// Save persists the FilePV to disk.
func (pv *FilePV) Save() {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	pv.save()
}

func (pv *FilePV) save() {
	outFile := pv.filePath
	if outFile == "" {
		panic("Cannot save PrivValidator: filePath not set")
	}

	bakPrivVal := *pv

	jsonBytes, err := ser.MarshalJSONIndent(bakPrivVal, "", "  ")
	if err != nil {
		panic(err)
	}
	err = cmn.WriteFileAtomic(outFile, jsonBytes, 0600)
	if err != nil {
		panic(err)
	}
}

// Reset resets all fields in the FilePV.
// NOTE: Unsafe!
func (pv *FilePV) Reset() {
	var sig crypto.Signature
	pv.LastHeight = 0
	pv.LastRound = 0
	pv.LastStep = 0
	pv.LastSignature = sig
	pv.LastSignBytes = nil
	pv.Save()
}

// SignData signs a piece of data. Implements PrivValidator.
func (pv *FilePV) SignData(data []byte) ([]byte, error) {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	sig, err := pv.PrivKey.Sign(data)
	if err != nil {
		return nil, err
	}
	return sig.Bytes(), nil
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (pv *FilePV) SignVote(chainID string, vote *Vote) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	if err := pv.signVote(chainID, vote, true); err != nil {
		return errors.New(cmn.Fmt("Error signing vote: %v", err))
	}
	return nil
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (pv *FilePV) SignVoteWithoutSave(chainID string, vote *Vote) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	if err := pv.signVote(chainID, vote, false); err != nil {
		return errors.New(cmn.Fmt("Error signing vote: %v", err))
	}
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements PrivValidator.
func (pv *FilePV) SignProposal(chainID string, proposal *Proposal) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	if err := pv.signProposal(chainID, proposal); err != nil {
		return fmt.Errorf("Error signing proposal: %v", err)
	}
	return nil
}

// returns error if HRS regression or no LastSignBytes. returns true if HRS is unchanged
func (pv *FilePV) checkHRS(height uint64, round int, step int8) (bool, error) {
	if pv.LastHeight > height {
		return false, errors.New("Height regression")
	}

	if pv.LastHeight == height {
		if pv.LastRound > round {
			return false, errors.New("Round regression")
		}

		if pv.LastRound == round {
			if pv.LastStep > step {
				return false, errors.New("Step regression")
			} else if pv.LastStep == step {
				if pv.LastSignBytes != nil {
					if pv.LastSignature == nil {
						panic("pv: LastSignature is nil but LastSignBytes is not!")
					}
					return true, nil
				}
				return false, errors.New("No LastSignature found")
			}
		}
	}
	return false, nil
}

// signVote checks if the vote is good to sign and sets the vote signature.
// It may need to set the timestamp as well if the vote is otherwise the same as
// a previously signed vote (ie. we crashed after signing but before the vote hit the WAL).
func (pv *FilePV) signVote(chainID string, vote *Vote, save bool) error {
	height, round, step := vote.Height, vote.Round, voteToStep(vote)
	signBytes := vote.SignBytes(chainID)

	sameHRS, err := pv.checkHRS(height, round, step)
	if err != nil {
		return err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, pv.LastSignBytes) {
			vote.Signature = pv.LastSignature
		} else if timestamp, ok := checkVotesOnlyDifferByTimestamp(pv.LastSignBytes, signBytes); ok {
			vote.Timestamp = timestamp
			vote.Signature = pv.LastSignature
		} else {
			err = fmt.Errorf("Conflicting data")
		}
		return err
	}

	// It passed the checks. Sign the vote
	sig, err := pv.PrivKey.Sign(signBytes)
	if err != nil {
		return err
	}

	if save {
		pv.saveSigned(height, round, step, signBytes, sig)
	}

	vote.Signature = sig
	return nil
}

// signProposal checks if the proposal is good to sign and sets the proposal signature.
// It may need to set the timestamp as well if the proposal is otherwise the same as
// a previously signed proposal ie. we crashed after signing but before the proposal hit the WAL).
func (pv *FilePV) signProposal(chainID string, proposal *Proposal) error {
	height, round, step := proposal.Height, proposal.Round, stepPropose
	signBytes := proposal.SignBytes(chainID)

	sameHRS, err := pv.checkHRS(height, round, step)
	if err != nil {
		return err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, pv.LastSignBytes) {
			proposal.Signature = pv.LastSignature
		} else if timestamp, ok := checkProposalsOnlyDifferByTimestamp(pv.LastSignBytes, signBytes); ok {
			proposal.Timestamp = timestamp
			proposal.Signature = pv.LastSignature
		} else {
			err = fmt.Errorf("Conflicting data")
		}
		return err
	}

	// It passed the checks. Sign the proposal
	sig, err := pv.PrivKey.Sign(signBytes)
	if err != nil {
		return err
	}
	pv.saveSigned(height, round, step, signBytes, sig)
	proposal.Signature = sig
	return nil
}

// Persist height/round/step and signature
func (pv *FilePV) saveSigned(height uint64, round int, step int8,
	signBytes []byte, sig crypto.Signature) {

	pv.LastHeight = height
	pv.LastRound = round
	pv.LastStep = step
	pv.LastSignature = sig
	pv.LastSignBytes = signBytes
	if pv.pv == nil {
		pv.pv = pv.Copy()
	}
	pv.pv.LastHeight = height
	pv.pv.LastRound = round
	pv.pv.LastStep = step
	pv.pv.LastSignature = sig
	pv.pv.LastSignBytes = signBytes
	pv.pv.save()
}

// SignHeartbeat signs a canonical representation of the heartbeat, along with the chainID.
// Implements PrivValidator.
func (pv *FilePV) SignHeartbeat(chainID string, heartbeat *Heartbeat) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	sig, err := pv.PrivKey.Sign(heartbeat.SignBytes(chainID))
	if err != nil {
		return err
	}
	heartbeat.Signature = sig
	return nil
}

// String returns a string representation of the FilePV.
func (pv *FilePV) String() string {
	return fmt.Sprintf("PrivValidator{%v LH:%v, LR:%v, LS:%v}", pv.GetAddress(), pv.LastHeight, pv.LastRound, pv.LastStep)
}

//-------------------------------------

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the votes is their timestamp.
func checkVotesOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastVote, newVote CanonicalJSONVote
	if err := ser.UnmarshalJSON(lastSignBytes, &lastVote); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into vote: %v", err))
	}
	if err := ser.UnmarshalJSON(newSignBytes, &newVote); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into vote: %v", err))
	}

	lastTime, err := time.Parse(TimeFormat, lastVote.Timestamp)
	if err != nil {
		panic(err)
	}

	// set the times to the same value and check equality
	now := CanonicalTime(time.Now())
	lastVote.Timestamp = now
	newVote.Timestamp = now
	lastVoteBytes, _ := ser.MarshalJSON(lastVote)
	newVoteBytes, _ := ser.MarshalJSON(newVote)

	return lastTime, bytes.Equal(newVoteBytes, lastVoteBytes)
}

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the proposals is their timestamp
func checkProposalsOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastProposal, newProposal CanonicalJSONProposal
	if err := ser.UnmarshalJSON(lastSignBytes, &lastProposal); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into proposal: %v", err))
	}
	if err := ser.UnmarshalJSON(newSignBytes, &newProposal); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into proposal: %v", err))
	}

	lastTime, err := time.Parse(TimeFormat, lastProposal.Timestamp)
	if err != nil {
		panic(err)
	}

	// set the times to the same value and check equality
	now := CanonicalTime(time.Now())
	lastProposal.Timestamp = now
	newProposal.Timestamp = now
	lastProposalBytes, _ := ser.MarshalJSON(lastProposal)
	newProposalBytes, _ := ser.MarshalJSON(newProposal)

	return lastTime, bytes.Equal(newProposalBytes, lastProposalBytes)
}

//----------------------------------------
// MockPV

// MockPV implements PrivValidator without any safety or persistence.
// Only use it for testing.
type MockPV struct {
	privKey crypto.PrivKey
}

func NewMockPV() *MockPV {
	return &MockPV{crypto.GenPrivKeyEd25519()}
}

// Implements PrivValidator.
func (pv *MockPV) GetAddress() crypto.Address {
	return pv.privKey.PubKey().Address()
}

func (pv *MockPV) GetPrikey() crypto.PrivKey {
	return pv.privKey
}

// Implements PrivValidator.
func (pv *MockPV) GetPubKey() crypto.PubKey {
	return pv.privKey.PubKey()
}

// Implements PrivValidator.
func (pv *MockPV) UpdatePrikey(priv crypto.PrivKey) {
}

// Implements PrivValidator.
func (pv *MockPV) SignData(data []byte) ([]byte, error) {
	sig, err := pv.privKey.Sign(data)
	if err != nil {
		return nil, err
	}
	return sig.Bytes(), nil
}

// Implements PrivValidator.
func (pv *MockPV) SignVote(chainID string, vote *Vote) error {
	signBytes := vote.SignBytes(chainID)
	sig, err := pv.privKey.Sign(signBytes)
	if err != nil {
		return err
	}
	vote.Signature = sig
	return nil
}

// Implements PrivValidator.
func (pv *MockPV) SignVoteWithoutSave(chainID string, vote *Vote) error {
	return pv.SignVote(chainID, vote)
}

// Implements PrivValidator.
func (pv *MockPV) SignProposal(chainID string, proposal *Proposal) error {
	signBytes := proposal.SignBytes(chainID)
	sig, err := pv.privKey.Sign(signBytes)
	if err != nil {
		return err
	}
	proposal.Signature = sig
	return nil
}

// signHeartbeat signs the heartbeat without any checking.
func (pv *MockPV) SignHeartbeat(chainID string, heartbeat *Heartbeat) error {
	sig, err := pv.privKey.Sign(heartbeat.SignBytes(chainID))
	if err != nil {
		return err
	}
	heartbeat.Signature = sig
	return nil
}

// String returns a string representation of the MockPV.
func (pv *MockPV) String() string {
	return fmt.Sprintf("MockPV{%v}", pv.GetAddress())
}

// XXX: Implement.
func (pv *MockPV) DisableChecks() {
	// Currently this does nothing,
	// as MockPV has no safety checks at all.
}
