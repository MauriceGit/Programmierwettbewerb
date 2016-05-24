#!/bin/bash

export GOPATH=$(pwd)/Go
export GOBIN=$(pwd)/bin

if [[ ! -d "bin" ]]
then
	mkdir bin
else
	echo "./bin directory already exists"
fi

echo "Set the GOPATH to '$GOPATH'"
echo "Set the GOBIN to '$GOBIN'"

echo "Get Libaries"
go get "golang.org/x/net/websocket"

echo "Build Server"
go install Programmierwettbewerb-Server

echo "Build Middleware"
go install Programmierwettbewerb-Middleware
