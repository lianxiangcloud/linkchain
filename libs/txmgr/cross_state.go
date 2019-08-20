package txmgr

import (
	"github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/types"
)

//CrossState an interface for txmgr.
type CrossState interface {
	//txentry
	GetTxEntry(hash common.Hash) *types.TxEntry
	SaveTxEntry(block *types.Block, txsResult *types.TxsResult)
	DeleteTxEntry(block *types.Block)

	//specialtx
	AddSpecialTx(txs []types.Tx)

	//MultiSign
	GetMultiSignersInfo(txtype types.SupportType) *types.SignersInfo

	//new db batch
	NewDbBatch() dbm.Batch
	//flush db
	Sync()
}
