#!/bin/bash

PATH=../../bin/:$PATH

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
  home=./testdata
  logpath=${home}/logs

  if [ ! -e $logpath ]; then
    mkdir -p $logpath
  fi

  http_endpoint=":18082"
  binfile=wallet
  cmd=${binpath}/${binfile}
  peer_rpc="http://127.0.0.1:11000"

  nohup $binfile node --log_level "debug" --home ${home}  --daemon.peer_rpc $peer_rpc --detach true  >>${logpath}/attach.log  2>&1 &
}

function Stop(){
  pids=`ps aux|grep wallet|grep -v wallet.sh |grep -v grep|awk '{print $2}'`
  for i in $pids; do
    #`ps aux|grep $i`
    echo "kill $i"
    kill -9 $i 2> /dev/null
  done
}

function Restart(){
  Stop

  sleep 1

  Start

  Check
}

function Check(){
  pids=`ps aux|grep wallet|grep -v wallet.sh |grep -v grep|awk '{print $2}'`
  for i in $pids; do
    echo "pid $i"
  done
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

