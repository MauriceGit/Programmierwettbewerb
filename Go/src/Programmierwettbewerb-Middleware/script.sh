#!/bin/bash

while read line
do
    echo "My customized input: $line"
done < "${1:-/dev/stdin}"


