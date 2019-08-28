# 编译享云链
## 获取并运行镜像

`$ docker run -v /blockdata -dit --name linkchain garrixwong/go1.12-boost-centos7:0.1.0`
```
Unable to find image 'garrixwong/go1.12-boost-centos7:0.1.0' locally
0.1.0: Pulling from garrixwong/go1.12-boost-centos7
d8d02d457314: Already exists 
ec488c4822b0: Pull complete 
Digest: sha256:eb468b5c0615ead9329585c28f2f4beca0fc3a61a7650285e46c2c3ec3674f07
Status: Downloaded newer image for garrixwong/go1.12-boost-centos7:0.1.0
1461aaf1b2d6e9954909105d8faca90c2193b028cec30da0340cc21ad4fe73f2
```

## 获取最新代码，并进行编译
`$ docker exec -it linkchain /bin/bash`

`$ cd linkchain`

`$ git pull`

`$ export PATH=$PATH:/usr/local/go/bin && scl enable devtoolset-8 bash `

`$ ./build.sh `

编译成功后在/linkchain/bin目录能看到编译后的文件：

`$ ll /linkchain/bin`

```
total 89668
-rwxr-xr-x 1 root root 49804544 Aug 27 02:49 lkchain
-rwxr-xr-x 1 root root 42012056 Aug 27 02:49 wallet
```
`# /linkchain/bin/lkchain version`

```
linkchain version: 0.1.0, gitCommit:7f5d2a3e
```
# 运行享云链
## 测试模式运行单节点

`$ docker exec -it linkchain /bin/bash`

```
[root@5f400a3ad5cf /]# 
```
初始化：

`$ sh /linkchain/scripts/test_start.sh init test /blockdata/`

```
committee contract code nil!!!
validators white list contract code nil!!!
genesisBlock stateHash 0x0d8827403cb36d8d176cbf6257915f1b5274ba11ff2891b06a0263946ebf0b57
genesisBlock trieRoot 0x0000000000000000000000000000000000000000000000000000000000000000
genesisBlock ChainID:chainID block.Hash:0x26cb0291c88674df8614a93eb0e1b5e23b82e3117f18dade10acb0cf7c597b2d
```

启动节点：

`$ sh /linkchain/scripts/test_start.sh start test /blockdata/`

```
start lkchain ...
pid: 390
```

测试RPC:

`# curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_blockNumber","params":[]}' http://127.0.0.1:11000`

```
{"jsonrpc":"2.0","id":"0","result":"0x0"}
```
查看Log:
`# tail /blockdata/test_logs/lkchain.log`

```
DEBUG 2019-08-27 03:04:44.797 status report                            module=mempool specGoodTxs=0 goodTxs=0 futureTxs=0
DEBUG 2019-08-27 03:04:44.819 Broadcasting proposal heartbeat message  module=consensus height=3 round=0 sequence=1
DEBUG 2019-08-27 03:04:46.820 Broadcasting proposal heartbeat message  module=consensus height=3 round=0 sequence=2
DEBUG 2019-08-27 03:04:48.821 Broadcasting proposal heartbeat message  module=consensus height=3 round=0 sequence=3
DEBUG 2019-08-27 03:04:49.797 status report                            module=mempool specGoodTxs=0 goodTxs=0 futureTxs=0
DEBUG 2019-08-27 03:04:49.865 dialOutLoop                              module=conManager maxDialOutNums=3 needDynDials=3
DEBUG 2019-08-27 03:04:49.865 ReadRandomNodes                          module=httpTable tab.seeds=[]
DEBUG 2019-08-27 03:04:49.865 after dialRandNodesFromCache             module=conManager needDynDials=3
DEBUG 2019-08-27 03:04:49.865 dialNodesFromNetLoop                     module=conManager needDynDials=3
DEBUG 2019-08-27 03:04:50.822 Broadcasting proposal heartbeat message  module=consensus height=3 round=0 sequence=4
```

关闭节点:

`# sh /linkchain/scripts/test_start.sh stop test`

```
kill 390
```
## 运行多节点
`$ docker exec -it linkchain /bin/bash`

