package service

import (
	"context"
	"fmt"
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/libs/txmgr"
	"github.com/lianxiangcloud/linkchain/rpc/filters"
	"github.com/lianxiangcloud/linkchain/rpc/rtypes"
	"github.com/lianxiangcloud/linkchain/types"
)

type PubsubApi struct {
	s     *Service
	txMgr *txmgr.Service
}

func (ps *PubsubApi) context() *Context {
	return ps.s.context()
}

func (ps *PubsubApi) backend() *ApiBackend {
	return ps.s.apiBackend()
}

// BlockSubscribe subscribe new block notifi.
func (ps *PubsubApi) BlockSubscribe(ctx context.Context) (*rpc.Subscription, error) {
	if ps.s.context().eventBus == nil {
		// @Note: Should not happen!
		log.Error("rpc: eventbus nil, not support Subscribetion!!!")
		return nil, rpc.ErrNotificationsUnsupported
	}

	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return nil, rpc.ErrNotificationsUnsupported
	}

	subscription := notifier.CreateSubscription()

	suberName := fmt.Sprintf("rpc-block-suber-%s", subscription.ID)
	ebCtx := context.Background()
	blockCh := make(chan interface{}, 128)
	if err := ps.context().eventBus.Subscribe(ebCtx, suberName, types.EventQueryNewBlock, blockCh); err != nil {
		log.Warn("rpc: Subscribe fail", "err", err)
		return nil, err
	}

	go func() {
		defer func() {
			ps.context().eventBus.Unsubscribe(ebCtx, suberName, types.EventQueryNewBlock)
			//close(blockCh)
		}()

		for {
			select {
			case b := <-blockCh:
				nb := b.(types.EventDataNewBlock)
				if nb.Block == nil {
					log.Warn("ignore empty block")
					continue
				}

				receipts := ps.backend().GetReceipts(nil, nb.Block.HeightU64())
				wBlock := rtypes.NewWholeBlock(nb.Block, receipts)
				if err := notifier.Notify(subscription.ID, wBlock); err != nil {
					log.Error("rpc: notify failed", "err", err, "suber", suberName, "blockHash", nb.Block.Hash().Hex(), "blockNum", nb.Block.HeightU64())
					return
				}

				log.Info("rpc: notify success", "sub", suberName, "blockHash", nb.Block.Hash().Hex(), "blockNum", nb.Block.HeightU64())

			case <-notifier.Closed():
				log.Info("rpc BlockSubscribe: unsubscribe", "suber", suberName)
				return
			case err := <-subscription.Err():
				if err != nil {
					log.Error("rpc subscription: error", "suber", suberName, "err", err)
				} else {
					log.Info("rpc subscription: exit", "suber", suberName)
				}
				return
			}
		}
	}()

	log.Info("rpc BlockSubscribe: ok", "name", suberName)
	return subscription, nil
}

func (ps *PubsubApi) BalanceRecordsSubscribe(ctx context.Context) (*rpc.Subscription, error) {
	if ps.s.context().eventBus == nil {
		// @Note: Should not happen!
		log.Error("rpc: eventbus nil, not support Subscribetion!!!")
		return nil, rpc.ErrNotificationsUnsupported
	}
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return nil, rpc.ErrNotificationsUnsupported
	}

	subscription := notifier.CreateSubscription()
	suberName := fmt.Sprintf("rpc-balance-suber-%s", subscription.ID)
	ebCtx := context.Background()
	blockCh := make(chan interface{}, 128)
	if err := ps.context().eventBus.Subscribe(ebCtx, suberName, types.EventQueryNewBlock, blockCh); err != nil {
		log.Warn("rpc: Subscribe fail", "err", err)
		return nil, err
	}

	go func() {
		defer func() {
			ps.context().eventBus.Unsubscribe(ebCtx, suberName, types.EventQueryNewBlock)
		}()

		for {
			select {
			case b := <-blockCh:
				nb := b.(types.EventDataNewBlock)
				if nb.Block == nil {
					log.Warn("ignore empty block")
					continue
				}
				bbr := ps.backend().GetBlockBalanceRecords(nb.Block.Height)
				br := rtypes.NewBalanceRecordsWithBlockMsg(nb.Block.Height, bbr)
				if err := notifier.Notify(subscription.ID, br); err != nil {
					log.Error("rpc: notify failed", "err", err, "suber", suberName, "blockHash", nb.Block.Hash().Hex(), "blockNum", nb.Block.HeightU64())
					return
				}
				log.Info("rpc: notify success", "sub", suberName, "blockHash", nb.Block.Hash().Hex(), "blockNum", nb.Block.HeightU64())

			case <-notifier.Closed():
				log.Info("rpc BalanceRecordsSubscribe: unsubscribe", "suber", suberName)
				return
			case err := <-subscription.Err():
				if err != nil {
					log.Error("rpc subscription: error", "suber", suberName, "err", err)
				} else {
					log.Info("rpc subscription: exit", "suber", suberName)
				}
				return
			}
		}
	}()

	log.Info("rpc BalanceRecordsSubscribe: ok", "name", suberName)
	return subscription, nil
}

