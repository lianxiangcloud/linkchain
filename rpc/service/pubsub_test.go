package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/rpc/rtypes"
)

const (
	defaultRetryInterval = 500 * time.Millisecond

	NamespacePubSub               = "lk"
	MethodGetBlock                = NamespacePubSub + "_getBlock"
	MethodGetTransaction          = NamespacePubSub + "_getTransaction"
	MethodBroadcastTxSync         = NamespacePubSub + "_broadcastTxSync"
	MethodGetLogs                 = NamespacePubSub + "_getLogs"
	MethodGetNonce                = NamespacePubSub + "_getTransactionCount"
	MethodBlockSubscribe          = "blockSubscribe"
	MethodBalanceRecordsSubscribe = "balanceRecordsSubscribe"
)

var (
	ErrEmptyClient = errors.New("emptyClient")
)

type Client struct {
	url    string
	client *rpc.Client
}

func NewClient(peer string) (*Client, error) {
	url := fmt.Sprintf("ws://%s", peer)
	client, err := rpc.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to dial peer(%s): %v", url, err)
	}
	return &Client{
		url:    url,
		client: client,
	}, nil
}

func NewLazyClient(peer string) *Client {
	return &Client{
		url: fmt.Sprintf("ws://%s", peer),
	}
}

func (c *Client) Close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}

func (c *Client) AddTxLocal(tx []byte) error {
	if c == nil || c.client == nil {
		return ErrEmptyClient
	}
	if err := c.client.Call(nil, MethodBroadcastTxSync, tx); err != nil {
		return fmt.Errorf("failed to addtxlocal: %v", err)
	}
	return nil
}

func (c *Client) GetNonce(address common.Address) (uint64, error) {
	if c == nil || c.client == nil {
		return 0, ErrEmptyClient
	}
	var nonce uint64
	if err := c.client.Call(&nonce, MethodGetNonce, address, "latest"); err != nil {
		return 0, fmt.Errorf("failed to GetNonce of addr: %v, err: %v", address, err)
	}
	return nonce, nil
}

func (c *Client) StartBalance(chanBalance chan *rtypes.BalanceRecordsWithBlockHeight) {
	for {
		if err := c.receiveBalanceRoutine(chanBalance); err != nil {
			log.Error("receiveBalanceRoutine", "peer", c.url, "err", err)
			time.Sleep(defaultRetryInterval)
		}
	}
}

func (c *Client) receiveBalanceRoutine(chanBalance chan *rtypes.BalanceRecordsWithBlockHeight) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		// 3. Subscribe
		cliSub, err := c.client.Subscribe(ctx, NamespacePubSub, chanBalance, MethodBalanceRecordsSubscribe)
		if err != nil {
			log.Warn("clients.Subscribe fail", "err", err)
			return fmt.Errorf("failed to subscribe event(%s): %v", MethodBalanceRecordsSubscribe, err)
		}

		log.Info("Client Subscribe successfully", "peer", c.url)
		// 4. for loop to receive notifi and handle it.
		for {
			select {
			case err := <-cliSub.Err(): // got error
				log.Warn("Subscription encounter error", "peer", c.url, "err", err)
				return err
			}
		}

	}
	return nil
}

func (c *Client) Start(chanBlock chan *rtypes.WholeBlock) {
	for {
		if err := c.receiveRoutine(chanBlock); err != nil {
			log.Error("receiveRoutine", "peer", c.url, "err", err)
			time.Sleep(defaultRetryInterval)
		}
	}
}

func (c *Client) receiveRoutine(chanBlock chan *rtypes.WholeBlock) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		// 3. Subscribe
		cliSub, err := c.client.Subscribe(ctx, NamespacePubSub, chanBlock, MethodBlockSubscribe)
		if err != nil {
			log.Warn("clients.Subscribe fail", "err", err)
			return fmt.Errorf("failed to subscribe event(%s): %v", MethodBlockSubscribe, err)
		}

		log.Info("Client Subscribe successfully", "peer", c.url)
		// 4. for loop to receive notifi and handle it.
		for {
			select {
			case err := <-cliSub.Err(): // got error
				log.Warn("Subscription encounter error", "peer", c.url, "err", err)
				return err
			}
		}

	}
	return nil
}

/*

func TestSubscribeBlock(t *testing.T) {
	client, err := NewClient("192.168.126.50:18000")
	require.Nil(t, err)
	chanBlock := make(chan *rtypes.WholeBlock)
	go client.Start(chanBlock)
	for {
		select {
		case block := <-chanBlock:
			log.Debug("recv", "block", block)
		}
	}
}

func TestBalanceRecordsSubscribe(t *testing.T) {
	client, err := NewClient("127.0.0.1:8001")
	require.Nil(t, err)
	chanBalance := make(chan *rtypes.BalanceRecordsWithBlockHeight)
	go client.StartBalance(chanBalance)
	for {
		select {
		case balance := <-chanBalance:
			log.Debug("recv", "block", balance)
		}
	}
}
*/

