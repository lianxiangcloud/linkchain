package state

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/types"
)

//CoefficientJSON used for json encoded in coefficient contract
type CoefficientJSON struct {
	VotePeriod     uint64 `json:"VotePeriod"` //voting per VotePeriod blocks
	types.VoteRate `json:"VoteRate"`
	types.CalRate  `json:"CalRate"`
	MaxScore       int64  `json:"MaxScore"`
	UTXOFee        string `json:"UTXOFee"`
}

//GetCoefficient read all coefficient from contract stateDB
func (st *StateDB) GetCoefficient(logger log.Logger) *types.Coefficient {
	buff := st.GetState(config.ContractCoefficientAddr, crypto.Keccak256Hash([]byte("Coefficient")))
	if len(buff) <= 3 {
		logger.Error("GetCoefficient: GetState nil")
		return nil
	}
	var coJSON = CoefficientJSON{}
	if err := json.Unmarshal(buff[3:len(buff)-1], &coJSON); err != nil {
		logger.Error("GetCoefficient: JSON Unmarshal Coefficient", "err", err)
		return nil
	}
	UTXOFee, ok := big.NewInt(0).SetString(coJSON.UTXOFee, 0)
	if !ok {
		logger.Error("GetCoefficient: UTXOFee is not BigInt string", "UTXOFee", coJSON.UTXOFee)
		return nil
	}
	return &types.Coefficient{
		VotePeriod: coJSON.VotePeriod,
		VoteRate:   coJSON.VoteRate,
		CalRate:    coJSON.CalRate,
		MaxScore:   coJSON.MaxScore,
		UTXOFee:    UTXOFee,
	}
}

//ValidatorJSON used for json encoded in validator contract
type ValidatorJSON struct {
	PubKey      string         `json:"pub_key"`
	CoinBase    common.Address `json:"coinbase"`
	VotingPower int64          `json:"voting_power"`
}

//GetWhiteValidators get inner validators in whiteList contract
func (st *StateDB) GetWhiteValidators(logger log.Logger) []*types.Validator {
	buff := st.GetState(config.ContractValidatorsAddr, crypto.Keccak256Hash([]byte("ValidatorList")))
	pubkeyList := readStringV(buff)

	vals := make([]*types.Validator, 0)
	for _, key := range pubkeyList {
		keyByte := packStringkey("Validator", key)
		buff := st.GetState(config.ContractValidatorsAddr, crypto.Keccak256Hash(keyByte))
		if len(buff) <= 3 {
			logger.Error("GetWhiteValidators: GetState nil")
			continue
		}
		var valJSON = ValidatorJSON{}
		if err := json.Unmarshal(buff[3:len(buff)-1], &valJSON); err != nil {
			logger.Error("GetWhiteValidators: JSON Unmarshal fail", "err", err, "buff", hex.EncodeToString(buff))
			continue
		}
		pubkey, err := crypto.HexToPubkey(valJSON.PubKey)
		if err != nil {
			logger.Error("GetWhiteValidators: pubkey err", "pubkey", valJSON.PubKey)
			continue
		}
		vals = append(vals, &types.Validator{
			PubKey:      pubkey,
			Address:     pubkey.Address(),
			CoinBase:    valJSON.CoinBase,
			VotingPower: valJSON.VotingPower,
		})
	}
	return vals
}

//CandidateJSON used for json encoded in candidate contract
type CandidateJSON struct {
	PubKey       string         `json:"pub_key"`
	CoinBase     common.Address `json:"coinbase"`
	VotingPower  int64          `json:"voting_power"`
	Score        int64          `json:"score"`
	PunishHeight uint64         `json:"punish_height"`
}

//GetAllCandidates get inner CandidateState from contract
func (st *StateDB) GetAllCandidates(logger log.Logger) []*types.CandidateState {
	buff := st.GetState(config.ContractCandidatesAddr, crypto.Keccak256Hash([]byte("pubkeys")))
	pubkeyList := readStringV(buff)

	cans := make([]*types.CandidateState, 0)
	for _, key := range pubkeyList {
		keyByte := packStringkey("cand", key)
		buff := st.GetState(config.ContractCandidatesAddr, crypto.Keccak256Hash(keyByte))
		if len(buff) <= 3 {
			logger.Error("GetAllCandidates: GetState nil")
			continue
		}

		var canJSON = CandidateJSON{}
		if err := json.Unmarshal(buff[3:len(buff)-1], &canJSON); err != nil {
			logger.Error("GetAllCandidates: JSON Unmarshal fail", "err", err, "buff", hex.EncodeToString(buff))
			continue
		}
		pubkey, err := crypto.HexToPubkey(canJSON.PubKey)
		if err != nil {
			logger.Error("GetAllCandidates: pubkey err", "pubkey", canJSON.PubKey)
			continue
		}
		cans = append(cans, &types.CandidateState{
			Candidate: types.Candidate{
				PubKey:      pubkey,
				Address:     pubkey.Address(),
				VotingPower: canJSON.VotingPower,
				CoinBase:    canJSON.CoinBase,
			},
			Score:        canJSON.Score,
			PunishHeight: canJSON.PunishHeight,
		})
	}
	return cans
}

func (st *StateDB) GetBlacklist() []common.Address {
	buff := st.GetState(config.ContractBlacklistAddr, crypto.Keccak256Hash([]byte("blacklist")))
	return readAddressV(buff)
}

