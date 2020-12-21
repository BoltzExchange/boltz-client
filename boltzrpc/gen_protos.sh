#!/bin/sh

# Go bindings
protoc -I . \
  --go_out=. \
  --go-grpc_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_opt=paths=source_relative \
  boltzrpc.proto

# Go bindings for the REST proxy
protoc -I . \
  --grpc-gateway_out=. \
  --grpc-gateway_opt=logtostderr=true \
  --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=grpc_api_configuration=rest-annotations.yaml \
  boltzrpc.proto

# gRPC Markdown docs
protoc -I . \
  --doc_opt=grpc_docs.template,grpc.md \
  --doc_out='../docs/' \
  boltzrpc.proto
