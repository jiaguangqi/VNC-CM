// database/db.go - PostgreSQL 连接管理与自动迁移

package database

import (
	"fmt"
	"log"

	"github.com/remote-desktop/master-service/config"
	"github.com/remote-desktop/master-service/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Init 初始化数据库连接并执行自动迁移
func Init(cfg *config.DatabaseConfig) error {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // 开发阶段可设为 Info，生产用 Silent
	})
	if err != nil {
		return fmt.Errorf("数据库连接失败: %w", err)
	}

	// 连接池配置
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(10)

	// 自动迁移
	if err := autoMigrate(); err != nil {
		return fmt.Errorf("自动迁移失败: %w", err)
	}

	log.Println("数据库连接与迁移成功")
	return nil
}

// autoMigrate 执行所有模型的自动迁移
func autoMigrate() error {
	return DB.AutoMigrate(
		&models.User{},
		&models.Host{},
		&models.Session{},
		&models.Collaboration{},
		&models.AuditLog{},
	)
}
