#!/bin/bash
# generate-proto.sh - 生成 gRPC 代码 (需在 go mod 就绪后手动执行)

set -e

# 安装 protoc-gen-go 和 protoc-gen-go-grpc（如果尚未安装）
# go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
# go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

cd "$(dirname "$0")"

protoc \
  --go_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_out=. \
  --go-grpc_opt=paths=source_relative \
  host_agent.proto

echo "✓ gRPC 代码生成完成"
