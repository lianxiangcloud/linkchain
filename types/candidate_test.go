package types

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
)

type CansInfo struct {
	coinBase common.Address
	deposit  int64
}

func initCans() CandidateInOrderList {
	var canInfos = []*CansInfo{
		&CansInfo{
			coinBase: common.HexToAddress("0x0000000000000000000000000000000000000019"),
			deposit:  int64(10362221),
		},
		&CansInfo{
			coinBase: common.HexToAddress("0x0000000000000000000000000000000000000017"),
			deposit:  int64(9656560),
		},
		&CansInfo{
			coinBase: common.HexToAddress("0x0000000000000000000000000000000000000007"),
			deposit:  int64(5071514),
		},
		&CansInfo{
			coinBase: common.HexToAddress("0x0000000000000000000000000000000000000011"),
			deposit:  int64(14079388),
		},
		&CansInfo{
			coinBase: common.HexToAddress("0x0000000000000000000000000000000000000008"),
			deposit:  int64(13687298),
		},
	}

	cans := make(CandidateInOrderList, len(canInfos))
	for i, canInfo := range canInfos {
		can := &CandidateInOrder{}
		can.CoinBase = canInfo.coinBase
		can.Deposit = canInfo.deposit
		can.Score = 100
		cans[i] = can
	}

	return cans
}

func TestWinCount(t *testing.T) {
	maxDeposit := int64(14079388)
	maxScore := int64(500)
	result := make(map[string]uint64)
	loop := 10000
	cans := initCans()
	r := rand.New(rand.NewSource(int64(time.Now().UnixNano())))
	for i := 0; i < loop; i++ {
		subScore := int64(0)

		salt := r.Int63()
		for _, can := range cans {
			h := crypto.Keccak256Hash([]byte(strconv.FormatInt(salt, 10)), can.CoinBase[:])
			randNum := binary.BigEndian.Uint64(h[:8])
			can.Rand = int64(randNum & math.MaxInt64)
			subScore += can.Score
		}
		for _, can := range cans {
			can.CalRank(4, 4, 2, maxDeposit, subScore)
		}

		//sort.Sort(cans)
		cans.RandomSort(salt)
		result[cans[0].CoinBase.String()]++
		result[cans[1].CoinBase.String()]++
		result[cans[2].CoinBase.String()]++
		if cans[0].Score < maxScore-33 {
			cans[0].Score += 33
			cans[1].Score += 33
			cans[2].Score += 33
		}
	}
	fmt.Println(result)
}

func TestRandomSort(t *testing.T) {

	r := rand.New(rand.NewSource(int64(time.Now().UnixNano())))

	var cans = CandidateInOrderList{
		&CandidateInOrder{
			RankResult: big.NewRat(8, 1),
			Score:      8,
		},
		&CandidateInOrder{
			RankResult: big.NewRat(3, 1),
			Score:      3,
		},
		&CandidateInOrder{
			RankResult: big.NewRat(5, 1),
			Score:      5,
		},
		&CandidateInOrder{
			RankResult: big.NewRat(1, 1),
			Score:      1,
		},
	}
	cans.RandomSort(r.Int63())
}
