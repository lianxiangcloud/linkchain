#!/bin/bash

proc=lkchain
export PATH=$PATH:../bin

emptyBlockInterval=300
blockInterval=1000
bootnode=$PWD/bootseeds.txt

function Init() {
    if [ $# -lt 3 ]; then
        echo "`Usage`"
        exit 1;
    fi
    
    if [ "x"$4 = "x" ];then
        nodeCount=1
    else
        nodeCount=$4
    fi
    echo "init nodeCount", $nodeCount
    for((i=0;i<$nodeCount;i++));  
    do
        logpath=$3_$i/$2_logs
        if [ ! -L "$logpath" ]; then
            mkdir -p "$logpath"
        fi
        $proc init --home $3_$i/$2_data  --log.filename $logpath/lkchain.log
    done
}

function Start() {
    if [ $# -lt 3 ]; then
        echo "`Usage`"
        exit 1;
    fi

    if [ "x"$4 = "x" ];then
        nodeCount=1
    else
        nodeCount=$4
    fi
    echo "start nodeCount", $nodeCount

    for((i=0;i<$nodeCount;i++));  
    do 
        datapath=$3_$i/$2_data
        logpath=$3_$i/$2_logs
    
        case $2 in
             peer)
                rpcport=`expr 16004 + $i`
                wsport=`expr 17004 + $i`
                p2pport=`expr 18004 + $i`
                StartNode $@
                ;;
			 validator)
                rpcport=`expr 26004 + $i`
                wsport=`expr 27004 + $i`
                p2pport=`expr 28004 + $i`
                StartNode $@
                ;;	
            test)
                rpcport=`expr 11000 + $i`
                wsport=`expr 12000 + $i`
                p2pport=`expr 15000 + $i`
                StartTest $@
                ;;
            *) Usage;;
        esac
    done
}

function StartNode() {
    echo "start $proc ..."
    nohup $proc node --home $datapath --bootnode.addrs $bootnode  --rpc.http_endpoint ":$rpcport" --rpc.ws_endpoint ":$wsport" --p2p.laddr "tcp://0.0.0.0:$p2pport" --consensus.create_empty_blocks_interval $emptyBlockInterval --consensus.timeout_commit $blockInterval --log.filename $logpath/lkchain.log --log_level debug > $logpath/error.log 2>&1 &
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
    pid=$(ps -ef | grep $proc | grep -v grep | awk '{print $2}')
    for i in $pid; do
        echo "kill $i"
        kill -9 $i 2> /dev/null
    done
}

function StopTest() {
    pid=$(ps -ef | grep $proc | grep -v grep | awk '{print $2}')
    for i in $pid; do
        echo "kill $i"
        kill -9 $i 2> /dev/null
    done
}

function Usage() {
    echo ""
    echo "USAGE:"
    echo "command1: $0 init type datapath"
    echo "          type: peer,validator, test"
    echo ""
    echo "command2: $0 start type datapath [code]"
    echo "          type: peer,validator, test"
s    echo ""
    echo "command3: $0 stop type"
    echo "          type: peer,validator, test"
    echo ""
}

cd "$(dirname $0)"
export PATH=$PWD/../bin:$PATH

case $1 in
    init) Init $@;;
    start) Start $@;;
    stop) Stop $@;;
    *) Usage;;
esac
