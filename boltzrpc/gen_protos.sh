#!/bin/sh

protoc -I /usr/local/include -I . \
    --go_out=. \
    --go-grpc_out=. \
    --go_opt=paths=source_relative \
    --go-grpc_opt=paths=source_relative \
    boltzrpc.proto


protoc -I/usr/local/include -I . \
    --grpc-gateway_out=. \
    --grpc-gateway_opt=logtostderr=true \
    --grpc-gateway_opt=paths=source_relative \
    --grpc-gateway_opt=grpc_api_configuration=rest-annotations.yaml \
    boltzrpc.proto
