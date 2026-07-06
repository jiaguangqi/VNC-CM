package services

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/models"
)

type DesktopHealthMonitor struct {
	encryptor *EncryptionService
	interval  time.Duration
	stopCh    chan struct{}
}

func NewDesktopHealthMonitor(encryptor *EncryptionService, interval time.Duration) *DesktopHealthMonitor {
	if interval <= 0 {
		interval = time.Minute
	}
	return &DesktopHealthMonitor{
		encryptor: encryptor,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

func (m *DesktopHealthMonitor) Start() {
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		m.reconcileRunningSessions()
		for {
			select {
			case <-ticker.C:
				m.reconcileRunningSessions()
			case <-m.stopCh:
				return
			}
		}
	}()
}

func (m *DesktopHealthMonitor) Stop() {
	close(m.stopCh)
}

func (m *DesktopHealthMonitor) reconcileRunningSessions() {
	var sessions []models.Session
	if err := database.DB.
		Where("status = ?", models.SessionStatusRunning).
		Preload("Host").
		Preload("User").
		Find(&sessions).Error; err != nil {
		log.Printf("桌面健康检查查询失败: %v", err)
		return
	}

	for _, session := range sessions {
		display, wsPort, err := sessionDisplayAndWSPort(session)
		if err != nil {
			m.markSessionError(session, fmt.Sprintf("连接信息无效: %v", err))
			continue
		}

		if err := m.checkRemoteDesktop(session.Host, session.User.Username, display, wsPort); err != nil {
			m.markSessionError(session, fmt.Sprintf("桌面进程健康检查失败: %v", err))
		}
	}
}

func sessionDisplayAndWSPort(session models.Session) (int, int, error) {
	if session.ConnectionInfo == "" {
		return 0, 0, fmt.Errorf("connection_info 为空")
	}

	var connInfo map[string]interface{}
	if err := json.Unmarshal([]byte(session.ConnectionInfo), &connInfo); err != nil {
		return 0, 0, err
	}

	displayValue, ok := connInfo["display"]
	if !ok {
		return 0, 0, fmt.Errorf("缺少 display")
	}
	display, ok := numericJSONValueToInt(displayValue)
	if !ok || display <= 0 {
		return 0, 0, fmt.Errorf("display 无效")
	}

	portValue, ok := connInfo["port"]
	if !ok {
		return 0, 0, fmt.Errorf("缺少 port")
	}
	port, ok := numericJSONValueToInt(portValue)
	if !ok || port <= 0 {
		return 0, 0, fmt.Errorf("port 无效")
	}

	return display, port + 200, nil
}

func numericJSONValueToInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case json.Number:
		i, err := v.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

func (m *DesktopHealthMonitor) checkRemoteDesktop(host models.Host, username string, display, wsPort int) error {
	client, err := m.dialHost(host)
	if err != nil {
		return err
	}
	defer client.Close()

	cmd := buildDesktopHealthCommand(username, display, wsPort)
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return fmt.Errorf("%w, output: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (m *DesktopHealthMonitor) dialHost(host models.Host) (*ssh.Client, error) {
	cred, err := m.encryptor.Decrypt(host.SSHCredentialEncrypted)
	if err != nil {
		return nil, fmt.Errorf("解密凭据失败: %w", err)
	}

	var authMethods []ssh.AuthMethod
	if host.SSHAuthType == "password" {
		authMethods = append(authMethods, ssh.Password(cred))
	} else {
		signer, err := ssh.ParsePrivateKey([]byte(cred))
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	config := &ssh.ClientConfig{
		User:            host.SSHUsername,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host.IPAddress, host.SSHPort)
	if host.SSHPort == 0 {
		addr = fmt.Sprintf("%s:22", host.IPAddress)
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH 连接失败: %w", err)
	}
	return client, nil
}

func buildDesktopHealthCommand(username string, display, wsPort int) string {
	userArg := shellQuote(username)
	displayArg := shellQuote(":" + strconv.Itoa(display))

	return fmt.Sprintf(`user=%s; display=%s; wsport=%d; if command -v ss >/dev/null 2>&1; then ss -ltn | grep -Eq "[:.]${wsport}[[:space:]]"; else netstat -ltn 2>/dev/null | grep -Eq "[:.]${wsport}[[:space:]]"; fi && ps -u "$user" -o args= | grep -Ev "grep" | grep -E "(Xvnc|Xtigervnc|vncserver).*${display}"`, userArg, displayArg, wsPort)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (m *DesktopHealthMonitor) markSessionError(session models.Session, message string) {
	connInfo := make(map[string]interface{})
	if session.ConnectionInfo != "" {
		_ = json.Unmarshal([]byte(session.ConnectionInfo), &connInfo)
	}
	connInfo["error"] = message
	connInfo["last_health_check_at"] = time.Now().UTC().Format(time.RFC3339)
	connInfoJSON, _ := json.Marshal(connInfo)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.Session{}).
			Where("id = ? AND status = ?", session.ID, models.SessionStatusRunning).
			Updates(map[string]interface{}{
				"status":          models.SessionStatusError,
				"connection_info": string(connInfoJSON),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if session.Host.CurrentSessions > 0 {
			return tx.Model(&models.Host{}).
				Where("id = ? AND current_sessions > 0", session.HostID).
				Update("current_sessions", gorm.Expr("current_sessions - 1")).Error
		}
		return nil
	})
	if err != nil {
		log.Printf("标记桌面异常失败 session_id=%s: %v", session.ID, err)
		return
	}

	log.Printf("桌面健康检查异常 session_id=%s host=%s: %s", session.ID, session.Host.Hostname, message)
}
