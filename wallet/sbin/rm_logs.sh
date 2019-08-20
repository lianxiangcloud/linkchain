#!/bin/bash

DDD=`date +%Y-%m-%d -d "7 day ago"`

path=`dirname $0`
rm -vf $path/../logs/wallet.${DDD}.log
