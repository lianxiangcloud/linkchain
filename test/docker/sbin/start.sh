#!/bin/bash

PATH=$PWD/../bin:$PWD/bin:$PATH
PROCNAME=lkchain
CONFPATH=$PWD/../conf
bootnode=$CONFPATH/bootnode.json

red="\033[0;31;40m"
color_end="\033[0m"
#echo -e "$color Heelooo!!!$color_end";

function CheckResult() {
    ret=$?;
    if [ "$ret"x != "0"x ]; then
        echo -e "${red}Your [$1] Failed !!!${color_end}";
    else
        echo -e "Your [$1] Success !!!";
    fi
}

action=$1

function Init(){
    if [ $# -lt 4 ]; then
        echo "`Usage`"
        exit 1;
    fi

    logfile=$4/$3_logs/node$2/$PROCNAME.log
    datapath=$4/$3_data/node$2

    $PROCNAME init --home $datapath  --log.filename $logfile --genesis_file $CONFPATH/genesis.json
    CheckResult "$PROCNAME init"
}

function Start(){
    if [ $# -lt 4 ]; then
        echo "`Usage`"
        exit 1;
    fi

    nodeid=$2
    nodetype=$3
    logpath=$4/$3_logs/node$2
    datapath=$4/$3_data/node$2

    if [ ! -L "$logpath" ]; then
        mkdir -p "$logpath"
    fi

    rpcport=$((16000+nodeid))
    wsport=$((14000+nodeid))
    p2pport=$((13000+nodeid))
    if [ "$nodetype" == "validator" ] || [ "$nodetype" == "candidate" ] ; then
        rpcport=$((16000+nodeid))
        wsport=$((14000+nodeid))
        p2pport=$((13000+nodeid))
    elif [ "$nodetype" == "peer" ] ; then
        rpcport=$((46000+nodeid))
        wsport=$((44000+nodeid))
        p2pport=$((43000+nodeid))
    fi

    emptyBlockInterval=50
    blockInterval=5000
    
    nohup $PROCNAME node --home $datapath --rpc.http_endpoint ":$rpcport" --rpc.ws_endpoint ":$wsport"  --p2p.laddr "tcp://0.0.0.0:$p2pport" --consensus.create_empty_blocks_interval $emptyBlockInterval --consensus.timeout_commit $blockInterval --bootnode.addrs $bootnode --log.filename $logpath/$PROCNAME.log --log_level debug > $logpath/error.log 2>&1 &
	CheckResult "$PROCNAME start"
}

function Stop(){
    if [ $# -lt 3 ] ; then
        echo "`Usage`"
        exit 1;
    fi

    nodeid=$2
    nodetype=$3
    port=13000
    if [ "$nodetype" == "validator" ] || [ "$nodetype" == "candidate" ] ; then
        port=$((13000+nodeid))
    elif [ "$nodetype" == "peer" ] ; then
        port=$((43000+nodeid))
    fi

    pid=$(ps -ef | grep $nodetype | grep node$2 | grep $port | grep -v grep | awk '{print $2}')
    if [ "$nodetype" == "test" ] ; then
        pid=$(ps -ef | grep $nodetype | grep node$2 | grep -v grep | awk '{print $2}')
    fi

    for i in $pid; do
        echo "kill $i"
        kill -9 $i 2> /dev/null
    done
    echo -e "Stop $nodetype service:\t\t\t\t\t[  OK  ]"
}

function Usage()
{
    echo ""
    echo "USAGE:"
    echo "command1: $0 init nodeid type datapath"
    echo "          nodeid: 0,1,2...."
    echo "          type: validator, peer, candidate, test"
    echo ""
    echo "command2: $0 start nodeid type datapath"
    echo "          nodeid: 0,1,2...."
    echo "          type: validator, peer, candidate, test"
    echo ""
    echo "command3: $0 stop nodeid type"
    echo "          nodeid: 0,1,2...."
    echo "          type: validator, peer, candidate, test"
    echo ""
}

case $1 in
    init) Init $@;;
    start) Start $@;;
    stop) Stop $@;;
    *) Usage;;
esac
