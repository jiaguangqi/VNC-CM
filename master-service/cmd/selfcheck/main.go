package main

import (
	"fmt"
	"os"
	"os/user"

	"github.com/remote-desktop/master-service/config"
	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/models"
	"github.com/remote-desktop/master-service/services"
)

func main() {
	cfg := config.Load()
	failed := false

	fmt.Println("VNC-CM deployment self-check")
	fmt.Println("============================")

	checkEnv("CREDENTIAL_MASTER_KEY", cfg.Encryption.MasterKey != "")
	checkEnv("JWT_SECRET", cfg.JWT.Secret != "" && cfg.JWT.Secret != "change-me-in-production-32byte!")
	checkEnv("DB_HOST", cfg.Database.Host != "")

	if cfg.Encryption.MasterKey == "" {
		failed = true
		fmt.Println("SKIP database and host checks: CREDENTIAL_MASTER_KEY is required")
		os.Exit(1)
	}

	encryptor, err := services.NewEncryptionService(cfg.Encryption.MasterKey)
	if err != nil {
		failed = true
		fmt.Printf("FAIL encryption: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK encryption")

	if err := database.Init(&cfg.Database); err != nil {
		failed = true
		fmt.Printf("FAIL database: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK database")

	var hosts []models.Host
	if err := database.DB.Find(&hosts).Error; err != nil {
		failed = true
		fmt.Printf("FAIL hosts query: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK hosts query: %d registered\n", len(hosts))

	username := os.Getenv("SELF_CHECK_USERNAME")
	if username == "" {
		if currentUser, err := user.Current(); err == nil {
			username = currentUser.Username
		}
	}
	if username == "" {
		username = "root"
	}

	readiness := services.NewNodeReadinessService(encryptor)
	for _, host := range hosts {
		result := readiness.CheckHost(host, username)
		status := "OK"
		if !result.Ready {
			status = "WARN"
		}
		fmt.Printf("%s host=%s ip=%s user=%s ready=%v current_user_exists=%v missing=%v\n",
			status, host.Hostname, host.IPAddress, username, result.Ready, result.CurrentUserExists, result.Missing)
	}

	if failed {
		os.Exit(1)
	}
}

func checkEnv(name string, ok bool) {
	if ok {
		fmt.Printf("OK env %s\n", name)
		return
	}
	fmt.Printf("WARN env %s is empty or default\n", name)
}
