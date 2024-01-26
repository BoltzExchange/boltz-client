#!/bin/sh

# Go bindings
protoc -I . \
  --go_out=. \
  --go-grpc_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_opt=paths=source_relative \
  --experimental_allow_proto3_optional \
  boltzrpc.proto autoswaprpc/autoswap.proto

# Go bindings for the REST proxy
protoc -I . -I autoswaprpc \
  --grpc-gateway_out=. \
  --grpc-gateway_opt=logtostderr=true \
  --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=grpc_api_configuration=rest-annotations.yaml \
  --experimental_allow_proto3_optional \
  boltzrpc.proto autoswaprpc/autoswap.proto

# gRPC Markdown docs
protoc -I . -I autoswaprpc \
  --doc_opt=grpc_docs.template,grpc.md \
  --doc_out='../docs/' \
  --experimental_allow_proto3_optional \
  boltzrpc.proto autoswaprpc/autoswap.proto
