#!/bin/bash

list=$(ssh pwb-agario@cagine.fh-wedel.de "cat Programmierwettbewerb/server.log | grep -E \"The player\" | grep -v \"dummy\"" | uniq -f 15 -c)

while read -r line; do
    successfullName=$(echo "$line" | awk -v FS="(player | can be)" '{print $2}' | grep -v "not allowed" | grep -v "not be associated")
    svn=$(echo "$line" | awk -v FS="(svn-repos |. At)" '{print $2}')
    time=$(echo "$line"| awk -v FS="(At: |CEST)" 'BEGIN{IGNORECASE = 1}{print $2}')
    failedName=$(echo "$line" | awk -v FS="(The player | is not)" '{print $2}' | grep -v "not be associated")

    RED='\033[0;31m'
    GREEN='\033[0;32m'
    NC='\033[0m' # No Color

    if [ -n "$successfullName" ]
    then
        echo -e "Player ${GREEN}$successfullName\t${NC} can be associated with ${GREEN}$svn${NC} at $time"
        #echo "$successfullName $svn $time"
    else
        if [ -n "$failedName" ]
        then
            echo -e "Player ${RED}$failedName\t${NC} failed at                        $time"
            #echo -e "Player $failedName\t can be associated with pwb_05 at $time"
        fi
    fi

done <<< "$list"