func (c *Client) GetBlock(num uint64) (*rtypes.WholeBlock, error) {
	if c == nil || c.client == nil {
		return nil, ErrEmptyClient
	}
	resp := &rtypes.WholeBlock{}
	log.Debug("GetBlock", "method", MethodGetBlock, "height", num)
	err := c.client.Call(&resp, MethodGetBlock, new(big.Int).SetUint64(num))
	return resp, err
}

/*
func TestGetBlock(t *testing.T) {
	client, err := NewClient("192.168.126.50:18000")
	require.Nil(t, err)
	block, err := client.GetBlock(1)
	assert.Nil(t, nil, err)
	log.Debug("recv", "block", block)
}
*/

func (c *Client) GetTransaction(txHash string) (*rtypes.RPCTx, error) {
	if c == nil || c.client == nil {
		return nil, ErrEmptyClient
	}
	var resp *rtypes.RPCTx
	log.Debug("GetTransactionInfo", "method", MethodGetTransaction, "tHash", txHash)
	err := c.client.Call(&resp, MethodGetTransaction, txHash)
	if err == nil && resp != nil {
		return resp, nil
	}
	return nil, err
}

/*
func TestGetTransaction(t *testing.T) {
	client, err := NewClient("192.168.126.50:18000")
	require.Nil(t, err)
	tx, err := client.GetTransaction("0x97fbf9e48d5bbe28d2aca97488ecbf7c279776efee2b5c06e15cf3c6e9c1cbd5")
	assert.Nil(t, nil, err)
	log.Debug("recv", "tx", tx)
}

func (c *Client) GetLogs(crit filters.FilterCriteria) ([]*types.Log, error) {
	if c == nil || c.client == nil {
		return nil, ErrEmptyClient
	}

	var result = make([]*types.Log, 0)
	if err := c.client.Call(&result, MethodGetLogs, crit); err != nil {
		log.Error("failed to get logs", "filter", crit, "err", err)
		return nil, err
	}
	return result, nil
}
*/

//func createMultiSignTx(zeroNonce uint64, txType types.SupportType) *types.MultiSignAccountTx {
//	mainInfo := &types.MultiSignMainInfo{
//		AccountNonce:  zeroNonce,
//		SupportTxType: txType,
//		SignersInfo: types.SignersInfo{
//			MinSignerPower: 20,
//			Signers: []*types.SignerEntry{
//				&types.SignerEntry{
//					Power: 10,
//					Addr:  common.HexToAddress("0xff03a9d1d2c7305ad1d0c1f42d62f9140f48a340"),
//				},
//				&types.SignerEntry{
//					Power: 10,
//					Addr:  common.HexToAddress("0xff03a9d1d2c7305ad1d0c1f42d62f9140f48a340"),
//				},
//				&types.SignerEntry{
//					Power: 10,
//					Addr:  common.HexToAddress("0x7fed47ec394776dc9e9c940218d0bf202e91a900"),
//				},
//			},
//		},
//	}
//	return types.NewMultiSignAccountTx(mainInfo, nil)
//}
//
//func TestBroadcastTxSync(t *testing.T) {
//	client, err := NewClient("192.168.126.53:18000")
//	//assert := assert.New(t)
//	require.Nil(t, err)
//	zeroAddr := common.HexToAddress("0x0000000000000000000000000000000000000000")
//	zeroNonce, err := client.GetNonce(zeroAddr)
//	fmt.Println("GetNonce", "zeroNonce", zeroNonce, "error", err)
//	mtx := createMultiSignTx(zeroNonce, types.TxUpdateAddrRouteType)
//	pvSignMultiSignTx(mtx)
//	data, err := ser.EncodeToBytes(mtx)
//	require.Nil(t, err)
//	client.AddTxLocal(data)
//	//
//	mtx = createMultiSignTx(zeroNonce, types.TxUpdateValidatorsType)
//	pvSignMultiSignTx(mtx)
//	data, err = ser.EncodeToBytes(mtx)
//	require.Nil(t, err)
//	client.AddTxLocal(data)
//	//
//	mtx = createMultiSignTx(zeroNonce, types.TxContractCreateType)
//	pvSignMultiSignTx(mtx)
//	data, err = ser.EncodeToBytes(mtx)
//	require.Nil(t, err)
//	client.AddTxLocal(data)
//}
