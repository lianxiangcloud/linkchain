//go:generate mockgen -destination mock_nodeapi.go -package wallet -self_package github.com/lianxiangcloud/linkchain/wallet/wallet github.com/lianxiangcloud/linkchain/wallet/wallet BackendAPI

package wallet

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/lianxiangcloud/linkchain/libs/common"
	lktypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	rtypes "github.com/lianxiangcloud/linkchain/rpc/rtypes"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/wallet/daemon"
	wtypes "github.com/lianxiangcloud/linkchain/wallet/types"
)

//BackendAPI for node
type BackendAPI interface {
	EthGetTransactionCount(addr common.Address) (*uint64, error)
	RefreshMaxBlock() (*big.Int, error)
	GetOutputsFromNode(indice []uint64, tokenID common.Address) ([]*types.UTXORingEntry, error)
	IsContract(addr common.Address) (bool, error)
	EstimateGas(from common.Address, nonce uint64, dest *types.AccountDestEntry, kind types.UTXOKind, tokenID common.Address) (*big.Int, error)
	GetTokenBalance(addr common.Address, tokenID common.Address) (*big.Int, error)
	Transfer(txs []string) (ret []wtypes.SendTxRet)
	GetChainVersion() (string, error)
	GetBlockUTXOsByNumber(height *big.Int) (*rtypes.RPCBlock, error)
	GetUTXOGas() (uint64, error)
	GetBlockTransactionCountByNumber(blockNr rpc.BlockNumber) (*hexutil.Uint, error)
	GetBlockTransactionCountByHash(blockHash common.Hash) (*hexutil.Uint, error)
	GetTransactionByBlockNumberAndIndex(blockNr rpc.BlockNumber, index hexutil.Uint) (r interface{}, err error)
	GetTransactionByBlockHashAndIndex(blockHash common.Hash, index hexutil.Uint) (r interface{}, err error)
	GetRawTransactionByBlockNumberAndIndex(blockNr rpc.BlockNumber, index hexutil.Uint) (r hexutil.Bytes, err error)
	GetRawTransactionByBlockHashAndIndex(blockHash common.Hash, index hexutil.Uint) (r hexutil.Bytes, err error)
	GetTransactionCount(address common.Address, blockNr rpc.BlockNumber) (*hexutil.Uint64, error)
	GetTransactionByHash(hash common.Hash) (r interface{}, err error)
	GetRawTransactionByHash(hash common.Hash) (r hexutil.Bytes, err error)
	GetTransactionReceipt(hash common.Hash) (r map[string]interface{}, err error)
	EthEstimateGas(args wtypes.CallArgs) (*hexutil.Uint64, error)
	SendRawTransaction(encodedTx hexutil.Bytes) (common.Hash, error)
	SendRawUTXOTransaction(encodedTx hexutil.Bytes) (common.Hash, error)
	GetBlockUTXO(height *big.Int) (*rtypes.QuickRPCBlock, error)
	GenesisBlockNumber() (*hexutil.Uint64, error)
	Call(args wtypes.CallArgs, blockNr string) (*hexutil.Bytes, error)
}

//NodeAPI implementation for API
type NodeAPI struct{}

