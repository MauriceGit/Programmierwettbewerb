#!/bin/bash


file="$PWD/languageStats"
file2="$PWD/languageStats2"

IFS_="$IFS"
IFS="
"
> "$file"
> "$file2"

for i in pwb*
do
    cd "$i"
    echo "$i:" >> $file
    tmp=$(cloc . | awk -v prefix="$i    " '{print prefix $0}')
    for t in "$tmp"
    do
        echo -e "$t" >> $file

    done
    cd ..
done
IFS="$IFS_"

cat "$file" | grep Python               >> $file2
cat "$file" | grep C | grep -v Header   >> $file2
cat "$file" | grep C++ | grep -v Header >> $file2
cat "$file" | grep "Bourne Shell"       >> $file2
cat "$file" | grep "Bourne Again Shell" >> $file2
cat "$file" | grep XML                  >> $file2
cat "$file" | grep make                 >> $file2
cat "$file" | grep Rust                 >> $file2
cat "$file" | grep Java                 >> $file2
cat "$file" | grep Ant                  >> $file2
cat "$file" | grep Rust                 >> $file2

sort "$file2" | uniq -c


