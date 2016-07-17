#!/bin/bash

list=$(ssh pwb-agario@cagine.fh-wedel.de "cat Programmierwettbewerb/server.log | grep -E \"The player\" | grep -v \"dummy\"")
tmpFile="tmpFile"

while read -r line; do
    successfullName=$(echo "$line" | grep -v "not allowed" | grep -v "not be associated" | awk -v FS="(player | can be)" '{print $2}')
    svn=$(echo "$line" | awk -v FS="(svn-repos |. At)" '{print $2}')
    time=$(echo "$line"| awk -v FS="(At: |CEST)" 'BEGIN{IGNORECASE = 1}{print $2}')
    failedName=$(echo "$line" | awk -v FS="(The player | is not)" '{print $2}' | grep -v "not be associated")

    RED='\033[0;31m'
    GREEN='\033[0;32m'
    NC='\033[0m' # No Color

    if [ -n "$successfullName" ]
    then
        echo -e "Player ${GREEN}$successfullName\t${NC} can be associated with ${GREEN}$svn${NC} at $time"
    else
        if [ -n "$failedName" ]
        then
            echo -e "Player ${RED}$failedName\t${NC} failed at                        $time"
        fi
    fi

done <<< "$list"


cat $tmpFile | uniq -f 10 -c
