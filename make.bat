@echo off

set GOPATH=%~dp0Go
set GOBIN=%~dp0bin\win

mkdir bin

echo Set the GOPATH to "%GOPATH%"
echo Set the GOBIN to "%GOBIN%"

echo Get Libaries
go get "golang.org/x/net/websocket"

echo Build Server
go install Programmierwettbewerb-Server

echo Build Middleware
go install Programmierwettbewerb-Middleware