// EthGetTransactionCount return eth account balance
func (api *NodeAPI) EthGetTransactionCount(addr common.Address) (*uint64, error) {
	p := make([]interface{}, 2)
	p[0] = addr
	p[1] = "latest"
	body, err := daemon.CallJSONRPC("eth_getTransactionCount", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("EthGetTransactionCount json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}
	var nonce hexutil.Uint64
	if err = json.Unmarshal(jsonRes.Result, &nonce); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}

	uNonce := uint64(nonce)

	return &uNonce, nil
}

// RefreshMaxBlock wallet
func (api *NodeAPI) RefreshMaxBlock() (*big.Int, error) {
	body, err := daemon.CallJSONRPC("eth_blockNumber", nil)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("RefreshMaxBlock json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}
	var h hexutil.Big
	if err = ser.UnmarshalJSON(jsonRes.Result, &h); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}

	return (*big.Int)(&h), nil
}

// OutputArg -
type OutputArg struct {
	Token common.Address `json:"token"`
	Index hexutil.Uint64 `json:"index"`
}

// RPCKey -
type RPCKey lktypes.Key

// RPCOutput -
type RPCOutput struct {
	Out    string `json:"out"`
	Commit string `json:"commit"`
}

func (api *NodeAPI) GetOutputsFromNode(indice []uint64, tokenID common.Address) ([]*types.UTXORingEntry, error) {
	if 0 == len(indice) {
		return nil, nil
	}
	ops := make([]OutputArg, len(indice))
	for i, idx := range indice {
		op := OutputArg{
			Token: tokenID,
			Index: hexutil.Uint64(idx),
		}
		ops[i] = op
	}

	p := make([]interface{}, 1)
	p[0] = ops
	body, err := daemon.CallJSONRPC("eth_getOutputs", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetOutputsFromNode json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}
	var outputs []*RPCOutput
	if err = ser.UnmarshalJSON(jsonRes.Result, &outputs); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}

	if len(outputs) != len(indice) {
		return nil, wtypes.ErrDaemonResponseData
	}
	ringEntries := make([]*types.UTXORingEntry, len(indice))
	for i := 0; i < len(indice); i++ {
		key, err := hex.DecodeString(outputs[i].Out)
		if err != nil {
			return nil, wtypes.ErrInnerServer
		}
		var otaddr lktypes.Key
		copy(otaddr[:], key)
		key, err = hex.DecodeString(outputs[i].Commit)
		if err != nil {
			return nil, wtypes.ErrInnerServer
		}
		var mask lktypes.Key
		copy(mask[:], key)
		ringEntry := &types.UTXORingEntry{
			Index:  indice[i],
			OTAddr: otaddr,
			Commit: mask,
		}
		ringEntries[i] = ringEntry
	}
	return ringEntries, nil
}

