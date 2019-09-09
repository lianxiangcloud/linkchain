#!/bin/bash
proc=lkchain

emptyBlockInterval=300
blockInterval=1000
bootnode=https://bootnode-test.lianxiangcloud.com

rootpath=$(dirname $(pwd))
dbpath=$rootpath/init/db
datapath=$rootpath/data
init_height=0

function GetInitHeight() {
    init_height=$(cat $dbpath/height.txt)
    echo "init_height:$init_height"
}

logpath=$datapath/logs
function Init() {
    if [ $# -ne 1 ]; then
        echo "`Usage`"
        exit 1;
    fi
    GetInitHeight
    if [ ! -L "$logpath" ]; then
        mkdir -p "$logpath"
    fi
    if [ -d "$dbpath/kv" ]; then
         echo "kv node"
         mkdir -p "$datapath/data"
         cp -a $dbpath/kv/state.db  $datapath/data/state.db
         $proc init --home $datapath  --init_height $init_height  --log.filename $logpath/lkchain.log
    else 
        echo "kv db not exit"
        exit 1;
    fi        
}

function Start() {
    if [ $# -lt 1 ]; then
        echo "`Usage`"
        exit 1;
    fi
    rpcport=45000
    wsport=46000
    p2pport=47000
    StartNode $@
}


function StartNode() {
    echo "start $proc ..."
    nohup $proc node --home $datapath --bootnode.addrs $bootnode  --rpc.http_endpoint "127.0.0.1:$rpcport" --rpc.ws_endpoint "127.0.0.1:$wsport" --p2p.laddr "tcp://0.0.0.0:$p2pport" --consensus.create_empty_blocks_interval $emptyBlockInterval --consensus.timeout_commit $blockInterval --log.filename $logpath/lkchain.log --log_level info > $logpath/error.log 2>&1 &
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
