#!/bin/bash
# ============================================================
# 远程桌面协同办公平台 - 一键部署脚本
# Remote Desktop Collaboration Platform - One-Click Deploy
# ============================================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目配置
PROJECT_NAME="remote-desktop-platform"
SERVER_DIR="master-service"
AGENT_DIR="host-agent"
SSH_GATEWAY_DIR="ssh-gateway"
PROTOCOL_GATEWAY_DIR="protocol-gateway"
FRONTEND_DIR="frontend"

# 检查命令是否存在
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# 打印带颜色信息
info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# 显示欢迎信息
show_banner() {
    cat << 'EOF'
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║    远程桌面协同办公平台 - 一键部署脚本                          ║
║    Remote Desktop Collaboration Platform Installer            ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
EOF
}

# 检测操作系统
detect_os() {
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        OS="linux"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        OS="macos"
    elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
        OS="windows"
    else
        OS="unknown"
    fi
    info "检测到操作系统: $OS"
}

# 检查前置依赖
check_prerequisites() {
    info "检查前置依赖..."

    if ! command_exists docker; then
        error "Docker 未安装，请先安装 Docker: https://docs.docker.com/get-docker/"
    fi
    success "Docker 已安装"

    if ! command_exists docker-compose; then
        warn "docker-compose 未找到，尝试使用 docker compose..."
        if ! docker compose version >/dev/null 2>&1; then
            error "docker-compose 或 docker compose 插件未安装"
        fi
        COMPOSE_CMD="docker compose"
    else
        COMPOSE_CMD="docker-compose"
    fi
    success "Docker Compose 可用: $COMPOSE_CMD"

    if ! command_exists go; then
        warn "Go 未安装，本地编译将不可用（但 Docker 构建不受影响）"
    else
        GO_VERSION=$(go version | awk '{print $3}')
        success "Go 已安装: $GO_VERSION"
    fi

    if ! command_exists node; then
        warn "Node.js 未安装，前端开发模式将不可用"
    else
        NODE_VERSION=$(node --version)
        success "Node.js 已安装: $NODE_VERSION"
    fi
}

# 部署服务端（Master Service + 前端 + 数据库）
deploy_server() {
    info "========================================"
    info "开始部署服务端组件..."
    info "========================================"

    # 检查目录
    if [ ! -f "docker-compose.yml" ]; then
        error "未找到 docker-compose.yml，请确保在远程桌面平台根目录运行此脚本"
    fi

    # 创建必要的目录
    mkdir -p data/postgres
    mkdir -p logs

    # 生成环境文件
    info "生成环境配置文件..."
    cat > .env << 'ENV_EOF'
# 数据库配置
DB_HOST=postgres
DB_PORT=5432
DB_USER=rdp
DB_PASSWORD=rdp123
DB_NAME=remote_desktop
DB_SSLMODE=disable

# JWT 配置
JWT_SECRET=change-me-in-production-32byte!
JWT_ACCESS_EXPIRY=15
JWT_REFRESH_EXPIRY=7
JWT_ISSUER=remote-desktop-platform

# 加密密钥（必须为32字节）
CREDENTIAL_MASTER_KEY=must-be-32-bytes-for-aes256-gcm!

# 服务端口
HTTP_PORT=8080
SSH_GATEWAY_PORT=8082
PROTOCOL_GATEWAY_PORT=8083

# 前端 API 地址
VITE_API_BASE_URL=http://localhost:8080/api/v1
ENV_EOF
    success "环境配置文件已生成: .env"

    # 拉取镜像并启动
    info "拉取 Docker 镜像并启动服务..."
    $COMPOSE_CMD down -v 2>/dev/null || true
    $COMPOSE_CMD pull
    $COMPOSE_CMD up -d postgres

    # 等待数据库就绪
    info "等待 PostgreSQL 就绪..."
    for i in {1..30}; do
        if docker exec rdp-postgres pg_isready -U rdp -d remote_desktop >/dev/null 2>&1; then
            success "PostgreSQL 已就绪"
            break
        fi
        echo -n "."
        sleep 1
    done

    # 启动主服务
    $COMPOSE_CMD up -d master-service

    # 等待主服务就绪
    info "等待 Master Service 就绪..."
    for i in {1..30}; do
        if curl -s http://localhost:8080/health | grep -q '"status":"ok"'; then
            success "Master Service 已就绪"
            break
        fi
        echo -n "."
        sleep 1
    done

    # 启动前端
    $COMPOSE_CMD up -d frontend

    success "服务端部署完成！"
}

# 编译宿主机代理（Host Agent）
build_agent() {
    info "========================================"
    info "编译 Host Agent..."
    info "========================================"

    if ! command_exists go; then
        error "编译 Host Agent 需要 Go 环境，请先安装 Go 1.22+"
    fi

    cd "$AGENT_DIR"

    # 下载依赖
    info "下载 Go 依赖..."
    go mod tidy

    # 编译各平台版本
    info "编译 Linux amd64 版本..."
    GOOS=linux GOARCH=amd64 go build -o ../dist/host-agent-linux-amd64 .

    info "编译 Linux arm64 版本..."
    GOOS=linux GOARCH=arm64 go build -o ../dist/host-agent-linux-arm64 .

    info "编译 Darwin amd64 版本..."
    GOOS=darwin GOARCH=amd64 go build -o ../dist/host-agent-darwin-amd64 .

    info "编译 Darwin arm64 版本..."
    GOOS=darwin GOARCH=arm64 go build -o ../dist/host-agent-darwin-arm64 .

    info "编译 Windows amd64 版本..."
    GOOS=windows GOARCH=amd64 go build -o ../dist/host-agent-windows-amd64.exe .

    cd ..
    success "Host Agent 编译完成！输出目录: ./dist/"
}

