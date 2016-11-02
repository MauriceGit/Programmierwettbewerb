#!/bin/bash

for i in pwb_*; do 
	let x=$(ls -l $i/ | wc -l)-1; 
	if [ $x -gt 4 ]; then 
		echo "$i $x"; 
	fi; 
done | wc -l