//OP indicate the operation to score
const (
	OPZERO  int = iota
	OPCLEAR     // clear 0
	OPADD       // +1
	OPSUB       //-1
)

//UpdateCandidateScore update the canidate score in canidate contract
func (st *StateDB) UpdateCandidateScore(pubkey crypto.PubKey, op int, maxScore, height int64, logger log.Logger) {
	key := common.Bytes2Hex(pubkey.Bytes())
	key = "0x" + key + string(0)

	logger.Debug("UpdateCandidateScore: ", "pubkey", key, "op", op, "maxScore", maxScore)
	keyByte := packStringkey("cand", key)

	buff := st.GetState(config.ContractCandidatesAddr, crypto.Keccak256Hash(keyByte))
	if len(buff) <= 3 {
		logger.Error("UpdateCandidateScore: GetState nil")
		return
	}

	var canJSON = CandidateJSON{}
	if err := json.Unmarshal(buff[3:len(buff)-1], &canJSON); err != nil {
		logger.Error("UpdateCandidateScore: JSON Unmarshal fail", "err", err, "buff", hex.EncodeToString(buff))
		return
	}

	var needUpdate bool
	switch op {
	case OPCLEAR:
		canJSON.Score, canJSON.PunishHeight = 0, uint64(height)
		needUpdate = true
	case OPADD:
		if canJSON.Score < maxScore {
			canJSON.Score++
			needUpdate = true
		}
	case OPSUB:
		if canJSON.Score > 1 {
			canJSON.Score--
			needUpdate = true
		}
	}
	if needUpdate {
		newbuff, _ := json.Marshal(canJSON)
		buffCopy := make([]byte, 0, len(buff))
		buffCopy = append(buffCopy, buff[0:3]...)
		buffCopy = append(buffCopy, newbuff...)
		buffCopy = append(buffCopy, byte(0))
		st.SetState(config.ContractCandidatesAddr, crypto.Keccak256Hash(keyByte), buffCopy)
	}
}

//GetCandidatesDeposit get deposit amount from pledge contract
func (st *StateDB) GetCandidatesDeposit(addrs []common.Address, logger log.Logger) []*big.Int {
	deposits := make([]*big.Int, 0, len(addrs))
	for _, addr := range addrs {
		key := packStringkey("electorsMap", addr.String()+string(0))
		value := st.GetState(config.ContractPledgeAddr, crypto.Keccak256Hash(key))
		deposit := big.NewInt(0)
		if len(value) > 3 {
			len := binary.LittleEndian.Uint16(value[2:4]) //1:tagObj 2:tagString 3,4:stringLen
			v, ok := big.NewInt(0).SetString(string(value[4:4+len-1]), 0)
			if !ok {
				logger.Error("GetCandidatesDeposit: get deposit fail", "addr", addr.String())
				panic(fmt.Sprintf("GetCandidatesDeposit: get deposit fail addr:%s value:%s", addr.String(), value))
			}
			deposit = v
		}
		deposits = append(deposits, deposit)
	}
	return deposits
}

//beblow to parse the contract slice json codes

//TagArray in TLV
var TagArray = byte(7)

//TagString in TLV
var TagString = byte(6)

func readTag(data []byte, pos *uint64) byte {
	t := data[*pos]
	*pos = *pos + 1
	return t
}

func readSize(val []byte, pos *uint64) uint16 {
	var size = uint16(val[*pos]) + uint16(val[*pos+1])<<uint(8)
	*pos = *pos + 2
	return size
}

func readString(val []byte, pos *uint64) string {
	var size = readSize(val, pos)
	s := val[*pos : *pos+uint64(size)]
	*pos = *pos + uint64(size)
	return string(s[:])
}

func readStringV(val []byte) []string {
	var strSlice []string
	var pos uint64
	if val == nil {
		return strSlice
	}
	tag := readTag(val, &pos)
	if tag != TagArray {
		return strSlice
	}
	vectorSize := readSize(val, &pos)
	strSlice = make([]string, 0)
	for loop := vectorSize; loop > 0; loop = loop - 1 {
		tag := readTag(val, &pos)
		if tag == TagString {
			str := readString(val, &pos)
			strSlice = append(strSlice, str)
		}
	}
	if len(strSlice) != int(vectorSize) {
		strSlice = nil
	}

	return strSlice
}

//packStringkey pack the key1 and key2 for tlv encode; key2 should end with 0
func packStringkey(key1, key2 string) []byte {
	var (
		key  = make([]byte, 0, len(key1)+len(key2)+3)
		buff = make([]byte, 2)
	)
	len := uint16(len(key2))
	binary.LittleEndian.PutUint16(buff, len)

	key = append(key, key1...)
	key = append(key, TagString)
	key = append(key, buff...)
	key = append(key, key2...)

	return key[:]
}

func readAddressV(val []byte) []common.Address {
	var addrSlice []common.Address
	var pos uint64
	if val == nil {
		return addrSlice
	}
	tag := readTag(val, &pos)
	if tag != TagArray {
		return addrSlice
	}
	vectorSize := readSize(val, &pos)
	addrSlice = make([]common.Address, 0)
	for loop := vectorSize; loop > 0; loop = loop - 1 {
		tag := readTag(val, &pos)
		if tag == TagString {
			str := readString(val, &pos)
			str = str[:len(str)-1]
			addrSlice = append(addrSlice, common.HexToAddress(str))
		}
	}
	if len(addrSlice) != int(vectorSize) {
		addrSlice = nil
	}
	return addrSlice
}