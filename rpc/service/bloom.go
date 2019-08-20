package service

import (
	"context"
	"fmt"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/bitutil"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/rpc/bloom"
	"github.com/lianxiangcloud/linkchain/types"
)

const (
	// bloomServiceThreads is the number of goroutines used globally by an Ethereum
	// instance to service bloombits lookups for all running filters.
	bloomServiceThreads = 16

	// bloomFilterThreads is the number of goroutines used locally per filter to
	// multiplex requests onto the global servicing goroutines.
	bloomFilterThreads = 3

	// bloomRetrievalBatch is the maximum number of bloom bit retrievals to service
	// in a single batch.
	bloomRetrievalBatch = 16

	// bloomRetrievalWait is the maximum time to wait for enough bloom bit requests
	// to accumulate request an entire batch (avoiding hysteresis).
	bloomRetrievalWait = time.Duration(0)
)

type BloomService struct {
	basic        *Service
	chainIndexer *bloom.ChainIndexer
	quit         chan struct{}
}

func NewBloomService(basic *Service) *BloomService {
	return &BloomService{
		basic:        basic,
		chainIndexer: bloom.NewBloomIndexer(basic.context().blockStore.GetDB(), types.BloomBitsBlocks, basic.apiBackend()),
		quit:         make(chan struct{}),
	}
}

func (bs *BloomService) backend() *ApiBackend {
	return bs.basic.apiBackend()
}

func (bs *BloomService) Start() error {
	header, err := bs.backend().HeaderByHeight(context.Background(), rpc.LatestBlockNumber)
	if err != nil {
		return err
	}

	bs.chainIndexer.Start(bs.basic.context().eventBus, header)
	go bs.startBloomHandlers()

	return nil
}

func (bs *BloomService) Stop() error {
	bs.quit <- struct{}{}
	return bs.chainIndexer.Close()
}

// startBloomHandlers starts a batch of goroutines to accept bloom bit database
// retrievals from possibly a range of filters and serving the data to satisfy.
func (bs *BloomService) startBloomHandlers() {
	for i := 0; i < bloomServiceThreads; i++ {
		go func() {
			db := bs.basic.context().blockStore.GetDB()
			for {
				select {
				case <-bs.quit:
					return

				case request := <-bs.backend().bloomRequests:
					task := <-request
					task.Bitsets = make([][]byte, len(task.Sections))
					for i, section := range task.Sections {
						//head := core.GetCanonicalHash(eth.chainDb, (section+1)*types.BloomBitsBlocks-1)
						header, err := bs.backend().HeaderByHeight(nil, rpc.BlockNumber((section+1)*types.BloomBitsBlocks-1))
						if err != nil {
							task.Error = err
						} else {
							if compVector := bloom.GetBloomBits(db, task.Bit, section, header.Hash()); compVector != nil {
								if blob, err := bitutil.DecompressBytes(compVector, int(types.BloomBitsBlocks)/8); err == nil {
									task.Bitsets[i] = blob
								} else {
									task.Error = err
								}
							} else {
								task.Error = fmt.Errorf("NotExist")
							}
						}
					}
					request <- task
				}
			}
		}()
	}
}
