#!/bin/sh

# Generate Go bindings for CLN protobuf files
protoc -I . \
  --go_out=. \
  --go-grpc_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_opt=paths=source_relative \
  --experimental_allow_proto3_optional \
  --go_opt=Mprimitives.proto=go_package=cln/protos \
  --go_opt=Mnode.proto=go_package=cln/protos \
  --go-grpc_opt=Mprimitives.proto=go_package=cln/protos \
  --go-grpc_opt=Mnode.proto=go_package=cln/protos \
  primitives.proto node.proto

echo "Generated Go bindings for CLN protobuf files"
