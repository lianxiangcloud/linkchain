#!/bin/bash

PATH=../../bin/:$PATH

home=./testdata-pre
logpath=${home}/logs
pidfile=${home}/wallet.pid

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
  echo init wallet ,now nothing todo
}

function Start(){
  echo start wallet

  if [ -f "$pidfile" ]; then 
    echo "wallet process is running, $pidfile"
    return
  fi

  if [ ! -e $logpath ]; then
    mkdir -p $logpath
  fi

  http_endpoint=":18082"
  binfile=wallet
  cmd=${binpath}/${binfile}
  #peer_rpc="http://127.0.0.1:61000"
  peer_rpc="https://c32024s1.lianxiangcloud.com:10443"
  height=1086101

  nohup $binfile node --test_net --daemon.sync_quick  --log_level "debug" --home ${home}  --daemon.peer_rpc $peer_rpc --daemon.init_height $height --detach true  >>${logpath}/attach.log  2>&1 &
}

function Stop(){
  if [ ! -f "$pidfile" ]; then 
    echo "wallet process is not running"
    return
  fi

  pids=`cat $pidfile`
  for i in $pids; do
    echo "kill -9 $i"
    kill -9 $i 2> /dev/null
  done
  rm -rf $pidfile
  echo "rm -rf $pidfile"
}

function Restart(){
  Stop

  sleep 1

  Start

  sleep 1
  
  Check
}

function Check(){
  if [ ! -f "$pidfile" ]; then 
    echo "wallet process is not running"
    return
  fi
  pids=`cat $pidfile`
  for i in $pids; do
    echo "wallet pid: $i"
    realpid=`ps aux |grep wallet |grep -w $i |wc -l`

    if [ $realpid -eq '0' ]; then
      echo "process is not running, please check $pidfile"
    fi
    return
  done
  echo "no wallet process running"
}

function Usage(){
    echo ""
    echo "USAGE:"
    echo "command1: $0 start"
    echo ""
    echo "command2: $0 stop"
    echo ""
    echo "command3: $0 restart"
    echo ""
    echo "command4: $0 check"
    echo ""
}

case $1 in
    start) Start $@;;
    stop) Stop $@;;
    restart) Restart $@;;
    check) Check $@;;
    *) Usage;;
esac

