#!/bin/bash

#echo $(rm -rf ./bin/*) >> ./update.log
echo $(git fetch --all) >> ./update.log 2>&1
echo $(git reset --hard origin/master) >> ./update.log 2>&1
echo $(./make.sh) >> ./update.log 2>&1
