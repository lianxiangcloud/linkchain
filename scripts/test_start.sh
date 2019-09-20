#!/bin/bash
proc=lkchain

emptyBlockInterval=300
blockInterval=1000
bootnode=https://bootnode.lianxiangcloud.com

function Init() {
    if [ $# -ne 3 ]; then
        echo "`Usage`"
        exit 1;
    fi
    
    logpath=$3/$2_logs
    if [ ! -L "$logpath" ]; then
        mkdir -p "$logpath"
    fi

    $proc init --home $3/$2_data  --log.filename $logpath/lkchain.log
}

function Start() {
    if [ $# -lt 3 ]; then
        echo "`Usage`"
        exit 1;
    fi

    datapath=$3/$2_data
    logpath=$3/$2_logs

    case $2 in
        peer)
            rpcport=41000
            wsport=43000
            p2pport=12000
            StartNode $@
			;;
	validator)
            rpcport=42000
            wsport=44000
            p2pport=12000
            StartNode $@	
            ;;
        test)
            rpcport=11000
            wsport=12000
            p2pport=15000
            StartTest $@
            ;;
        *) Usage;;
    esac
}


function StartNode() {
    echo "start $proc ..."
    nohup $proc node --home $datapath  --test_net=true --bootnode.addrs $bootnode  --rpc.http_endpoint ":$rpcport" --rpc.ws_endpoint ":$wsport" --p2p.laddr "tcp://0.0.0.0:$p2pport" --consensus.create_empty_blocks_interval $emptyBlockInterval --consensus.timeout_commit $blockInterval --log.filename $logpath/lkchain.log --log_level debug > $logpath/error.log 2>&1 &
    echo "pid: $!"
}

function StartTest() {
    echo "start $proc ..."
    nohup $proc node --home $datapath --is_test_mode true --rpc.http_endpoint ":$rpcport" --rpc.ws_endpoint ":$wsport" --p2p.laddr "tcp://0.0.0.0:$p2pport" --consensus.create_empty_blocks_interval $emptyBlockInterval --consensus.timeout_commit $blockInterval --log.filename $logpath/lkchain.log --log_level debug > $logpath/error.log 2>&1 &
    echo "pid: $!"
}

function Stop() {
    if [ $# -ne 2 ]; then
        echo "`Usage`"
        exit 1;
    fi

    case $2 in
        peer) StopNode $@;;
		validator) StopNode $@;;
        test) StopTest $@;;
        *) Usage;;
    esac
}

function StopNode() {
    pid=$(ps -ef | grep $proc |grep $2 |grep -v grep | awk '{print $2}')
    for i in $pid; do
        echo "kill $i"
        kill -9 $i 2> /dev/null
    done
}

function StopTest() {
    pid=$(ps -ef | grep $proc |grep $2 |grep -v grep | awk '{print $2}')
    for i in $pid; do
        echo "kill $i"
        kill -9 $i 2> /dev/null
    done
}

function Usage() {
    echo ""
    echo "USAGE:"
    echo "command1: $0 init type datapath"
    echo "          type: peer,validator,test"
    echo ""
    echo "command2: $0 start type datapath [code]"
    echo "          type: peer,validator,test"
    echo ""
    echo "command3: $0 stop type"
    echo "          type: peer,validator,test"
    echo ""
}

cd "$(dirname $0)"
export PATH=$PWD/../bin:$PWD/:$PATH

case $1 in
    init) Init $@;;
    start) Start $@;;
    stop) Stop $@;;
    *) Usage;;
esac
