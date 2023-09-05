#!/bin/bash

CURRENT_DIR=`pwd`

mkdir $CURRENT_DIR/bin
go get -d ./...
go build -o bin/application main.go
chmod +x bin/application

echo "Build successful!!"
