# Remote Desktop Collaboration Platform

<p align="center">
  <b>远程桌面协同办公平台</b> — 基于 Web 的远程桌面管理与协同办公解决方案
</p>

<p align="center">
  <a href="#功能特性">功能特性</a> •
  <a href="#技术架构">技术架构</a> •
  <a href="#快速开始">快速开始</a> •
  <a href="#安装指南">安装指南</a> •
  <a href="#开发指南">开发指南</a> •
  <a href="#项目结构">项目结构</a>
</p>

---

## 项目简介

Remote Desktop Collaboration Platform 是一个面向企业级场景的远程桌面协同办公平台。它提供了基于 Web 的远程桌面访问、多用户协同操作、文件传输、宿主机统一管理等功能，支持在现代浏览器中直接访问远程桌面，无需安装额外的客户端软件。

### 主要应用场景

- 🏢 **企业内部远程办公** — 员工通过浏览器安全访问公司内网的开发环境、设计工作站
- 🖥️ **服务器运维管理** — IT 管理员集中管理多台服务器，提供 Web 化的 SSH/VNC 接入
- 👥 **协同技术支持** — 多用户同时观看/协助操作同一台远程桌面，提升协作效率
- 🎓 **云端教学演示** — 教师共享远程桌面给学生，实时演示操作流程

---

## 功能特性

| 功能模块 | 描述 | 状态 |
|---------|------|------|
| 🔐 **用户认证** | JWT Token 认证，支持 LDAP 集成，细粒度权限控制 | ✅ 已完成 |
| 🖥️ **远程桌面** | 基于 noVNC 的 Web VNC 客户端，支持全屏、缩放、剪贴板 | ✅ 已完成 |
| 👥 **协同会话** | 多用户同时接入同一桌面，支持观察者/操作者模式 | ✅ 已完成 |
| 📁 **文件传输** | Web 端文件上传下载，支持文件夹批量传输 | ✅ 已完成 |
| 🏠 **宿主机管理** | Agent 自动注册、心跳检测、资源监控、状态上报 | ✅ 已完成 |
| ⚡ **实时通信** | WebSocket 全双工通信，低延迟的指令与画面传输 | ✅ 已完成 |
| 🔒 **安全加密** | AES-256-GCM 凭证加密，TLS/SSL 传输加密 | ✅ 已完成 |
| 🐳 **容器化部署** | Docker Compose 一键部署，支持水平扩展 | ✅ 已完成 |

---

## 技术架构

### 系统架构图

```
┌─────────────────────────────────────────────────────────────┐
│                        用户层 (User Layer)                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Web 浏览器  │  │   Web 浏览器  │  │   Web 浏览器         │  │
│  │  (前端 React) │  │  (前端 React) │  │  (前端 React)        │  │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘  │
└─────────┼────────────────┼────────────────────┼─────────────┘
          │                │                    │
          └────────────────┼────────────────────┘
                           │ HTTP / WebSocket
┌──────────────────────────┼──────────────────────────────────┐
│                     网关层 (Gateway Layer)                    │
│  ┌───────────────────────┴───────────────────────────────┐  │
│  │              Nginx / 负载均衡器                          │  │
│  └───────────────────────┬───────────────────────────────┘  │
└──────────────────────────┼──────────────────────────────────┘
                           │
┌──────────────────────────┼──────────────────────────────────┐
│                  控制平面 (Control Plane)                     │
│  ┌───────────────────────┴───────────────────────────────┐  │
│  │              Master Service (Go + Gin)                  │  │
│  │  • RESTful API  │  • gRPC Agent  │  • WebSocket       │  │
│  │  • 用户认证     │  • 宿主机管理  │  • 桌面调度         │  │
│  └───────────────────────┬───────────────────────────────┘  │
└──────────────────────────┼──────────────────────────────────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
┌─────────┴────────┐ ┌───┴──────────┐ ┌──┴─────────────┐
│   数据层          │ │   Agent 层    │ │   协议网关层    │
│  PostgreSQL       │ │ Host Agent   │ │ SSH Gateway    │
│  (用户/凭证/主机)  │ │ (Go 二进制)   │ │ Protocol GW    │
└───────────────────┘ └──────────────┘ └────────────────┘
                           │
               ┌───────────┴───────────┐
               │       宿主机层          │
               │  ┌─────────────────┐  │
               │  │  Linux Desktop  │  │
               │  │  (VNC Server)   │  │
               │  └─────────────────┘  │
               └───────────────────────┘
```

