package types

import (
	"bytes"
	"fmt"
	"math"
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
)

// Candidate the base info about candidate
type Candidate struct {
	Address     crypto.Address `json:"address"`
	PubKey      crypto.PubKey  `json:"pub_key"`
	VotingPower int64          `json:"voting_power"`
	CoinBase    common.Address `json:"coinbase"`
}

// CandidateState used to store candidates List in stateDB
type CandidateState struct {
	Candidate    `json:"candidate"`
	Score        int64  `json:"score"`
	PunishHeight uint64 `json:"punish_height"`
}

// CandidateInOrder used to store the candidates sorted by rankInfo in app
type CandidateInOrder struct {
	Candidate   `json:"candidate"`
	ProduceInfo int      `json:"prodece_info"`
	Deposit     int64    `json:"deposit"` //value/wei
	Score       int64    `json:"score"`
	Rand        int64    `json:"rand_num"`
	Rank        int      `json:"rank"`
	RankResult  *big.Rat `json:"-" rlp:"-"`
}

// Copy to deep copy CandidateState
func (v *CandidateState) Copy() *CandidateState {
	vCopy := *v
	return &vCopy
}

func (v *CandidateState) String() string {
	if v == nil {
		return "nil-Candidate"
	}
	return fmt.Sprintf("Candidate{%v %v CB:%v VP:%v S:%v P:%v}",
		v.Address.String(),
		v.PubKey,
		v.CoinBase.String(),
		v.VotingPower,
		v.Score,
		v.PunishHeight)
}

// Copy to deep copy CandidateInOrder
func (v *CandidateInOrder) Copy() *CandidateInOrder {
	vCopy := *v
	vCopy.RankResult = &big.Rat{}
	return &vCopy
}

func (v CandidateInOrder) String() string {
	return fmt.Sprintf("Candidate{%v %v CB:%v VP:%v P:%v D:%v S:%v R:%v RK:%v}",
		v.Address.String(),
		v.PubKey,
		v.CoinBase.String(),
		v.VotingPower,
		v.ProduceInfo,
		v.Deposit,
		v.Score,
		v.Rand,
		v.Rank)

}

// CandidatesList used to map CandidateState in stateDB
type CandidatesList map[string]*CandidateState

// Copy to deep copy CandidatesList
func (cls CandidatesList) Copy() CandidatesList {
	clsCopy := make(CandidatesList)
	for k, v := range cls {
		clsCopy[k] = v.Copy()
	}
	return clsCopy
}

// CandidatesByAddress for sort CandidateState List
type CandidatesByAddress []*CandidateState

func (cs CandidatesByAddress) Len() int {
	return len(cs)
}

func (cs CandidatesByAddress) Less(i, j int) bool {
	return bytes.Compare(cs[i].Address, cs[j].Address) == -1
}

func (cs CandidatesByAddress) Swap(i, j int) {
	it := cs[i]
	cs[i] = cs[j]
	cs[j] = it
}

// CalRank Calculate the rank Result,abc is the rank parameters
//	0.3*(score/500)+0.4*(deposit/Maxdeposit)+0.3*(rand/Rmax)
func (v *CandidateInOrder) CalRank(a, b, c, maxDeposit, maxScore int64) {
	sub := a + b + c
	p1, p2, p3 := big.NewRat(a, sub), big.NewRat(b, sub), big.NewRat(c, sub)
	s1 := p1.Mul(p1, big.NewRat(v.Score, maxScore))
	s2 := p2.Mul(p2, big.NewRat(v.Deposit, maxDeposit))
	s3 := p3.Mul(p3, big.NewRat(v.Rand, math.MaxInt64))
	v1 := (&big.Rat{}).Add(s1, s2)
	v.RankResult = (&big.Rat{}).Add(v1, s3)
}

// CandidateInOrderList for sort CandidateState List
type CandidateInOrderList []*CandidateInOrder

func (cl CandidateInOrderList) Len() int {
	return len(cl)
}

func (cl CandidateInOrderList) Less(i, j int) bool {
	return cl[i].RankResult.Cmp(cl[j].RankResult) > 0
}

func (cl CandidateInOrderList) Swap(i, j int) {
	it := cl[i]
	cl[i] = cl[j]
	cl[j] = it
}
