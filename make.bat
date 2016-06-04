@echo off

set GOPATH=%~dp0Go
set GOBIN=%~dp0bin

if not exist %GOBIN% mkdir %GOBIN%

echo Set the GOPATH to "%GOPATH%"
echo Set the GOBIN to "%GOBIN%"

echo Get Libaries
go get "golang.org/x/net/websocket"
go get "golang.org/x/image/bmp"

echo Build Server
go install Programmierwettbewerb-Server

echo Build Middleware
go install Programmierwettbewerb-Middleware
