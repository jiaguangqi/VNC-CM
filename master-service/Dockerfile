FROM alpine:latest

# 安装 ca-certificates 用于 HTTPS 请求
RUN apk --no-cache add ca-certificates

# 创建应用目录
WORKDIR /app

# 复制预编译的二进制文件
COPY master-service .

# 确保二进制可执行
RUN chmod +x ./master-service

# 暴露服务端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# 运行服务
CMD ["./master-service"]
