FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/master-service .

FROM alpine:3.20

# 安装 ca-certificates 用于 HTTPS 请求，python3 用于系统账号密码校验 helper
RUN apk --no-cache add ca-certificates python3

# 创建应用目录
WORKDIR /app

# 复制编译产物和系统账号认证辅助脚本
COPY --from=builder /out/master-service .
COPY auth-helper.py .

# 确保二进制可执行
RUN chmod +x ./master-service ./auth-helper.py

# 暴露服务端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 运行服务
CMD ["./master-service"]
