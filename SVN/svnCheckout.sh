#!/bin/bash

for i in `seq 1 9`; do
	echo "pwb_$i"
	svn checkout "https://stud.fh-wedel.de/repos/pwb_ws2016/pwb_0$i"
done
