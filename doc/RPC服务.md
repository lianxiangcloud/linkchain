## RPC接口

- [eth_accounts](#eth_accounts)
- [eth_coinbase](#eth_coinbase)
- [eth_getBalance](#eth_getbalance)
- [eth_getTokenBalance](#eth_gettokenbalance)
- [eth_getUTXOGas](#eth_getutxogas)
- [eth_sendRawUTXOTransaction](#eth_sendrawutxotransaction)
- [eth_getMaxOutputIndex](#eth_getmaxoutputindex)
- [eth_getOutputs](#eth_getoutputs)
- [eth_getBlockUTXOsByNumber](#eth_getblockutxosbynumber)
- [eth_estimateGas](#eth_estimategas)
- [eth_sendTransaction](#eth_sendtransaction)
- [eth_sendRawTransaction](#eth_sendrawtransaction)
- [eth_call](#eth_call)
- [eth_getTransactionByHash](#eth_gettransactionbyhash)
- [eth_getTransactionReceipt](#eth_gettransactionreceipt)
- [eth_blockNumber](#eth_blocknumber)
- [eth_getBlockByNumber](#eth_getblockbynumber)
- [eth_getBlockByHash](#eth_getblockbyhash)
- [eth_getTransactionCount](#eth_gettransactioncount)
- [eth_getBlockTransactionCountByNumber](#eth_getblocktransactioncountbynumber)
- [eth_getBlockTransactionCountByHash](#eth_getblocktransactioncountbyhash)
- [eth_getTransactionByBlockNumberAndIndex](#eth_gettransactionbyblocknumberandindex)
- [eth_getTransactionByBlockHashAndIndex](#eth_gettransactionbyblockhashandindex)
- [eth_getBlockBalanceRecordsByNumber](#eth_getblockbalancerecordsbynumber)
- [personal_newAccount](#personal_newaccount)
- [personal_lockAccount](#personal_lockaccount)
- [personal_unlockAccount](#personal_unlockaccount)

----

### eth_accounts
查看当前客户端拥有的普通账户地址列表

#### 参数
- 无

#### 返回
- 地址数组

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_accounts","params":[]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": ["0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5", "0x82c3a9d971fbb999ed1b541aa1ac65793a368bb5", "0x61ff8903116306edba4f38e8e91881555f306e55"]
}
```

### eth_coinbase
查询挖矿奖励账户

#### 参数
- 无

#### 返回
- `string` 接收挖矿奖励的账户地址

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_coinbase","params":[]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": "0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5"
}
```

### eth_getBalance
查询普通账户或者合约账户的链克余额

#### 参数
1. `string` 要查询的地址
2. `string` 16进制块高，或填 `latest`，`earliest`，`pending`

#### 返回
- `string` 余额，16进制字符串

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getBalance","params":["0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5","latest"]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": "0x1ed09bead87c0378d8e640000002a"
}
```

### eth_getTokenBalance
查询普通账户或者合约账户的Token余额

#### 参数
1. `string` 要查询的地址
2. `string` 16进制块高，或填 `latest`，`earliest`，`pending`
3. `string` Token地址，全0地址 `0x0000000000000000000000000000000000000000` 表示链克

#### 返回
- `string` 余额，16进制字符串

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getTokenBalance","params":["0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5","latest","0x0000000000000000000000000000000000000000"]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": "0x1ed09bead87c036dbeba7d13aff2a"
}
```

### eth_getUTXOGas
查询UTXO交易的Gas值

#### 参数
- 无

#### 返回
- `string` Gas，16进制字符串

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getUTXOGas","params":[]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": "0x7a120"
}
```

### eth_sendRawUTXOTransaction
发送UTXO交易

#### 参数
1. `string` 签名后的交易数据，以0x开头

#### 返回
- `string` 交易Hash

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_sendRawUTXOTransaction","params":["0xde6a3..."]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": "0xd6e48158472848e6687173a91ae6eebfa3e1d778e65252ee99d7515d63090408"
}
```

### eth_getMaxOutputIndex
按token查询当前最大的Output索引

#### 参数
1. token `string` 要查询的Token的地址，查链克时填 `0x0000000000000000000000000000000000000000`

#### 返回
- `string` 16进制字符串，token对应的最大Output索引

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getMaxOutputIndex","params":["0x0000000000000000000000000000000000000000"]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": "0x20"
}
```

### eth_getOutputs
查询UTXO交易的Output信息

#### 参数
1. `object`数组
    - token `string` 要查询的Token的地址，查链克时填 `0x0000000000000000000000000000000000000000`
    - index `string` 要查询的索引，16进制字符串

#### 返回
- `object`数组
    - height `int` 该Output所属的区块高度
    - out `string` 接收人
    - commit `string` 混淆后的金额
    - token `string`

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getOutputs","params":[[{"token":"0x0000000000000000000000000000000000000000","index":"0x1"}]]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": [{
        "out": "0000000000000000000000000000000000000000000000000000000000000000",
        "height": 10,
        "commit": "0000000000000000000000000000000000000000000000000000000000000000",
        "token": "0x0000000000000000000000000000000000000000"
    }]
}
```

### eth_getBlockUTXOsByNumber
根据区块高度查询块中的UTXO交易

#### 参数
- 参考 [eth_getBlockByNumber](#eth_getblockbynumber)

#### 返回
- 参考 [eth_getBlockByNumber](#eth_getblockbynumber)

#### 示例
- 参考 [eth_getBlockByNumber](#eth_getblockbynumber)

### eth_estimateGas
估算交易手续费

#### 参数
1. 交易数据，参考 [eth_call](#eth_call)

#### 返回
- `string` 16进制字符串，估算交易需要消耗的Gas值

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_estimateGas","params":[{"from":"0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5","to":"0x82c3a9d971fbb999ed1b541aa1ac65793a368bb5","value":"0x100"}]}' http://127.0.0.1:8000

{
    "jsonrpc":"2.0",
    "id":"0",
    "result":"0x7a120"
}
```

### eth_sendTransaction
发送普通交易和合约交易到区块链，From账户需先解锁

#### 参数
1. `object`
    - from `string` 交易发送者的地址
    - to `string` 交易接收者的地址
    - gas `string` 手续费，可选
    - gasPrice `string` 手续费单价，可选
    - tokenAddress `string` 要交易的token，默认为链克交易，可选
    - value `string` 交易金额，可选
    - data `string` 要执行的合约函数的签名和编码后的参数，可选
    - nonce `string` 16进制字符串，发送者的nonce值，可选

#### 返回
- `string` 交易Hash

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"personal_unlockAccount","params":["0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5","1234",3600]}' http://127.0.0.1:8000

{
    "jsonrpc":"2.0",
    "id":"0",
    "result":true
}

curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_sendTransaction","params":[{"from":"0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5","to":"0x82c3a9d971fbb999ed1b541aa1ac65793a368bb5","value":"0x100"}]}' http://127.0.0.1:8000

{
    "jsonrpc":"2.0",
    "id":"0",
    "result":"0xe5e416d32207ef3dea8d9086425f835592e7171c70cf518ecd4322610934dd74"
}
```

### eth_sendRawTransaction
发送签名后的普通交易和合约交易到区块链

#### 参数
1. `string` 签名后的交易，0x开头的字符串

#### 返回
- `string` 交易Hash

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_sendRawTransaction","params":["0xd46e8dd6..."]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": "0xfd7158eee4d6951b8624cc10d8ea18fb5ef6aec93807be68ed56582b9a4830c0"
}
```

### eth_call
通过执行合约来查询合约上的数据，只读操作

#### 参数
1. `object`
    - from `string` 发送请求的地址，可选
    - to `string` 要查询的合约地址
    - gas `string` 手续费，`eth_call` 不需要支付手续费，可选
    - gasPrice `string` 手续费单价，可选
    - tokenAddress `string` 要交易的token，默认为链克交易，可选
    - value `string` 交易金额，可选
    - data `string` 要执行的合约函数的签名和编码后的参数，可选
2. `string` 16进制块高，或填 `latest`，`earliest`，`pending`

#### 返回
- 合约执行结果

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_call","params":[{"to":"0x4c4743ed913adcdbd783248834f2bd136052b023","data":"0xd4fc9fc60000000000000000000000003002424b2dF8E8227d83A9b504D3C2578a557328"},"latest"]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": "0x0000000000000000000000000000000000000000000000001bc16d674ec80000"
}
```

### eth_getTransactionByHash
根据交易Hash查询交易

#### 参数
1. `string` 交易Hash

#### 返回
- `object` 交易信息，交易不存在时返回`null`
    - txType `string` 交易类型
    - txHash `string` 交易Hash
    - tx `object` 交易对象

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getTransactionByHash","params":["0xe5e416d32207ef3dea8d9086425f835592e7171c70cf518ecd4322610934dd74"]}' http://127.0.0.1:8000

{
    "jsonrpc":"2.0",
    "id":"0",
    "result":{
        "txType":"tx",
        "txHash":"0xe5e416d32207ef3dea8d9086425f835592e7171c70cf518ecd4322610934dd74",
        "signHash":"0x08d6cfb4ef69e5d02a7538865150d078a9e0dd229e4ee382558ae57d7aadbc54",
        "from":"0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5",
        "tx":{
            "type":"tx",
            "value":{
                "nonce":"0x0",
                "gasPrice":"0x174876e800",
                "gas":"0x7a120",
                "to":"0x82c3a9d971fbb999ed1b541aa1ac65793a368bb5",
                "value":"0x100",
                "input":"0x",
                "v":"0xe3e6",
                "r":"0x5587e56cd3261204002b0faf623d8fea47dc9ed6b6e8aad8aed78ac11d532a96",
                "s":"0x473b86e381cf0146d41996b12c5f3e7cf86a98666ee58d0880923776cdf469ca",
                "hash":"0xe5e416d32207ef3dea8d9086425f835592e7171c70cf518ecd4322610934dd74"
            }
        },
        "txEntry":{
            "blockHash":"0x02aed287a1e8b16c3d545bdd78d90ee0bf519cfc60bbfe84d36b9830a4fd71b5",
            "blockHeight":"3083",
            "txIndex":"0"
        }
    }
}
```

### eth_getTransactionReceipt
根据交易Hash查询收据

#### 参数
1. `string` 交易Hash

#### 返回
- `object` 交易收据，交易不存在时返回`null`
    - blockHash `string` 所属块的Hash
    - blockNumber `string` 所属块的高度
    - transactionHash `string` 交易Hash
    - transactionIndex `string` 交易在块中的索引
    - from `string` 交易发送者
    - to `string` 交易接收者
    - tokenAddress `string` token，链克交易为 `0x0000000000000000000000000000000000000000`
    - gasUsed `string` 此交易消耗的Gas
    - cumulativeGasUsed `string` 在块中执行交易时消耗的总Gas
    - contractAddress `string` 部署合约的地址
    - logs 交易生成的log数组
    - logsBloom `string`
    - root `string` 交易后的stateroot
    - status `string` `1`(成功) 或 `0`(失败)

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getTransactionReceipt","params":["0xe5e416d32207ef3dea8d9086425f835592e7171c70cf518ecd4322610934dd74"]}' http://127.0.0.1:8000

{
    "jsonrpc":"2.0",
    "id":"0",
    "result":{
        "blockHash":"0x02aed287a1e8b16c3d545bdd78d90ee0bf519cfc60bbfe84d36b9830a4fd71b5",
        "blockNumber":"0xc0b",
        "contractAddress":null,
        "cumulativeGasUsed":"0x7a120",
        "from":"0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5",
        "gasUsed":"0x7a120",
        "logs":[],
        "logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "status":"0x1",
        "to":"0x82c3a9d971fbb999ed1b541aa1ac65793a368bb5",
        "tokenAddress":"0x0000000000000000000000000000000000000000",
        "transactionHash":"0xe5e416d32207ef3dea8d9086425f835592e7171c70cf518ecd4322610934dd74",
        "transactionIndex":"0x0"
    }
}
```

### eth_blockNumber
查询区块高度

#### 参数
- 无

#### 返回
- `string` 区块高度，16进制字符串

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_blockNumber","params":[]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": "0xc0b"
}
```

### eth_getBlockByNumber
根据块高查询区块

#### 参数
1. `string` 16进制块高，或填 `latest`，`earliest`，`pending`
2. `boolean` 设置为`true`时显示完整交易，`false`时仅显示交易Hash

#### 返回
- `object` 区块信息，区块不存在时返回`null`
    - number `string` 16进制字符串，块高，未落盘的待定块为`null`
    - hash `string` 块的Hash，未落盘的待定块为`null`
    - miner `string`
    - timestamp `string` 16进制字符串，出块时间
    - parentHash `string` 上一个块的Hash
    - transactionsRoot `string` 该块的交易根Hash
    - stateRoot `string` 该块的状态树的根Hash
    - receiptsRoot `string` 该块的交易收据的根Hash
    - gasLimit `string` 该块允许的最大Gas值
    - gasUsed `string` 该块中所有交易消耗的Gas总和
    - logsBloom `string` 块的日志的bloom过滤器，未落盘的待定块为`null`
    - transactions `数组` 交易数组，或交易Hash的数组

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getBlockByNumber","params":["0xc0b",false]}' http://127.0.0.1:8000

{
    "jsonrpc":"2.0",
    "id":"0",
    "result":{
        "number":"0xc0b",
        "hash":"0x02aed287a1e8b16c3d545bdd78d90ee0bf519cfc60bbfe84d36b9830a4fd71b5",
        "miner":"0x00000000000000000000466f756e646174696f6e",
        "timestamp":"0x5d5a8848",
        "parentHash":"0x668c0f0413d1bf6e428154cc725b31a47c358357a9aa53798ef9632ea49c944f",
        "transactionsRoot":"0xe5e416d32207ef3dea8d9086425f835592e7171c70cf518ecd4322610934dd74",
        "stateRoot":"0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
        "receiptsRoot":"0x0000000000000000000000000000000000000000000000000000000000000000",
        "gasLimit":"0x12a05f200",
        "gasUsed":"0x0",
        "logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "transactions":[
            {
                "type":"hash",
                "value":"0xe5e416d32207ef3dea8d9086425f835592e7171c70cf518ecd4322610934dd74"
            }
        ]
    }
}
```

### eth_getBlockByHash
根据块的Hash查询区块

#### 参数
1. `string` 块的Hash
2. `boolean` 设置为`true`时显示完整交易，`false`时仅显示交易Hash

#### 返回
- `object` 区块信息，同 [eth_getBlockByNumber](#eth_getblockbynumber)

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getBlockByHash","params":["0x23d8a6e7abcc66d0073487ec0b6334c3d0b31b470b70289963d8623e098aef3f",false]}' http://127.0.0.1:8000

返回结果和 eth_getBlockByNumber 相同
```

### eth_getTransactionCount
查询某账户发送过的交易数量

#### 参数
1. `string` 要查询的账户地址
2. `string` 16进制块高，或填 `latest`，`earliest`，`pending`

#### 返回
- `string` 16进制字符串，该地址所发出的交易总量

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getTransactionCount","params":["0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5","latest"]}' http://127.0.0.1:8000

{
    "jsonrpc":"2.0",
    "id":"0",
    "result":"0x1"
}
```

### eth_getBlockTransactionCountByNumber
根据块高查询指定块的交易数量

#### 参数
1. `string` 16进制块高，或填 `latest`，`earliest`，`pending`

#### 返回
- `string` 16进制字符串，该块所包含的交易数量

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getBlockTransactionCountByNumber","params":["0xc0b"]}' http://127.0.0.1:8000

{
    "jsonrpc":"2.0",
    "id":"0",
    "result":"0x1"
}
```

### eth_getBlockTransactionCountByHash
根据块的Hash查询指定块的交易数量

#### 参数
1. `string` 块的Hash

#### 返回
- `string` 16进制字符串，该块所包含的交易数量

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getBlockTransactionCountByHash","params":["0x02aed287a1e8b16c3d545bdd78d90ee0bf519cfc60bbfe84d36b9830a4fd71b5"]}' http://127.0.0.1:8000

{
    "jsonrpc":"2.0",
    "id":"0",
    "result":"0x1"
}
```

### eth_getTransactionByBlockNumberAndIndex
根据块高和在块中的索引来查询交易

#### 参数
1. `string` 16进制块高，或填 `latest`，`earliest`，`pending`
2. `string` 16进制字符串，交易在块中的索引

#### 返回
- `object` 交易信息，交易不存在时返回 `null`

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getTransactionByBlockNumberAndIndex","params":["0xc0b","0x0"]}' http://127.0.0.1:8000

{
    "jsonrpc":"2.0",
    "id":"0",
    "result":{
        "txType":"tx",
        "txHash":"0xe5e416d32207ef3dea8d9086425f835592e7171c70cf518ecd4322610934dd74",
        "signHash":"0x08d6cfb4ef69e5d02a7538865150d078a9e0dd229e4ee382558ae57d7aadbc54",
        "from":"0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5",
        "tx":{
            "type":"tx",
            "value":{
                "nonce":"0x0",
                "gasPrice":"0x174876e800",
                "gas":"0x7a120",
                "to":"0x82c3a9d971fbb999ed1b541aa1ac65793a368bb5",
                "value":"0x100",
                "input":"0x",
                "v":"0xe3e6",
                "r":"0x5587e56cd3261204002b0faf623d8fea47dc9ed6b6e8aad8aed78ac11d532a96",
                "s":"0x473b86e381cf0146d41996b12c5f3e7cf86a98666ee58d0880923776cdf469ca",
                "hash":"0xe5e416d32207ef3dea8d9086425f835592e7171c70cf518ecd4322610934dd74"
            }
        },
        "txEntry":{
            "blockHash":"0x02aed287a1e8b16c3d545bdd78d90ee0bf519cfc60bbfe84d36b9830a4fd71b5",
            "blockHeight":"3083",
            "txIndex":"0"
        }
    }
}
```

### eth_getTransactionByBlockHashAndIndex
根据块的Hash和块中的交易索引查询交易

#### 参数
1. `string` 块的Hash
2. `string` 16进制字符串，交易在块中的索引

#### 返回
- `object` 交易信息，交易不存在时返回 `null`

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getTransactionByBlockHashAndIndex","params":["0x02aed287a1e8b16c3d545bdd78d90ee0bf519cfc60bbfe84d36b9830a4fd71b5","0x0"]}' http://127.0.0.1:8000

返回结果和 eth_getTransactionByBlockNumberAndIndex 相同
```

### eth_getBlockBalanceRecordsByNumber
根据区块高度获取区块交易记录，需要先通过命令行参数 `--save_balance_record` 开启此功能

#### 参数
1. `string` 区块高度，16进制字符串，以0x开头

#### 返回
- `object` 交易记录
    - block_hash `string` 区块Hash
    - block_time `string` 区块创建时间
    - tx_records `object` 区块交易记录集
        - type `string` 交易类型
        - from `string`
        - to `string`
        - gas_limit `string`
        - gas_price `string`
        - hash `string` 交易Hash
        - nonce `string` 交易Nonce
        - payloads 合约调用参数
        - records `object数组` 一笔交易中的子交易记录集合
            - address `string` 子交易记录地址
            - amount `string` 16进制字符串，金额
            - hash `string` 子交易Hash
            - operation `string` 16进制字符串，操作(0为扣款，1为加钱)
            - token_id `string` Token标示，即Token合约地址
            - type `string` 子交易类型(transfer，contract，create_contract，sucicide，refund，fee)

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_getBlockBalanceRecordsByNumber","params":["0xf8d"]}' http://127.0.0.1:8000

{
    "jsonrpc":"2.0",
    "id":"0",
    "result":{
        "block_time":"0x5d5b6341",
        "block_hash":"0x90c2144f18dd19a8e3a33ea4cc47b702788840a4d24a55f15ebfb08038bd669e",
        "tx_records":[
            {
                "hash":"0x2a15574d8ce4056b4ce9e799a8fd432db1b4aea8f1ef58a204820f3f09291683",
                "type":"tx",
                "records":[
                    {
                        "from":"0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5",
                        "to":"0x82c3a9d971fbb999ed1b541aa1ac65793a368bb5",
                        "from_address_type":"0x0",
                        "to_address_type":"0x0",
                        "type":"transfer",
                        "token_id":"0x0000000000000000000000000000000000000000",
                        "amount":"0x20",
                        "hash":"0xe886802adabfa324b1fb332c12829512f44b71e6d91a355a6bb13897cd1bb75d"
                    },
                    {
                        "from":"0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5",
                        "to":"0x00000000000000000000466f756e646174696f6e",
                        "from_address_type":"0x0",
                        "to_address_type":"0x0",
                        "type":"fee",
                        "token_id":"0x0000000000000000000000000000000000000000",
                        "amount":"0xb1a2bc2ec50000",
                        "hash":"0x256baa40f28a68e604a4cb155af9e32d41cbbe0da94903d4bfb6b25fdd78eeb6"
                    }
                ],
                "payloads":[],
                "nonce":"0x1",
                "gas_limit":"0x7a120",
                "gas_price":"0x174876e800",
                "from":"0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5",
                "to":"0x82c3a9d971fbb999ed1b541aa1ac65793a368bb5",
                "token_id":"0x0000000000000000000000000000000000000000"
            }
        ]
    }
}
```

### personal_newAccount
创建普通账户

#### 参数
1. `string` 密码

#### 返回
- `string` 创建好的账户地址地址

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"personal_newAccount","params":["1234"]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": "0x682d048ee929d3ed7567c7ab580e913a1acc8b37"
}
```

### personal_lockAccount
解锁后重新锁住普通账户

#### 参数
1. `string` 要锁的账户地址

#### 返回
- `boolean` 成功或失败

#### 示例
```shell
curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"personal_lockAccount","params":["0x08085a83232c4a3c2f9065f5bc1d93845fe8a4b5"]}' http://127.0.0.1:8000

{
    "jsonrpc": "2.0",
    "id": "0",
    "result": true
}
```

### personal_unlockAccount
解锁普通账户

#### 参数
1. `string` 要解锁的账户地址
2. `string` 密码
3. `int` 解锁状态持续的时间，单位秒

#### 返回
- `boolean` 成功或失败

#### 示例
参考 [eth_sendTransaction](#eth_sendtransaction)