# 部署宿主机代理
deploy_agent() {
    info "========================================"
    info "部署 Host Agent..."
    info "========================================"

    # 检查是否已经编译
    AGENT_BINARY="./dist/host-agent-linux-amd64"
    if [ "$OS" = "macos" ]; then
        if [ "$(uname -m)" = "arm64" ]; then
            AGENT_BINARY="./dist/host-agent-darwin-arm64"
        else
            AGENT_BINARY="./dist/host-agent-darwin-amd64"
        fi
    fi

    if [ ! -f "$AGENT_BINARY" ]; then
        warn "未找到编译好的 Agent 二进制文件，先执行编译..."
        build_agent
    fi

    # 获取 Master 地址
    read -rp "请输入 Master Service 地址 [ws://localhost:8080/ws/agent]: " MASTER_ADDR
    MASTER_ADDR=${MASTER_ADDR:-ws://localhost:8080/ws/agent}

    read -rp "请输入本机名称 [$(hostname)]: " HOST_NAME
    HOST_NAME=${HOST_NAME:-$(hostname)}

    read -rp "请输入操作系统类型 [linux]: " OS_TYPE
    OS_TYPE=${OS_TYPE:-linux}

    # 创建 systemd 服务文件
    info "创建 systemd 服务..."
    cat > /tmp/rdp-host-agent.service << EOF
[Unit]
Description=Remote Desktop Platform Host Agent
After=network.target

[Service]
Type=simple
ExecStart=$PWD/$AGENT_BINARY -master=$MASTER_ADDR -hostname=$HOST_NAME -os=$OS_TYPE
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
EOF

    info "systemd 服务文件已生成: /tmp/rdp-host-agent.service"
    info "安装命令:"
    info "  sudo cp /tmp/rdp-host-agent.service /etc/systemd/system/"
    info "  sudo systemctl daemon-reload"
    info "  sudo systemctl enable rdp-host-agent"
    info "  sudo systemctl start rdp-host-agent"

    # 也可以直接运行
    info ""
    info "或者手动运行:"
    info "  $AGENT_BINARY -master=$MASTER_ADDR -hostname=$HOST_NAME -os=$OS_TYPE"
}

# 编译服务端（本地开发模式）
build_server_local() {
    info "========================================"
    info "本地编译 Master Service..."
    info "========================================"

    if ! command_exists go; then
        error "编译需要 Go 环境"
    fi

    cd "$SERVER_DIR"
    go mod tidy
    go build -o ../dist/master-service .
    cd ..
    success "Master Service 本地编译完成: ./dist/master-service"
}

# 显示部署结果
show_result() {
    info "========================================"
    info "部署完成！"
    info "========================================"
    echo ""
    echo -e "${GREEN}服务端访问地址:${NC}"
    echo "  - Web 界面:     http://localhost:3000"
    echo "  - API 接口:     http://localhost:8080/api/v1"
    echo "  - 健康检查:     http://localhost:8080/health"
    echo "  - Agent WebSocket: ws://localhost:8080/ws/agent"
    echo ""
    echo -e "${GREEN}默认管理账号:${NC}"
    echo "  先注册第一个用户，admin 角色可通过数据库直接设置"
    echo "  或调用: POST /api/v1/auth/register 注册用户"
    echo ""
    echo -e "${YELLOW}常用命令:${NC}"
    echo "  查看日志:     docker-compose logs -f master-service"
    echo "  停止服务:     docker-compose down"
    echo "  重启服务:     docker-compose restart"
    echo "  宿主机列表:   curl http://localhost:8080/api/v1/hosts"
    echo ""
    echo -e "${YELLOW}安装包分发说明:${NC}"
    echo "  服务端包: master-service, frontend, ssh-gateway, docker-compose.yml"
    echo "  宿主机包: host-agent (只需把编译好的二进制分发到各宿主机)"
    echo ""
}

# 主菜单
show_menu() {
    echo ""
    echo "请选择部署模式:"
    echo "  1) 部署完整服务端 (Docker Compose)"
    echo "  2) 编译宿主机代理 (Host Agent)"
    echo "  3) 部署宿主机代理到本机"
    echo "  4) 编译服务端 (本地开发)"
    echo "  5) 一键全部 (服务端 + 编译 Agent)"
    echo "  0) 退出"
    echo ""
}

# 主函数
main() {
    show_banner
    detect_os
    check_prerequisites

    if [ "$1" = "all" ]; then
        deploy_server
        build_agent
        show_result
        exit 0
    fi

    if [ "$1" = "server" ]; then
        deploy_server
        show_result
        exit 0
    fi

    if [ "$1" = "agent" ]; then
        build_agent
        deploy_agent
        exit 0
    fi

    # 交互式菜单
    while true; do
        show_menu
        read -rp "请输入选项 [0-5]: " choice
        case $choice in
            1)
                deploy_server
                show_result
                ;;
            2)
                build_agent
                ;;
            3)
                deploy_agent
                ;;
            4)
                build_server_local
                ;;
            5)
                deploy_server
                build_agent
                show_result
                ;;
            0)
                info "退出安装脚本"
                exit 0
                ;;
            *)
                warn "无效选项，请重新选择"
                ;;
        esac
    done
}

# 处理命令行参数
case "${1:-}" in
    -h|--help|help)
        echo "用法: $0 [选项]"
        echo ""
        echo "选项:"
        echo "  (无参数)    交互式菜单"
        echo "  all         一键部署服务端并编译所有 Agent"
        echo "  server      仅部署服务端"
        echo "  agent       仅编译并部署 Agent"
        echo "  -h, --help  显示帮助"
        echo ""
        exit 0
        ;;
    *)
        main "$@"
        ;;
esac
