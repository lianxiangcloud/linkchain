#!/bin/bash
proc=lkchain
#monitor_path=$0
#datapath= $(dirname ${monitor_path})
datapath=$(cd `dirname $0`;cd ..;pwd)
pid=$(ps -ef | grep $proc |grep $datapath |grep -v grep | awk '{print $2}')
ulimit -c unlimited

now=`date  +%Y-%m-%d[%H:%M:%S]`

if [ -d "$datapath/data/data" ]; then
  if [[ $pid -eq 0 ]]
  then
      cd $datapath/sbin
      ./start.sh start
      now=`date  +%Y-%m-%d[%H:%M:%S]`
      echo "at $now restart start lkchain" >> check_lkchain.log
  fi
fi



