// main.go - Host Agent 入口

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/remote-desktop/host-agent/agent"
)

func main() {
	var (
		masterAddr = flag.String("master", "ws://localhost:8080/ws/agent", "Master Service WebSocket 地址")
		hostname   = flag.String("hostname", "", "宿主机标识（留空使用系统 hostname）")
		osType     = flag.String("os", "linux", "操作系统类型：linux 或 windows")
		region     = flag.String("region", "default", "区域标识")
		az         = flag.String("az", "default", "可用区标识")
	)
	flag.Parse()

	if *hostname == "" {
		hn, err := os.Hostname()
		if err != nil {
			log.Fatalf("获取系统 hostname 失败: %v", err)
		}
		*hostname = hn
	}

	cfg := &agent.Config{
		MasterAddr: *masterAddr,
		Hostname:   *hostname,
		OSType:     *osType,
		Region:     *region,
		AZ:         *az,
	}

	a, err := agent.New(cfg)
	if err != nil {
		log.Fatalf("Host Agent 初始化失败: %v", err)
	}

	// 启动 Agent
	if err := a.Start(); err != nil {
		log.Fatalf("Host Agent 启动失败: %v", err)
	}

	log.Printf("Host Agent 已启动，Hostname=%s, Master=%s", cfg.Hostname, cfg.MasterAddr)

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("收到退出信号，正在优雅关闭...")
	if err := a.Stop(); err != nil {
		log.Printf("关闭出错: %v", err)
	}
}