func (api *NodeAPI) IsContract(addr common.Address) (bool, error) {
	p := make([]interface{}, 2)
	p[0] = addr.Hex()
	p[1] = "latest"
	body, err := daemon.CallJSONRPC("eth_getCode", p)
	if err != nil || body == nil || len(body) == 0 {
		return false, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return false, fmt.Errorf("isContract json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return false, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return false, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return false, wtypes.ErrDaemonResponseCode
	}
	var code hexutil.Bytes
	if err = ser.UnmarshalJSON(jsonRes.Result, &code); err != nil {
		//return false, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return false, wtypes.ErrDaemonResponseData
	}
	if len(code) > 2 {
		return true, nil
	}
	return false, nil
}

//limit contract fee > 1e11 and tx fee mod 1e11 == 0
func (api *NodeAPI) EstimateGas(from common.Address, nonce uint64, dest *types.AccountDestEntry, kind types.UTXOKind, tokenID common.Address) (*big.Int, error) {
	req := make(map[string]interface{})
	req["from"] = from.Hex()
	req["to"] = dest.To.Hex()
	if len(dest.Data) > 0 {
		req["data"] = fmt.Sprintf("0x%x", dest.Data)
	} else {
		req["data"] = "0x"
	}
	req["value"] = hexutil.EncodeBig(dest.Amount)
	req["nonce"] = fmt.Sprintf("0x%s", strconv.FormatUint(nonce, 16))
	req["tokenAddress"] = tokenID.Hex()

	body, err := daemon.CallJSONRPC("eth_estimateGas", []interface{}{req})
	if err != nil || body == nil || len(body) == 0 {
		return big.NewInt(0), wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return big.NewInt(0), fmt.Errorf("EstimateGas json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return big.NewInt(0), wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return big.NewInt(0), fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return big.NewInt(0), wtypes.ErrDaemonResponseCode
	}
	var gas hexutil.Uint64
	if err = ser.UnmarshalJSON(jsonRes.Result, &gas); err != nil {
		//return big.NewInt(0), fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return big.NewInt(0), wtypes.ErrDaemonResponseData
	}
	return big.NewInt(0).Mul(big.NewInt(0).SetUint64(uint64(gas)), big.NewInt(1e11)), nil
}

func (api *NodeAPI) GetTokenBalance(addr common.Address, tokenID common.Address) (*big.Int, error) {
	body, err := daemon.CallJSONRPC("eth_getTokenBalance", []interface{}{addr.Hex(), "latest", tokenID.Hex()})
	if err != nil || body == nil || len(body) == 0 {
		return big.NewInt(0), wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return big.NewInt(0), fmt.Errorf("GetTokenBalance json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return big.NewInt(0), wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return big.NewInt(0), fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return big.NewInt(0), wtypes.ErrDaemonResponseCode
	}
	var balance hexutil.Big
	if err = ser.UnmarshalJSON(jsonRes.Result, &balance); err != nil {
		//return big.NewInt(0), fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return big.NewInt(0), wtypes.ErrDaemonResponseData
	}
	return (*big.Int)(&balance), nil
}

// Transfer wallet
func (api *NodeAPI) Transfer(txs []string) (ret []wtypes.SendTxRet) {
	txCnt := len(txs)

	ret = make([]wtypes.SendTxRet, txCnt)
	for i := 0; i < txCnt; i++ {
		p := make([]interface{}, 1)
		p[0] = txs[i]

		body, err := daemon.CallJSONRPC("eth_sendRawUTXOTransaction", p)
		if err != nil || body == nil || len(body) == 0 {
			ret[i] = wtypes.SendTxRet{Raw: txs[i], Hash: common.EmptyHash, ErrCode: -1, ErrMsg: fmt.Sprintf("%v", err)}
			continue
		}
		var jsonRes wtypes.RPCResponse
		if err = json.Unmarshal(body, &jsonRes); err != nil {
			ret[i] = wtypes.SendTxRet{Raw: txs[i], Hash: common.EmptyHash, ErrCode: -1, ErrMsg: err.Error()}
			continue
		}
		if jsonRes.Error.Code != 0 {
			ret[i] = wtypes.SendTxRet{Raw: txs[i], Hash: common.EmptyHash, ErrCode: jsonRes.Error.Code, ErrMsg: jsonRes.Error.Message}
			continue
		}
		var hash common.Hash

		if err = json.Unmarshal(jsonRes.Result, &hash); err != nil {
			ret[i] = wtypes.SendTxRet{Raw: txs[i], Hash: common.EmptyHash, ErrCode: -1, ErrMsg: err.Error()}
			continue
		}
		ret[i] = wtypes.SendTxRet{Raw: txs[i], Hash: hash, ErrCode: 0, ErrMsg: ""}
	}

	return
}

func (api *NodeAPI) GetChainVersion() (string, error) {
	body, err := daemon.CallJSONRPC("eth_getChainVersion", []interface{}{})
	if err != nil || body == nil || len(body) == 0 {
		return "", wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return "", fmt.Errorf("GetChainVersion json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return "", wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return "", fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return "", wtypes.ErrDaemonResponseCode
	}
	var peerVersion string
	if err = ser.UnmarshalJSON(jsonRes.Result, &peerVersion); err != nil {
		//return "", fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return "", wtypes.ErrDaemonResponseData
	}
	return peerVersion, nil
}

func (api *NodeAPI) GetBlockUTXOsByNumber(height *big.Int) (*rtypes.RPCBlock, error) {
	p := make([]interface{}, 2)
	p[0] = hexutil.EncodeBig(height)
	p[1] = true
	body, err := daemon.CallJSONRPC("eth_getBlockUTXOsByNumber", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetBlockUTXOsByNumber json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}
	log.Debug("GetBlockUTXOsByNumber", "jsonRes.Result", string(jsonRes.Result))

	var block rtypes.RPCBlock
	if err = json.Unmarshal(jsonRes.Result, &block); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}

	return &block, nil
}

func (api *NodeAPI) GetUTXOGas() (uint64, error) {
	body, err := daemon.CallJSONRPC("eth_getUTXOGas", []interface{}{})
	if err != nil || body == nil || len(body) == 0 {
		return 0, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return 0, fmt.Errorf("getUTXOGas json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return 0, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return 0, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return 0, wtypes.ErrDaemonResponseCode
	}
	var utxoGas hexutil.Uint64
	if err = ser.UnmarshalJSON(jsonRes.Result, &utxoGas); err != nil {
		//return 0, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return 0, wtypes.ErrDaemonResponseData
	}
	return uint64(utxoGas), nil
}

// GetBlockTransactionCountByNumber return block transaction count
func (api *NodeAPI) GetBlockTransactionCountByNumber(blockNr rpc.BlockNumber) (*hexutil.Uint, error) {
	p := make([]interface{}, 1)
	p[0] = blockNr.String()

	body, err := daemon.CallJSONRPC("eth_getBlockTransactionCountByNumber", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetBlockTransactionCountByNumber json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}
	var cnt hexutil.Uint
	if err = ser.UnmarshalJSON(jsonRes.Result, &cnt); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}
	return (*hexutil.Uint)(&cnt), nil
}

// GetBlockTransactionCountByHash return block transaction count
func (api *NodeAPI) GetBlockTransactionCountByHash(blockHash common.Hash) (*hexutil.Uint, error) {
	p := make([]interface{}, 1)
	p[0] = blockHash

	body, err := daemon.CallJSONRPC("eth_getBlockTransactionCountByHash", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetBlockTransactionCountByHash json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}
	var cnt hexutil.Uint
	if err = ser.UnmarshalJSON(jsonRes.Result, &cnt); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}
	return (*hexutil.Uint)(&cnt), nil
}

// GetTransactionByBlockNumberAndIndex return rpc tx
func (api *NodeAPI) GetTransactionByBlockNumberAndIndex(blockNr rpc.BlockNumber, index hexutil.Uint) (r interface{}, err error) {
	p := make([]interface{}, 2)
	p[0] = blockNr.String()
	p[1] = index.String()

	body, err := daemon.CallJSONRPC("eth_getTransactionByBlockNumberAndIndex", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetTransactionByBlockNumberAndIndex json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}
	if string(jsonRes.Result) == "null" {
		return nil, nil
	}
	var tx rtypes.RPCTx
	// w.Logger.Debug("GetTransactionByBlockNumberAndIndex", "result", string(jsonRes.Result), "body", string(body))
	if err = json.Unmarshal(jsonRes.Result, &tx); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}

	return &tx, nil
}

// GetTransactionByBlockHashAndIndex return tx
func (api *NodeAPI) GetTransactionByBlockHashAndIndex(blockHash common.Hash, index hexutil.Uint) (r interface{}, err error) {
	p := make([]interface{}, 2)
	p[0] = blockHash.String()
	p[1] = index.String()

	body, err := daemon.CallJSONRPC("eth_getTransactionByBlockHashAndIndex", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetTransactionByBlockHashAndIndex json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}

	if string(jsonRes.Result) == "null" {
		return nil, nil
	}

	var tx rtypes.RPCTx
	if err = json.Unmarshal(jsonRes.Result, &tx); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}
	return &tx, nil
}