```
[root@5f400a3ad5cf /]# 
```
初始化4个测试节点：

`$ sh /linkchain/scripts/start_multi.sh init test /blockdata/ 4`

```
init nodeCount, 4
committee contract code nil!!!
validators white list contract code nil!!!
genesisBlock stateHash 0x0d8827403cb36d8d176cbf6257915f1b5274ba11ff2891b06a0263946ebf0b57
genesisBlock trieRoot 0x0000000000000000000000000000000000000000000000000000000000000000
genesisBlock ChainID:chainID block.Hash:0x26cb0291c88674df8614a93eb0e1b5e23b82e3117f18dade10acb0cf7c597b2d
committee contract code nil!!!
validators white list contract code nil!!!
genesisBlock stateHash 0x0d8827403cb36d8d176cbf6257915f1b5274ba11ff2891b06a0263946ebf0b57
genesisBlock trieRoot 0x0000000000000000000000000000000000000000000000000000000000000000
genesisBlock ChainID:chainID block.Hash:0x26cb0291c88674df8614a93eb0e1b5e23b82e3117f18dade10acb0cf7c597b2d
committee contract code nil!!!
validators white list contract code nil!!!
genesisBlock stateHash 0x0d8827403cb36d8d176cbf6257915f1b5274ba11ff2891b06a0263946ebf0b57
genesisBlock trieRoot 0x0000000000000000000000000000000000000000000000000000000000000000
genesisBlock ChainID:chainID block.Hash:0x26cb0291c88674df8614a93eb0e1b5e23b82e3117f18dade10acb0cf7c597b2d
committee contract code nil!!!
validators white list contract code nil!!!
genesisBlock stateHash 0x0d8827403cb36d8d176cbf6257915f1b5274ba11ff2891b06a0263946ebf0b57
genesisBlock trieRoot 0x0000000000000000000000000000000000000000000000000000000000000000
genesisBlock ChainID:chainID block.Hash:0x26cb0291c88674df8614a93eb0e1b5e23b82e3117f18dade10acb0cf7c597b2d
```

启动4个测试节点：

`$ sh /linkchain/scripts/start_multi.sh start test /blockdata/ 4`

```
start nodeCount, 4
start lkchain ...
pid: 355
start lkchain ...
pid: 359
start lkchain ...
pid: 363
start lkchain ...
pid: 372
```

测试RPC:

`# curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_blockNumber","params":[]}' http://127.0.0.1:11000`

```
{"jsonrpc":"2.0","id":"0","result":"0x0"}
```

查看第一个节点的Log:
`# tail /blockdata/_0/test_logs/lkchain.log`

```
DEBUG 2019-08-27 03:04:44.797 status report                            module=mempool specGoodTxs=0 goodTxs=0 futureTxs=0
DEBUG 2019-08-27 03:04:44.819 Broadcasting proposal heartbeat message  module=consensus height=3 round=0 sequence=1
DEBUG 2019-08-27 03:04:46.820 Broadcasting proposal heartbeat message  module=consensus height=3 round=0 sequence=2
DEBUG 2019-08-27 03:04:48.821 Broadcasting proposal heartbeat message  module=consensus height=3 round=0 sequence=3
DEBUG 2019-08-27 03:04:49.797 status report                            module=mempool specGoodTxs=0 goodTxs=0 futureTxs=0
DEBUG 2019-08-27 03:04:49.865 dialOutLoop                              module=conManager maxDialOutNums=3 needDynDials=3
DEBUG 2019-08-27 03:04:49.865 ReadRandomNodes                          module=httpTable tab.seeds=[]
DEBUG 2019-08-27 03:04:49.865 after dialRandNodesFromCache             module=conManager needDynDials=3
DEBUG 2019-08-27 03:04:49.865 dialNodesFromNetLoop                     module=conManager needDynDials=3
DEBUG 2019-08-27 03:04:50.822 Broadcasting proposal heartbeat message  module=consensus height=3 round=0 sequence=4
```

关闭节点:

`# sh /linkchain/scripts/start_multi.sh stop test`

```
kill 355
kill 359
kill 363
kill 372
```