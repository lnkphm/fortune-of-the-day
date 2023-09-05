#!/bin/bash

CURRENT_DIR=`pwd`

go get -d ./...
go build -o bin/application main.go
chmod +x bin/application

echo "Build successful!!"