// GetRawTransactionByBlockNumberAndIndex (ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) hexutil.Bytes
func (api *NodeAPI) GetRawTransactionByBlockNumberAndIndex(blockNr rpc.BlockNumber, index hexutil.Uint) (r hexutil.Bytes, err error) {
	p := make([]interface{}, 2)
	p[0] = blockNr.String()
	p[1] = index.String()

	body, err := daemon.CallJSONRPC("eth_getRawTransactionByBlockNumberAndIndex", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetRawTransactionByBlockNumberAndIndex json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}

	if err = json.Unmarshal(jsonRes.Result, &r); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}
	return r, nil
}

// GetRawTransactionByBlockHashAndIndex (ctx context.Context, blockHash common.Hash, index hexutil.Uint) hexutil.Bytes
func (api *NodeAPI) GetRawTransactionByBlockHashAndIndex(blockHash common.Hash, index hexutil.Uint) (r hexutil.Bytes, err error) {
	p := make([]interface{}, 2)
	p[0] = blockHash.String()
	p[1] = index.String()

	body, err := daemon.CallJSONRPC("eth_getRawTransactionByBlockHashAndIndex", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetRawTransactionByBlockHashAndIndex json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}

	if err = json.Unmarshal(jsonRes.Result, &r); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}
	return r, nil
}

// GetTransactionCount (ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*hexutil.Uint64, error)
func (api *NodeAPI) GetTransactionCount(address common.Address, blockNr rpc.BlockNumber) (*hexutil.Uint64, error) {
	p := make([]interface{}, 2)
	p[0] = address.String()
	p[1] = blockNr.String()
	body, err := daemon.CallJSONRPC("eth_getTransactionCount", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetTransactionCount json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrGetTransactionCount
	}
	var nonce hexutil.Uint64
	if err = json.Unmarshal(jsonRes.Result, &nonce); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}

	return &nonce, nil
}

