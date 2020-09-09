#!/bin/sh

protoc -I/usr/local/include -I. \
    --go_out=plugins=grpc,paths=source_relative:. \
    boltzrpc.proto
