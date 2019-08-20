package txmgr

import (
	"testing"

	//"strings"
	//"unsafe"

	"github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/stretchr/testify/require"
)

var crossState CrossState
var txmgr *Service

func init() {
	db := initDB()
	txmgr = NewCrossState(db, nil)
	crossState = txmgr
}

func initDB() dbm.DB {
	db := dbm.NewMemDB()

	//multisigner
	needSaveInfo := &types.SignersInfo{}
	value, err := ser.EncodeToBytes(needSaveInfo)
	if err != nil {
		panic(err)
	}

	key := []byte(types.DBupdateValidatorsKey)
	db.Set(key, value)
	key = []byte(types.DBcontractCreateKey)
	db.Set(key, value)

	return db
}

func getTestMultiSignMainInfo(txType types.SupportType) (*types.MultiSignMainInfo, []types.PrivValidator, *types.ValidatorSet) {
	validators := make([]*types.Validator, 0, 10)
	privs := make([]types.PrivValidator, 0, 10)
	for i := 0; i < 10; i++ {
		v, p := types.RandValidator(false, 1)
		validators = append(validators, v)
		privs = append(privs, p)
	}
	valSet := types.NewValidatorSet(validators)

	mainInfo := &types.MultiSignMainInfo{
		SupportTxType: txType,
		SignersInfo: types.SignersInfo{
			MinSignerPower: 20,
			Signers: []*types.SignerEntry{
				&types.SignerEntry{
					Power: 10,
					Addr:  common.HexToAddress("0x1"),
				},
				&types.SignerEntry{
					Power: 10,
					Addr:  common.HexToAddress("0x2"),
				},
				&types.SignerEntry{
					Power: 10,
					Addr:  common.HexToAddress("0x3"),
				},
			},
		},
	}
	return mainInfo, privs, valSet
}

func TestMultiSign(t *testing.T) {
	mainInfo, _, _ := getTestMultiSignMainInfo(types.TxUpdateValidatorsType)
	mtx := types.NewMultiSignAccountTx(mainInfo, nil)
	crossState.AddSpecialTx([]types.Tx{mtx})

	mainInfo, _, _ = getTestMultiSignMainInfo(types.TxContractCreateType)
	mtx = types.NewMultiSignAccountTx(mainInfo, nil)
	crossState.AddSpecialTx([]types.Tx{mtx})

	msinfo := crossState.GetMultiSignersInfo(types.TxUpdateValidatorsType)
	require.NotNil(t, msinfo)
	msinfo = crossState.GetMultiSignersInfo(types.TxContractCreateType)
	require.NotNil(t, msinfo)
}
