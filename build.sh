#!/bin/sh
 
set -e
rm -rf bin

echo "start build ...."
# build chain
make build
echo "build success!"
