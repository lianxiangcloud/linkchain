#!/bin/bash

DockerDir=`cd $(dirname $0); pwd -P`
NetName=lklocaltest

if [ ! -f $DockerDir/bin/lkchain ]; then
    if [ ! -e $DockerDir/bin/ ]; then
        mkdir -p $DockerDir/bin/
    fi
    cp $DockerDir/../../bin/lkchain $DockerDir/bin/
fi

if [ `docker ps -a | grep $NetName | wc -l` -lt 1 ]; then
    echo docker run container $NetName
    docker run -v $DockerDir:/src/linkchain -w /src/linkchain -dit --name $NetName centos:7 bash
    #echo run \"docker exec lklocaltest bash /src/linkchain/sbin/start4node.sh init\" to init the linkchain lklocaltest with 4 nodes
    #echo run \"docker exec lklocaltest bash /src/linkchain/sbin/start4node.sh start\" to start the linkchain lklocaltest with 4 nodes
    #echo run \"docker exec -it lklocaltest bash\" to create a new Bash session in the container lklocaltest
fi

function Exec() {
    docker exec $NetName bash /src/linkchain/sbin/start4node.sh $1
}

function Attach() {
    docker exec -it $NetName bash
}

function Usage()
{
    echo ""
    echo "USAGE:"
    echo "command1: $0 clean"
    echo ""
    echo "command1: $0 init"
    echo ""
    echo "command1: $0 start"
    echo ""
    echo "command1: $0 stop"
    echo ""
    echo "command1: $0 reset"
    echo ""
    echo "command1: $0 attach"
    echo ""
}

case $1 in
    clean) Exec $@;;
    init) Exec $@;;
    start) Exec $@;;
    stop) Exec $@;;
    reset) Exec $@;;
    attach) Attach $@;;
    *) Usage;;
esac