// GetTransactionByHash (ctx context.Context, hash common.Hash) interface{}
func (api *NodeAPI) GetTransactionByHash(hash common.Hash) (r interface{}, err error) {
	p := make([]interface{}, 1)
	p[0] = hash.String()

	body, err := daemon.CallJSONRPC("eth_getTransactionByHash", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetTransactionByHash json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}

	var tx rtypes.RPCTx
	if err = json.Unmarshal(jsonRes.Result, &tx); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}
	return &tx, nil
}

// GetRawTransactionByHash (ctx context.Context, hash common.Hash) (hexutil.Bytes, error)
func (api *NodeAPI) GetRawTransactionByHash(hash common.Hash) (r hexutil.Bytes, err error) {
	p := make([]interface{}, 1)
	p[0] = hash.String()

	body, err := daemon.CallJSONRPC("eth_getRawTransactionByHash", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetRawTransactionByHash json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}

	if err = json.Unmarshal(jsonRes.Result, &r); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}
	return r, nil
}

// GetTransactionReceipt (ctx context.Context, hash common.Hash) (map[string]interface{}, error)
func (api *NodeAPI) GetTransactionReceipt(hash common.Hash) (r map[string]interface{}, err error) {
	p := make([]interface{}, 1)
	p[0] = hash.String()

	body, err := daemon.CallJSONRPC("eth_getTransactionReceipt", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetTransactionReceipt json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}

	if err = json.Unmarshal(jsonRes.Result, &r); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}
	return r, nil
}

//EthEstimateGas limit contract fee > 1e11 and tx fee mod 1e11 == 0
func (api *NodeAPI) EthEstimateGas(args wtypes.CallArgs) (*hexutil.Uint64, error) {
	if args.Value.ToInt().Sign() < 0 {
		return nil, wtypes.ErrArgsInvalid
	}
	if args.UTXOKind == types.AinUout {
		gasLimit := types.CalNewAmountGas(args.Value.ToInt(), types.EverLiankeFee)
		return (*hexutil.Uint64)(&gasLimit), nil
	}

	req := make(map[string]interface{})
	req["from"] = args.From
	req["tokenAddress"] = args.TokenAddress
	if args.To != nil {
		req["to"] = *args.To
	}
	if args.Gas > 0 {
		req["gas"] = args.Gas
	}
	if args.GasPrice.ToInt().Cmp(big.NewInt(0)) > 0 {
		req["gasPrice"] = args.GasPrice
	}
	req["value"] = args.Value

	req["data"] = args.Data.String()
	req["nonce"] = args.Nonce

	//support estimate gas from UTXOTransition

	body, err := daemon.CallJSONRPC("eth_estimateGas", []interface{}{req})
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("EthEstimateGas json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrEstimateGas
	}
	var gas hexutil.Uint64
	if err = ser.UnmarshalJSON(jsonRes.Result, &gas); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}
	return &gas, nil
}