### 技术栈

| 层级 | 技术选型 | 说明 |
|------|---------|------|
| **前端** | React 18 + TypeScript + Vite + Ant Design 5 | 现代化 SPA，响应式布局 |
| **后端 API** | Go 1.23 + Gin + GORM | 高性能 HTTP API 服务 |
| **实时通信** | WebSocket (gorilla/websocket) | 全双工长连接 |
| **数据库** | PostgreSQL 16 | 关系型数据持久化 |
| **消息协议** | Protocol Buffers | 前后端/服务间通信协议 |
| **安全** | JWT v5 + bcrypt + AES-256-GCM | 认证、加密、凭证保护 |
| **容器化** | Docker + Docker Compose | 一键部署与环境隔离 |
| **远程桌面** | noVNC + TurboVNC + VirtualGL | Web VNC 客户端与加速 |

---

## 快速开始

### 前置依赖

在开始之前，请确保您的系统已安装以下软件：

| 依赖 | 版本要求 | 用途 | 安装命令 |
|------|---------|------|---------|
| **Docker** | >= 24.0 | 容器化运行 | [Docker 官网](https://docs.docker.com/get-docker/) |
| **Docker Compose** | >= 2.20 | 编排多容器 | 随 Docker Desktop 附带 |
| **Go** | >= 1.22 | 编译后端/Agent | `brew install go` 或 [官网](https://go.dev/dl/) |
| **Node.js** | >= 18 | 前端开发 | `brew install node` 或 [官网](https://nodejs.org/) |

验证安装：

```bash
docker --version        # Docker version 24.x
docker compose version  # Docker Compose version 2.x
go version              # go version go1.23.x
node --version          # v20.x.x
```

### 一键部署（服务端）

```bash
# 克隆项目
git clone https://github.com/yourusername/remote-desktop-platform.git
cd remote-desktop-platform

# 方式一：使用安装脚本（推荐）
chmod +x install.sh
./install.sh all

# 方式二：直接使用 Docker Compose
docker compose up -d
```

服务启动后访问：
- 🌐 **Web 界面**: http://localhost:3000
- 🔌 **API 接口**: http://localhost:8080/api/v1
- 💓 **健康检查**: http://localhost:8080/health

### 编译宿主机 Agent

```bash
# 编译所有平台的 Agent
./install.sh agent

# 或手动编译
cd host-agent
go mod tidy
GOOS=linux GOARCH=amd64 go build -o host-agent-linux-amd64 .
```

在目标宿主机上运行 Agent：

```bash
./host-agent-linux-amd64 \
  -master=ws://your-master-ip:8080/ws/agent \
  -hostname=server-01 \
  -os=linux
```

---

## 安装指南

### 环境变量配置

创建 `.env` 文件（或修改已有配置）：

```bash
# 数据库配置
DB_HOST=postgres
DB_PORT=5432
DB_USER=rdp
DB_PASSWORD=your-secure-password
DB_NAME=remote_desktop
DB_SSLMODE=disable

# JWT 配置（生产环境请修改！）
JWT_SECRET=your-32-byte-secret-key-here!!
JWT_ACCESS_EXPIRY=15      # Access Token 有效期（分钟）
JWT_REFRESH_EXPIRY=7      # Refresh Token 有效期（天）
JWT_ISSUER=remote-desktop-platform

# 加密密钥（必须为 32 字节，用于凭证加密）
CREDENTIAL_MASTER_KEY=your-32-byte-master-key-here!

# 服务端口
HTTP_PORT=8080
SSH_GATEWAY_PORT=8082
PROTOCOL_GATEWAY_PORT=8083

# 前端 API 地址
VITE_API_BASE_URL=http://localhost:8080/api/v1
```

> ⚠️ **安全提示**：生产环境请务必修改 `JWT_SECRET` 和 `CREDENTIAL_MASTER_KEY` 为强密码，且长度至少 32 字节！

### Docker Compose 手动部署

```bash
# 1. 创建数据目录
mkdir -p data/postgres logs

# 2. 启动数据库（等待就绪）
docker compose up -d postgres

# 3. 启动主服务
docker compose up -d master-service

# 4. 启动前端
docker compose up -d frontend

# 5. 查看服务状态
docker compose ps
docker compose logs -f master-service
```

### 生产环境部署建议

1. **使用外部 PostgreSQL** — 生产环境建议使用独立的 PostgreSQL 集群，而非容器内数据库
2. **启用 TLS** — 配置 Nginx 反向代理并启用 HTTPS
3. **修改默认密码** — 所有默认密钥和数据库密码必须修改
4. **配置防火墙** — 仅开放 3000 (前端)、8080 (API)、8082 (SSH) 等必要端口
5. **日志收集** — 接入 ELK/Loki 进行日志聚合与分析

---

## 开发指南

### 本地开发环境启动

#### 后端开发（Master Service）

```bash
cd master-service

# 下载依赖
go mod tidy

# 本地运行（需要本地 PostgreSQL）
go run main.go

# 或编译运行
go build -o master-service .
./master-service
```

#### 前端开发

```bash
cd frontend

# 安装依赖（国内用户可使用 cnpm/pnpm）
npm install

# 启动开发服务器
npm run dev

# 构建生产包
npm run build
```

前端开发服务器默认运行在 http://localhost:5173

#### Agent 开发

```bash
cd host-agent
go mod tidy
go run main.go -master=ws://localhost:8080/ws/agent
```

### API 接口文档

Master Service 提供 RESTful API，主要接口：

| 接口 | 方法 | 描述 |
|------|------|------|
| `/api/v1/auth/register` | POST | 用户注册 |
| `/api/v1/auth/login` | POST | 用户登录 |
| `/api/v1/auth/refresh` | POST | 刷新 Token |
| `/api/v1/hosts` | GET | 获取宿主机列表 |
| `/api/v1/hosts` | POST | 注册宿主机 |
| `/api/v1/desktops` | GET | 获取桌面实例列表 |
| `/api/v1/desktops` | POST | 创建桌面实例 |
| `/api/v1/files/upload` | POST | 文件上传 |
| `/api/v1/files/download` | GET | 文件下载 |
| `/health` | GET | 服务健康检查 |

完整 API 文档可通过源码中的 `handlers/` 目录查看，或使用工具（如 Swagger）生成。

### 数据库模型

核心数据表：

- `users` — 用户账户信息
- `hosts` — 宿主机信息（含资源状态）
- `desktops` — 桌面实例信息
- `credentials` — 加密存储的登录凭证
- `sessions` — 用户会话与协同会话
- `file_transfers` — 文件传输记录

---

## 项目结构

```
remote-desktop-platform/
├── 📁 frontend/                  # Web 前端 (React + Vite)
│   ├── src/
│   │   ├── api/                  # API 请求封装
│   │   ├── components/           # 通用组件
│   │   ├── layouts/              # 页面布局
│   │   ├── pages/                # 页面组件
│   │   │   ├── LoginPage.tsx
│   │   │   ├── DashboardPage.tsx
│   │   │   ├── HostsPage.tsx
│   │   │   ├── DesktopsPage.tsx
│   │   │   ├── DesktopViewerPage.tsx
│   │   │   └── SystemSettingsPage.tsx
│   │   ├── stores/               # Zustand 状态管理
│   │   └── styles/               # 全局样式
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
│
├── 📁 master-service/            # 主控服务 (Go)
│   ├── cmd/                      # 命令行工具
│   ├── config/                   # 配置加载
│   ├── database/                 # 数据库初始化
│   ├── grpc/                     # gRPC/WebSocket Agent 服务
│   ├── handlers/                 # HTTP 请求处理器
│   │   ├── auth.go               # 认证相关
│   │   ├── host.go               # 宿主机管理
│   │   ├── desktop.go            # 桌面管理
│   │   ├── file.go               # 文件传输
│   │   └── collaboration.go      # 协同会话
│   ├── middleware/               # Gin 中间件
│   ├── models/                   # GORM 数据模型
│   ├── services/                 # 业务逻辑服务
│   │   ├── encryption.go         # 加密服务
│   │   └── scheduler.go          # 调度器
│   ├── Dockerfile
│   └── main.go
│
├── 📁 host-agent/                # 宿主机代理 (Go)
│   ├── agent/                    # Agent 核心逻辑
│   ├── main.go
│   └── go.mod
│
├── 📁 ssh-gateway/               # SSH 网关 (Go)
│   ├── gateway/
│   └── main.go
│
├── 📁 protocol-gateway/          # 协议网关 (Go)
│   └── go.mod
│
├── 📁 shared/                    # 共享资源
│   └── proto/                    # Protocol Buffers 定义
│       └── host_agent.proto
│
├── 📄 docker-compose.yml         # Docker Compose 部署配置
├── 📄 install.sh                 # 一键部署脚本
├── 📄 auth-helper.py             # 认证辅助脚本
├── 📄 .env                       # 环境变量模板
└── 📄 README.md                  # 本文档
```

---

## 配置说明

### Master Service 配置

Master Service 通过环境变量进行配置，支持以下配置项：

| 变量名 | 默认值 | 必填 | 说明 |
|--------|--------|------|------|
| `DB_HOST` | `localhost` | ✅ | 数据库主机 |
| `DB_PORT` | `5432` | ✅ | 数据库端口 |
| `DB_USER` | `rdp` | ✅ | 数据库用户 |
| `DB_PASSWORD` | — | ✅ | 数据库密码 |
| `DB_NAME` | `remote_desktop` | ✅ | 数据库名称 |
| `JWT_SECRET` | — | ✅ | JWT 签名密钥（32 字节） |
| `JWT_ACCESS_EXPIRY` | `15` | — | Access Token 有效期（分钟） |
| `JWT_REFRESH_EXPIRY` | `7` | — | Refresh Token 有效期（天） |
| `CREDENTIAL_MASTER_KEY` | — | ✅ | AES 加密主密钥（32 字节） |
| `HTTP_PORT` | `8080` | — | HTTP 服务端口 |
| `LDAP_HOST` | — | — | LDAP 服务器地址 |
| `LDAP_BASE_DN` | — | — | LDAP 基础 DN |

### Host Agent 配置

Agent 通过命令行参数配置：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-master` | `ws://localhost:8080/ws/agent` | Master Service WebSocket 地址 |
| `-hostname` | 系统 hostname | 宿主机标识名称 |
| `-os` | `linux` | 操作系统类型 |
| `-region` | `default` | 区域标识 |
| `-az` | `default` | 可用区标识 |

---

## 使用说明

### 1. 首次登录

1. 访问 http://localhost:3000
2. 点击注册账号，创建管理员账户
3. 登录后进入 Dashboard

### 2. 添加宿主机

1. 在目标宿主机上运行 Host Agent
2. Agent 会自动向 Master Service 注册
3. 在 **Hosts** 页面查看已注册的宿主机

### 3. 创建远程桌面

1. 进入 **Desktops** 页面
2. 点击「创建桌面」，选择目标宿主机
3. 配置桌面参数（分辨率、VNC 端口等）
4. 保存后点击「连接」即可在浏览器中打开远程桌面

### 4. 文件传输

1. 在桌面 Viewer 页面点击「文件传输」按钮
2. 支持拖拽上传和下载文件
3. 传输进度实时显示在悬浮窗中

### 5. 协同操作

1. 已有用户连接桌面后，其他用户可通过「协同加入」进入
2. 支持观察者模式（仅观看）和操作者模式（可控制）

---

## 常见问题

### Q: Agent 无法连接到 Master Service？

检查以下几点：
- Master Service 是否正常运行（访问 `/health`）
- Agent 的 `-master` 参数是否指向正确的 WebSocket 地址
- 防火墙是否放行了对应端口
- 检查 Master Service 日志中的连接信息

### Q: VNC 连接失败？

- 确认宿主机上已安装并运行 VNC Server（如 TigerVNC、TurboVNC）
- 检查 VNC 端口是否正确配置（默认 5900）
- 确认防火墙允许 VNC 端口通信

### Q: 前端页面无法访问 API？

- 检查 `VITE_API_BASE_URL` 是否配置正确
- 确认浏览器 Network 面板中的请求地址
- 检查 CORS 配置（开发环境默认允许跨域）

---

## 路线图

- [x] M1: 基础骨架搭建（认证、API、数据库）
- [x] M2: 核心功能实现（桌面管理、Agent 通信）
- [ ] M3: 协同功能完善（多用户同步、权限细化）
- [ ] M4: 运维与监控（日志、告警、资源监控面板）
- [ ] M5: 安全强化（审计日志、会话录制、零信任）

---

## 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

---

<p align="center">
  Made with ❤️ for remote work
</p>
