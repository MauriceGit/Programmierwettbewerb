#!/bin/bash

#echo $(rm -rf ./bin/*) >> ./update.log
echo $(git fetch --all) >> ./update.log
echo $(git reset --hard origin/master) >> ./update.log
echo $(./make.sh) >> ./update.log
