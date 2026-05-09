// main.go - SSH Gateway 入口

package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/remote-desktop/ssh-gateway/gateway"
)

func main() {
	var (
		listenAddr = flag.String("listen", ":8082", "HTTP 监听地址")
		jwtSecret  = flag.String("jwt-secret", "", "JWT 签名密钥（与 Master Service 一致）")
		wsPath     = flag.String("ws-path", "/ws/ssh", "WebSocket 路径")
	)
	flag.Parse()

	if *jwtSecret == "" {
		log.Fatal("必须设置 --jwt-secret 参数")
	}

	g := gateway.New(&gateway.Config{
		JWTSecret: *jwtSecret,
		WSPath:    *wsPath,
	})

	http.HandleFunc(*wsPath, g.HandleWebSocket)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("SSH Gateway 启动于 %s", *listenAddr)
	if err := http.ListenAndServe(*listenAddr, nil); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
