// main.go - Master Service 入口

package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/remote-desktop/master-service/config"
	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/grpc"
	"github.com/remote-desktop/master-service/handlers"
	"github.com/remote-desktop/master-service/middleware"
	"github.com/remote-desktop/master-service/services"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化数据库
	if err := database.Init(&cfg.Database); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 初始化加密服务
	encryptor, err := services.NewEncryptionService(cfg.Encryption.MasterKey)
	if err != nil {
		log.Fatalf("加密服务初始化失败: %v", err)
	}

	// 初始化调度器
	scheduler := services.NewScheduler()
	_ = scheduler // 后续 will be used

	// 初始化 WebSocket Agent Server
	agentServer := grpc.NewHostAgentServer()

	// 初始化 JWT 中间件
	jwtMiddleware := middleware.NewJWTMiddleware(cfg.JWT.Secret, cfg.JWT.Issuer)

	// 初始化处理器
	authHandler := handlers.NewAuthHandler(jwtMiddleware, &cfg.LDAP)
	hostHandler := handlers.NewHostHandler(encryptor)
	desktopHandler := handlers.NewDesktopHandler(encryptor)
	statsHandler := handlers.NewStatsHandler()

	// 初始化 Gin 路由
	router := gin.Default()

	// 公开路由
	public := router.Group("/api/v1")
	{
		public.POST("/auth/register", authHandler.Register)
		public.POST("/auth/login", authHandler.Login)
		public.POST("/auth/refresh", authHandler.Refresh)
	}

	// 协作处理器
	collabHandler := handlers.NewCollaborationHandler(encryptor)

	// Agent WebSocket 连接（无需 HTTP 认证，使用 agent_token）
	router.GET("/ws/agent", agentServer.HandleWebSocket)

	// 需要认证的路由
	authorized := router.Group("/api/v1")
	authorized.Use(jwtMiddleware.AuthRequired())
	{
		// 用户相关
		authorized.GET("/auth/me", authHandler.Me)

		// 桌面会话管理（普通用户）
		authorized.GET("/desktops", desktopHandler.ListDesktops)
		authorized.POST("/desktops", desktopHandler.CreateDesktop)
		authorized.GET("/desktops/:id", desktopHandler.GetDesktopDetail)
		authorized.DELETE("/desktops/:id", desktopHandler.CloseDesktop)
		authorized.DELETE("/desktops/:id/record", desktopHandler.DeleteDesktop)
		authorized.POST("/desktops/batch/terminate", desktopHandler.BatchTerminateDesktops)
		authorized.POST("/desktops/batch/delete", desktopHandler.BatchDeleteDesktops)

	// 文件传输
	fileHandler := handlers.NewFileHandler(encryptor)
	authorized.GET("/desktops/:id/files", fileHandler.ListFiles)
	authorized.POST("/desktops/:id/upload", fileHandler.UploadFile)
	authorized.GET("/desktops/:id/download", fileHandler.DownloadFile)
	authorized.DELETE("/desktops/:id/files", fileHandler.DeleteFile)
	authorized.POST("/desktops/:id/mkdir", fileHandler.Mkdir)

		// 协同协助（需要认证）
			authorized.GET("/collaborations/invited", collabHandler.ListInvited)
			authorized.GET("/collaborations", collabHandler.ListMyInvites)
			authorized.POST("/collaborations", collabHandler.Invite)
			authorized.DELETE("/collaborations/:id", collabHandler.Stop)

		// 统计数据（需要认证）
		authorized.GET("/stats/overview", statsHandler.GetOverview)
		authorized.GET("/stats/trend", statsHandler.GetTrend)

		// 宿主机管理（仅管理员）
		admin := authorized.Group("")
		admin.Use(middleware.AdminOnly())
		{
			admin.POST("/hosts", hostHandler.CreateHost)
			admin.GET("/hosts", hostHandler.ListHosts)
			admin.GET("/hosts/:id", hostHandler.GetHost)
			admin.PATCH("/hosts/:id", hostHandler.UpdateHost)
			admin.DELETE("/hosts/:id", hostHandler.DeleteHost)
		}
	}

	// 共享访问代理（无需认证，通过 token）
	router.GET("/share/:token/*path", collabHandler.ShareProxy)
	router.GET("/share/:token", collabHandler.ShareProxy)
	router.GET("/api/v1/share/:token", collabHandler.ValidateToken)

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "master-service"})
	})

	// 优雅关闭
	httpPort := cfg.Server.HTTPPort
	if httpPort == "" {
		httpPort = ":8080"
	}

	log.Printf("Master Service 启动于 %s", httpPort)
	if err := router.Run(httpPort); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
