package types

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"math/rand"

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
//	0.3*(score/500)+0.4*(deposit/Maxdeposit)+0.3*(rand/sub)
func (v *CandidateInOrder) CalRank(a, b, c, maxDeposit, subScore int64) {
	sub := a + b + c
	p1, p2, p3 := big.NewRat(a, sub), big.NewRat(b, sub), big.NewRat(c, sub)
	s1 := p1.Mul(p1, big.NewRat(v.Score, subScore))
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
	return cl[i].RankResult.Cmp(cl[j].RankResult) < 0
}

func (cl CandidateInOrderList) Swap(i, j int) {
	it := cl[i]
	cl[i] = cl[j]
	cl[j] = it
}

//RandomSort re-sort candidateList by random chose based on RankResult
func (cl CandidateInOrderList) RandomSort(salt int64) {
	r := rand.New(rand.NewSource(salt))
	size := len(cl)
	for i := 0; i < size-1; i++ {
		//fmt.Println("---------------------------------------------------------------------------------")
		randomRat := (&big.Rat{}).SetFloat64(r.Float64()) //0.0~1.0
		//f, _ := randomRat.Float64()
		//fmt.Println("randomRat", f)

		//cal the range power e.g. if power list is 1,3,5,8
		//then the range is 1,4,9,17
		rangePower := make([]*big.Rat, size-i)
		for index := range rangePower {
			if index == 0 {
				rangePower[index] = (&big.Rat{}).Set(cl[i+index].RankResult)
				continue
			}
			rangePower[index] = (&big.Rat{}).Set(cl[i+index].RankResult)
			rangePower[index] = (&big.Rat{}).Add(rangePower[index], rangePower[index-1])
		}

		//fmt.Println("rangePower", rangePower)

		//randomRat =maxRange*(0.0~1.0),if range is 1,4,9,17
		//randomRat = 17*(0.0~1.0)
		//if randomRat = 0.5 then 1 is chosen, =10 then 8 is chosen
		randomRat = randomRat.Mul(randomRat, rangePower[size-i-1]) //rand(0.0~1.0)*max

		//f, _ = randomRat.Float64()
		//fmt.Println("randomRat*mul", f)

		var j = 0
		for index, r := range rangePower {
			if randomRat.Cmp(r) < 0 {
				j = index
				break
			}
		}
		cl[i], cl[i+j] = cl[i+j], cl[i]

		//sort.Sort(cl[i+1:])
		//fmt.Println("canidates", cl)
	}
}
