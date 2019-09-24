#!/bin/bash

# set default chain home path
defaultChainPath=~/blockdata

# default node type
defaultType="kv"
# 
# defaultType="full"

emptyBlockInterval=300
blockInterval=1000

rootpath=$(dirname $(pwd))
proc=$rootpath/bin/lkchain
dbpath=$rootpath/init/db_kv
datapath=$defaultChainPath/data
init_height=0
state_hash=""
state_path="$dbpath/kv"
ext_init_params=""
ext_start_params=""
bootnodeAddrList=https://bootnode.lianxiangcloud.com
kv_initdb_url="https://github.com/lianxiangcloud/linkchain/releases/download/db_init/db_kv.tar.gz"
full_initdb_url="https://github.com/lianxiangcloud/linkchain/releases/download/db_init/db_full.tar.gz"

function GetInitHeight() {
    init_height=$(cat $dbpath/height.txt)
    echo "init_height:$init_height"
}

function GetStateHash() {
    state_hash=$(cat $state_path/stateRoot.txt)
    echo "state_hash:$state_hash"
}

function defaultInitSet() {
    if [ $defaultType == "full" ]; then
        dbpath=$rootpath/init/db_full
        state_path="$dbpath/full"
        GetStateHash
        ext_init_params="--full_node=true --init_state_root $state_hash"
        ext_start_params="--full_node=true"
    fi
}

function downloadDB() {
    if [ $defaultType == "full" ]; then
        if [ ! -d "$rootpath/init/db_full" ]; then
            wget $full_initdb_url -O $rootpath/init/db_full.tar.gz
            cd $rootpath/init/ && tar zxf db_full.tar.gz
        fi
    else
        if [ ! -d "$rootpath/init/db_kv" ]; then
            wget $kv_initdb_url -O $rootpath/init/db_kv.tar.gz
            cd $rootpath/init/ && tar zxf db_kv.tar.gz
        fi
    fi
}

cd $rootpath
logpath=$datapath/logs

function Init() {
    if [ $# -ne 1 ]; then
        echo "`Usage`"
        exit 1;
    fi
    
    defaultInitSet

    downloadDB

    GetInitHeight

    if [ ! -d "$logpath" ]; then
        mkdir -p "$logpath"
    fi
    if [ -d "$state_path" ]; then
         echo "$state_path node"
         mkdir -p "$datapath/data"
         cp -a $state_path/state.db  $datapath/data/state.db
         $proc init --home $datapath --on_line=true --init_height $init_height $ext_init_params --log.filename $logpath/lkchain.log
    else 
        echo "$state_path not exit"
        exit 1;
    fi        
}

function Start() {
    if [ $# -lt 1 ]; then
        echo "`Usage`"
        exit 1;
    fi
    rpcport=16000
    wsport=18000
    p2pport=17000
    StartNode $@
}


function StartNode() {
    echo "start $proc ..."
    defaultInitSet
    nohup $proc node --home $datapath --bootnode.addrs $bootnodeAddrList $ext_start_params --rpc.http_endpoint "127.0.0.1:$rpcport" --rpc.ws_endpoint "127.0.0.1:$wsport" --p2p.laddr "tcp://0.0.0.0:$p2pport" --consensus.create_empty_blocks_interval $emptyBlockInterval --consensus.timeout_commit $blockInterval --log.filename $logpath/lkchain.log --log_level info > $logpath/error.log 2>&1 &
    echo "pid: $!"
}

function Stop() {
    if [ $# -ne 1 ]; then
        echo "`Usage`"
        exit 1;
    fi
    StopNode $@
}

function StopNode() {
    pid=$(ps -ef | grep $proc |grep $datapath |grep -v grep | awk '{print $2}')
    for i in $pid; do
        echo "kill $i"
        kill -9 $i 2> /dev/null
    done
}

function Restart() {
    Stop $@
    Start $@
}

function Usage() {
    echo ""
    echo "USAGE:"
    echo "command1: $0 init"
    echo ""
    echo "command2: $0 start"
    echo ""
    echo "command3: $0 stop"
    echo ""
    echo "command4: $0 restart"
    echo ""
}

cd "$(dirname $0)"
export PATH=$PWD/../bin:$PATH

case $1 in
    init) Init $@;;
    start) Start $@;;
    stop) Stop $@;;
    restart) Restart $@;;
    *) Usage;;
esac
