#!/bin/sh
# cold build (empty cache) of a hello world program
set -e
export GOCACHE=/tmp/gocache GOPATH=/tmp/gopath GOFLAGS=-trimpath
rm -rf /tmp/h /tmp/gocache && mkdir /tmp/h && cd /tmp/h
printf 'package main\n\nimport "fmt"\n\nfunc main() { fmt.Println("hello") }\n' > main.go
go mod init h
go build -o h .
./h
rm -rf /tmp/h /tmp/gocache /tmp/gopath