// SendRawTransaction wallet
func (api *NodeAPI) SendRawTransaction(encodedTx hexutil.Bytes) (common.Hash, error) {
	p := make([]interface{}, 1)
	p[0] = encodedTx

	body, err := daemon.CallJSONRPC("eth_sendRawTransaction", p)
	if err != nil || body == nil || len(body) == 0 {
		//return common.EmptyHash, fmt.Errorf("CallJSONRPC fail,err:%v", err)
		return common.EmptyHash, wtypes.ErrDaemonResponseBody
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return common.EmptyHash, fmt.Errorf("SendRawTransaction json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return common.EmptyHash, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return common.EmptyHash, fmt.Errorf("CallJSONRPC check jsonRes.Error.Code,err:%v", jsonRes.Error)
		return common.EmptyHash, wtypes.ErrSendRawTransaction
	}
	var hash common.Hash

	if err = json.Unmarshal(jsonRes.Result, &hash); err != nil {
		//return common.EmptyHash, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return common.EmptyHash, wtypes.ErrDaemonResponseData
	}

	return hash, nil
}

// SendRawUTXOTransaction wallet
func (api *NodeAPI) SendRawUTXOTransaction(encodedTx hexutil.Bytes) (common.Hash, error) {
	p := make([]interface{}, 1)
	p[0] = encodedTx

	body, err := daemon.CallJSONRPC("eth_sendRawUTXOTransaction", p)
	if err != nil || body == nil || len(body) == 0 {
		//return common.EmptyHash, fmt.Errorf("CallJSONRPC fail,err:%v", err)
		return common.EmptyHash, wtypes.ErrDaemonResponseBody
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return common.EmptyHash, fmt.Errorf("SendRawUTXOTransaction json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return common.EmptyHash, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return common.EmptyHash, fmt.Errorf("CallJSONRPC check jsonRes.Error.Code,err:%v", jsonRes.Error)
		return common.EmptyHash, wtypes.ErrSendRawUTXOTransaction
	}
	var hash common.Hash

	if err = json.Unmarshal(jsonRes.Result, &hash); err != nil {
		//return common.EmptyHash, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return common.EmptyHash, wtypes.ErrDaemonResponseData
	}

	return hash, nil
}

// GetBlockUTXO -
// curl -X POST --data '{"jsonrpc":"2.0","method":"eth_getBlockUTXO","params":["0x1a"],"id":1}' https://pocketapi-lianxiangcloud.com/getBlockUTXO
func (api *NodeAPI) GetBlockUTXO(height *big.Int) (*rtypes.QuickRPCBlock, error) {
	p := make([]interface{}, 1)
	p[0] = hexutil.EncodeBig(height)
	body, err := daemon.CallJSONRPC("eth_getBlockUTXO", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}

	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GetBlockUTXO json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}

	var quickBlock rtypes.QuickRPCBlock
	if err = json.Unmarshal(jsonRes.Result, &quickBlock); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}

	return &quickBlock, nil
}

// GenesisBlockNumber return genesisBlock init height
func (api *NodeAPI) GenesisBlockNumber() (*hexutil.Uint64, error) {
	p := make([]interface{}, 0)
	body, err := daemon.CallJSONRPC("eth_genesisBlockNumber", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}

	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		//return nil, fmt.Errorf("GenesisBlockNumber json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		//return nil, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		return nil, wtypes.ErrDaemonResponseCode
	}

	var blockNumber hexutil.Uint64
	if err = json.Unmarshal(jsonRes.Result, &blockNumber); err != nil {
		//return nil, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		return nil, wtypes.ErrDaemonResponseData
	}

	return &blockNumber, nil
}

//Call -
func (api *NodeAPI) Call(args wtypes.CallArgs, blockNr string) (*hexutil.Bytes, error) {
	p := make([]interface{}, 2)
	p[0] = args
	p[1] = blockNr
	body, err := daemon.CallJSONRPC("eth_call", p)
	if err != nil || body == nil || len(body) == 0 {
		return nil, wtypes.ErrNoConnectionToDaemon
	}
	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		return nil, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		return nil, wtypes.ErrDaemonResponseCode
	}
	var ret hexutil.Bytes
	if err = ser.UnmarshalJSON(jsonRes.Result, &ret); err != nil {
		return nil, wtypes.ErrDaemonResponseData
	}
	return &ret, nil
}

func (api *NodeAPI) GetUTXOChangeRate(addr common.Address) (int64, error) {
	p := make([]interface{}, 1)
	p[0] = addr
	body, err := daemon.CallJSONRPC("eth_getUTXOChangeRate", p)
	if err != nil || body == nil || len(body) == 0 {
		return 0, wtypes.ErrNoConnectionToDaemon
	}

	var jsonRes wtypes.RPCResponse
	if err = json.Unmarshal(body, &jsonRes); err != nil {
		return 0, fmt.Errorf("GetUTXOChangeRate json.Unmarshal(body, &jsonRes) fail, err:%v, body:%s", err, string(body))
		//return 0, wtypes.ErrDaemonResponseBody
	}
	if jsonRes.Error.Code != 0 {
		return 0, fmt.Errorf("json RPC error:%v,body:[%s]", jsonRes.Error, string(body))
		//return 0, wtypes.ErrDaemonResponseCode
	}

	var rate hexutil.Uint64
	if err = json.Unmarshal(jsonRes.Result, &rate); err != nil {
		return 0, fmt.Errorf("json.Unmarshal jsonRes.Result fail, err:%v, body:%s", err, string(body))
		//return 0, wtypes.ErrDaemonResponseData
	}

	return int64(rate), nil
}