func (ps *PubsubApi) ReceiptsSubscribe(ctx context.Context) (*rpc.Subscription, error) {
	if ps.s.context().eventBus == nil {
		// @Note: Should not happen!
		log.Error("rpc: eventbus nil, not support Subscribetion!!!")
		return nil, rpc.ErrNotificationsUnsupported
	}

	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return nil, rpc.ErrNotificationsUnsupported
	}

	subscription := notifier.CreateSubscription()
	suberName := fmt.Sprintf("rpc-receipts-suber-%s", subscription.ID)
	ebCtx := context.Background()
	blockCh := make(chan interface{}, 128)
	if err := ps.context().eventBus.Subscribe(ebCtx, suberName, types.EventQueryNewBlock, blockCh); err != nil {
		log.Warn("rpc: Subscribe fail", "err", err)
		return nil, err
	}

	go func() {
		defer func() {
			ps.context().eventBus.Unsubscribe(ebCtx, suberName, types.EventQueryNewBlock)
		}()

		for {
			select {
			case b := <-blockCh:
				nb := b.(types.EventDataNewBlock)
				if nb.Block == nil {
					log.Warn("ignore empty block")
					continue
				}
				receipts := ps.backend().GetReceipts(nil, nb.Block.HeightU64())
				rwb := rtypes.NewReceiptsWithBlockHeight(nb.Block.Height, receipts)
				if err := notifier.Notify(subscription.ID, rwb); err != nil {
					log.Error("rpc: notify failed", "err", err, "suber", suberName, "blockNum", nb.Block.HeightU64())
					return
				}
				log.Info("rpc: notify success", "sub", suberName, "blockNum", nb.Block.HeightU64())

			case <-notifier.Closed():
				log.Info("rpc ReceiptsSubscribe: unsubscribe", "suber", suberName)
				return
			case err := <-subscription.Err():
				if err != nil {
					log.Error("rpc subscription: error", "suber", suberName, "err", err)
				} else {
					log.Info("rpc subscription: exit", "suber", suberName)
				}
				return
			}
		}
	}()

	log.Info("rpc ReceiptsSubscribe: ok", "name", suberName)
	return subscription, nil
}

// GetBlock query block
func (ps *PubsubApi) GetBlock(blockNr rpc.BlockNumber) (*rtypes.WholeBlock, error) {
	block, err := ps.backend().BlockByNumber(context.Background(), blockNr)
	if err != nil {
		log.Warn("rpc GetBlock: BlockByNumber fail", "err", err, "number", blockNr)
		return nil, err
	}

	receipts := ps.backend().GetReceipts(nil, block.HeightU64())
	return rtypes.NewWholeBlock(block, receipts), nil
}

// GetTransaction by hash and type.
func (ps *PubsubApi) GetTransaction(hash common.Hash) *rtypes.RPCTx {
	tx, txEntry := ps.backend().GetTx(hash)
	if tx == nil {
		log.Info("GetTransaction fail", "hash", hash)
	}
	return rtypes.NewRPCTx(tx, txEntry)
}

// BroadcastTxSync to broadcast a tx with txType
func (ps *PubsubApi) BroadcastTxSync(txBytes []byte, txType string) (bool, error) {
	var tx types.Tx
	if txType == "" {
		txType = types.TxNormal
	}
	switch txType {
	case types.TxNormal:
		tx = new(types.Transaction)
	case types.TxToken:
		tx = new(types.TokenTransaction)
	case types.TxContractUpgrade:
		tx = new(types.ContractUpgradeTx)
	case types.TxMultiSignAccount:
		tx = new(types.MultiSignAccountTx)
	default:
		return false, types.ErrTxNotSupport
	}

	if err := ser.DecodeBytes(txBytes, tx); err != nil {
		log.Warn("rpc BroadcastTxSync: fail", "err", err, "txType", txType, "otx", string(txBytes))
		return false, err
	}

	log.Debug("BroadcastTxSync", "hash", tx.Hash())
	if err := ps.context().mempool.AddTx("", tx); err != nil {
		rerr, ok := err.(rpc.Error)
		if !ok || (rerr.ErrorCode() != types.ErrTxDuplicate.ErrorCode() && rerr.ErrorCode() != types.ErrMempoolIsFull.ErrorCode()) {
			log.Warn("rpc BroadcastTxSync: mempool.AddTx fail", "err", err, "tx", tx)
		}
		return false, err
	}

	return true, nil
}

// GetBalance the same as rpc api
func (ps *PubsubApi) GetBalance(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*big.Int, error) {
	state, _, err := ps.backend().StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	b := state.GetBalance(address)
	return b, state.Error()
}

// GetTransactionCount the same as rpc api
func (ps *PubsubApi) GetTransactionCount(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (uint64, error) {
	state, _, err := ps.backend().StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return 0, err
	}
	nonce := state.GetNonce(address)
	return nonce, state.Error()
}

