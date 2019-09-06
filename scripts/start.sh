#!/bin/bash
proc=lkchain

emptyBlockInterval=300
blockInterval=1000
bootnode=https://bootnode-test.lianxiangcloud.com

cd ../data
datapath=`pwd`
cd ../sbin

function Init() {
    if [ $# -ne 1 ]; then
        echo "`Usage`"
        exit 1;
    fi
    logpath=$datapath/logs
    if [ ! -L "$logpath" ]; then
        mkdir -p "$logpath"
    fi
    $proc init --home $datapath  --init_height 39476  --init_state_root 0x477d0c839b7d7a93a718e66a6b0b5642e42aa661fc333159ace82 --log.filename $logpath/lkchain.log
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
    nohup $proc node --home $datapath --bootnode.addr $bootnode  --rpc.http_endpoint "127.0.0.1:$rpcport" --rpc.ws_endpoint "127.0.0.1:$wsport" --p2p.laddr "tcp://0.0.0.0:$p2pport" --consensus.create_empty_blocks_interval $emptyBlockInterval --consensus.timeout_commit $blockInterval --log.filename $logpath/lkchain.log --log_level debug > $logpath/error.log 2>&1 &
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
