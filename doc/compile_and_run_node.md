# 编译运行享云链

<!-- TOC -->

- [编译运行享云链](#编译运行享云链)
    - [创建本地数据目录](#创建本地数据目录)
    - [获取编译镜像](#获取编译镜像)
    - [编译可执行程序](#编译可执行程序)
    - [运行享云链节点](#运行享云链节点)
    - [测试模式运行单节点本地测试网络](#测试模式运行单节点本地测试网络)
    - [启动一个本地钱包](#启动一个本地钱包)
    - [运行多节点本地测试网络](#运行多节点本地测试网络)

<!-- /TOC -->

本文档将介绍如何通过docker编译享云链节点源码，并启动一个享云链的peer节点，或者运行测试节点。

## 创建本地数据目录

`mkdir -p ~/blockdata`

## 获取编译镜像

`$ docker run -dit -v ~/blockdata:/blockdata --name linkchain --net=host garrixwong/go1.12-boost-centos7:0.1.0`

```bash
Unable to find image 'garrixwong/go1.12-boost-centos7:0.1.0' locally
0.1.0: Pulling from garrixwong/go1.12-boost-centos7
d8d02d457314: Already exists  
ec488c4822b0: Pull complete  
Digest: sha256:eb468b5c0615ead9329585c28f2f4beca0fc3a61a7650285e46c2c3ec3674f07
Status: Downloaded newer image for garrixwong/go1.12-boost-centos7:0.1.0
1461aaf1b2d6e9954909105d8faca90c2193b028cec30da0340cc21ad4fe73f2
```

## 编译可执行程序

进入docker容器内  
`$ docker exec -it linkchain /bin/bash`

创建目录软连接  
`mkdir -p ~/blockdata && ln -s ~/blockdata /blockdata`

进入源代码目录  
`$ cd linkchain`

拉取最新代码  
`$ git pull`

设置环境变量  
`$ export PATH=$PATH:/usr/local/go/bin && scl enable devtoolset-8 bash`

执行编译打包  
`$ ./build.sh`

编译成功后在/linkchain/pack/lkchain/bin/目录能看到编译后的文件：  
`$ ll /linkchain/pack/lkchain/bin/`

```bash
total 89668
-rwxr-xr-x 1 root root 49804544 Aug 27 02:49 lkchain
```

查看查询后版本号  
`/linkchain/pack/lkchain/bin/lkchain version`

```bash
linkchain version: 0.1.0, gitCommit:7f5d2a3e
```

## 运行享云链节点

进入docker容器内  
`$ docker exec -it linkchain /bin/bash`

进入启动脚本目录  
`cd /linkchain/pack/lkchain/sbin`

第一次运行节点，需要执行初始化  
`./start.sh init`

启动节点  
`./start.sh start`

暂停节点  
`./start.sh stop`

## 测试模式运行单节点本地测试网络

进入docker容器内  
`$ docker exec -it linkchain /bin/bash`

初始化  
`$ sh /linkchain/scripts/test_start.sh init test ~/blockdata/`

```bash
committee contract code nil!!!
validators white list contract code nil!!!
genesisBlock stateHash 0x0d8827403cb36d8d176cbf6257915f1b5274ba11ff2891b06a0263946ebf0b57
genesisBlock trieRoot 0x0000000000000000000000000000000000000000000000000000000000000000
genesisBlock ChainID:chainID block.Hash:0x26cb0291c88674df8614a93eb0e1b5e23b82e3117f18dade10acb0cf7c597b2d
```

启动节点：  
`$ sh /linkchain/scripts/test_start.sh start test ~/blockdata/`

```bash
start lkchain ...
pid: 390
```

测试RPC:

`# curl -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"0","method":"eth_blockNumber","params":[]}' http://127.0.0.1:11000`

```bash
{"jsonrpc":"2.0","id":"0","result":"0x0"}
```

查看Log:  
`# tail ~/blockdata/test_logs/lkchain.log`

```bash
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

```bash
kill 390
```

## 启动一个本地钱包

`$ docker exec -it linkchain /bin/bash`

进入钱包启动脚本目录  
`$ cd /linkchain/wallet/sbin`

钱包默认连接本地的peer节点，如果上一步已经启动了一个本地的测试peer，那么现在可以直接启动钱包连接这个peer  

启动钱包进程  
`$ ./wallet.sh start`

复制测试账户的密钥文件到钱包账户目录  

`$ cp ../tests/UTC--2019-07-08T10-03-04.871669363Z--a73810e519e1075010678d706533486d8ecc8000 ./testdata/keystore/`

解锁钱包，测试账户的密码是"1234"  

```bash
$ curl -s -X POST http://127.0.0.1:18082 -d '{"jsonrpc":"2.0","method":"personal_unlockAccount","params":["0xa73810e519e1075010678d706533486d8ecc8000","1234",3600],"id":67}' -H 'Content-Type:application/json'  
{"jsonrpc":"2.0","id":67,"result":true}
```

接下来可以进行其他钱包操作，如转账、查看交易等，具体参考 [wallet钱包](../wallet/README.md)  

停止钱包进程  
`$ ./wallet.sh stop`

## 运行多节点本地测试网络

`$ docker exec -it linkchain /bin/bash`

初始化4个测试节点：

`$ sh /linkchain/scripts/start_multi.sh init test /blockdata/ 4`

```bash
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

```bash
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

```bash
{"jsonrpc":"2.0","id":"0","result":"0x0"}
```

查看第一个节点的Log:  
`# tail /blockdata/_0/test_logs/lkchain.log`

```bash
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

```bash
kill 355
kill 359
kill 363
kill 372
```