// LogsSubscribe subscribe new log notifi.
func (ps *PubsubApi) LogsSubscribe(ctx context.Context, crit filters.FilterCriteria) (*rpc.Subscription, error) {
	if ps.s.context().eventBus == nil {
		// @Note: Should not happen!
		log.Error("rpc: eventbus nil, not support Subscribetion!!!")
		return nil, rpc.ErrNotificationsUnsupported
	}

	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return nil, rpc.ErrNotificationsUnsupported
	}

	subscription := notifier.CreateSubscription()

	suberName := fmt.Sprintf("rpc-log-suber-%s", subscription.ID)
	ebCtx := context.Background()
	logsCh := make(chan interface{}, 128)
	if err := ps.context().eventBus.Subscribe(ebCtx, suberName, types.EventQueryLog, logsCh); err != nil {
		log.Warn("rpc: Subscribe fail", "err", err)
		return nil, err
	}

	go func() {
		defer ps.context().eventBus.Unsubscribe(ebCtx, suberName, types.EventQueryLog)

		for {
			select {
			case ev := <-logsCh:
				logs := ev.(types.EventDataLog).Logs
				logs = filterLogs(logs, crit.FromBlock.ToInt(), crit.ToBlock.ToInt(), crit.Addresses, crit.Topics)
				for _, l := range logs {
					notifier.Notify(subscription.ID, l)
					log.Info("rpc: notify success", "suber", suberName, "log", l)
				}
			case <-notifier.Closed():
				log.Info("rpc LogSubscribe: unsubscribe", "suber", suberName)
				return
			case err := <-subscription.Err():
				if err != nil {
					log.Error("rpc subscription: error", "suber", suberName, "err", err)
				} else {
					log.Info("rpc subscription: exit", "suber", suberName)
				}
				return
			}
		}
	}()

	log.Info("rpc LogsSubscribe: ok", "name", suberName, "crit", crit.String())
	return subscription, nil
}

// GetLogs by filter
func (ps *PubsubApi) GetLogs(ctx context.Context, crit filters.FilterCriteria) ([]*types.Log, error) {
	// Convert the RPC block numbers into internal representations
	if crit.FromBlock == nil {
		crit.FromBlock = (*hexutil.Big)(big.NewInt(rpc.LatestBlockNumber.Int64()))
	}
	if crit.ToBlock == nil {
		crit.ToBlock = (*hexutil.Big)(big.NewInt(rpc.LatestBlockNumber.Int64()))
	}
	log.Trace("rpc GetLogs:", "crit", crit.String())

	// Create and run the filter to get all the logs
	filter := filters.New(ps.backend(), crit.FromBlock.ToInt().Int64(), crit.ToBlock.ToInt().Int64(), crit.Addresses, crit.Topics)

	logs, err := filter.Logs(ctx)
	if err != nil {
		log.Info("rpc GetLogs: filter fail", "err", err)
		return nil, err
	}
	log.Trace("rpc GetLogs: filter ok", "logs", logs)
	return returnLogs(logs), err
}

func (ps *PubsubApi) Validators(ctx context.Context, number rpc.BlockNumber) (*rtypes.ResultValidators, error) {
	if number == rpc.LatestBlockNumber || number == rpc.PendingBlockNumber {
		return ps.backend().Validators(nil)
	}
	height := uint64(number.Int64())
	return ps.backend().Validators(&height)
}

// returnLogs is a helper that will return an empty log array in case the given logs array is nil,
// otherwise the given logs array is returned.
func returnLogs(logs []*types.Log) []*types.Log {
	if logs == nil {
		return []*types.Log{}
	}
	return logs
}

// filterLogs creates a slice of logs matching the given criteria.
func filterLogs(logs []*types.Log, fromBlock, toBlock *big.Int, addresses []common.Address, topics [][]common.Hash) []*types.Log {
	var ret []*types.Log
Logs:
	for _, log := range logs {
		if fromBlock != nil && fromBlock.Int64() >= 0 && fromBlock.Uint64() > log.BlockNumber {
			continue
		}
		if toBlock != nil && toBlock.Int64() >= 0 && toBlock.Uint64() < log.BlockNumber {
			continue
		}

		if len(addresses) > 0 && !includes(addresses, log.Address) {
			continue
		}
		// If the to filtered topics is greater than the amount of topics in logs, skip.
		if len(topics) > len(log.Topics) {
			continue Logs
		}
		for i, topics := range topics {
			match := len(topics) == 0 // empty rule set == wildcard
			for _, topic := range topics {
				if log.Topics[i] == topic {
					match = true
					break
				}
			}
			if !match {
				continue Logs
			}
		}
		ret = append(ret, log)
	}
	return ret
}

func includes(addresses []common.Address, a common.Address) bool {
	for _, addr := range addresses {
		if addr == a {
			return true
		}
	}

	return false
}
