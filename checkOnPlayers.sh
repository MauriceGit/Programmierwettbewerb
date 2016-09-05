#!/bin/bash

list=$(ssh pwb-agario@cagine.fh-wedel.de "cat Programmierwettbewerb/server.log")
tmpFile="tmpFile"

function parseValue(){
    value="$2"
    line=$(echo "$1" | sed -s "s/^.*$2/$2/")
    echo $(echo $line | awk -v FS="($value=\"|\".)" '{print $2}')
}

function parseTime(){
    echo $(echo "$1" | awk -v FS="(^| Line)" '{print $1}')
}

while read -r line; do

    if [ -n "$(echo "$line" | grep "NEW_BOT")" ]
    then
        name=$(parseValue "$line" "NAME")
        svn=$(parseValue "$line" "SVN")
        ip=$(parseValue "$line" "IP")
        time=$(parseTime "$line")
        echo -e "New Bot: $name, svn: $svn, ip: $ip, time: $time."
        continue
    fi

done <<< "$list"

