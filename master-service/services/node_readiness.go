package services

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/remote-desktop/master-service/models"
)

type NodeReadinessService struct {
	encryptor *EncryptionService
}

type ReadinessCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type NodeReadinessResult struct {
	Ready             bool             `json:"ready"`
	CurrentUserExists bool             `json:"current_user_exists"`
	Checks            []ReadinessCheck `json:"checks"`
	Missing           []string         `json:"missing"`
	CheckedAt         time.Time        `json:"checked_at"`
}

func NewNodeReadinessService(encryptor *EncryptionService) *NodeReadinessService {
	return &NodeReadinessService{encryptor: encryptor}
}

func (s *NodeReadinessService) CheckHost(host models.Host, username string) NodeReadinessResult {
	result := NodeReadinessResult{
		Ready:     true,
		CheckedAt: time.Now().UTC(),
	}

	if host.AgentManaged && (host.SSHUsername == "" || host.SSHCredentialEncrypted == "") {
		result.CurrentUserExists = true
		result.addCheck("agent_managed", true, "SSH 未配置；桌面创建时由 Host Agent 在本机校验用户和组件")
		return result
	}

	client, err := s.dialHost(host)
	if err != nil {
		result.Ready = false
		result.addCheck("ssh", false, err.Error())
		return result
	}
	defer client.Close()
	result.addCheck("ssh", true, "")

	checks := []struct {
		name string
		cmd  string
	}{
		{"user", fmt.Sprintf("id %s >/dev/null 2>&1", shellQuote(username))},
		{"vncserver", "command -v vncserver >/dev/null 2>&1 || test -x /opt/TurboVNC/bin/vncserver"},
		{"vncpasswd", "command -v vncpasswd >/dev/null 2>&1 || test -x /opt/TurboVNC/bin/vncpasswd"},
		{"websockify", "command -v websockify >/dev/null 2>&1"},
		{"novnc", "test -f /opt/noVNC/vnc.html || test -f /usr/share/novnc/vnc.html"},
		{"gnome", "command -v gnome-session >/dev/null 2>&1"},
		{"xfce", "command -v startxfce4 >/dev/null 2>&1"},
	}

	for _, check := range checks {
		ok, output := runSSHCheck(client, check.cmd)
		if check.name == "user" {
			result.CurrentUserExists = ok
		}
		if !ok && check.name != "gnome" && check.name != "xfce" {
			result.Ready = false
		}
		result.addCheck(check.name, ok, output)
	}

	hasDesktopEnv := checkOK(result.Checks, "gnome") || checkOK(result.Checks, "xfce")
	if !hasDesktopEnv {
		result.Ready = false
		result.Missing = append(result.Missing, "desktop_environment")
	}

	return result
}

func (s *NodeReadinessService) CheckUserExists(host models.Host, username string) (bool, error) {
	if host.AgentManaged && (host.SSHUsername == "" || host.SSHCredentialEncrypted == "") {
		return true, nil
	}

	client, err := s.dialHost(host)
	if err != nil {
		return false, err
	}
	defer client.Close()

	ok, output := runSSHCheck(client, fmt.Sprintf("id %s >/dev/null 2>&1", shellQuote(username)))
	if !ok {
		return false, fmt.Errorf("宿主机上不存在用户 %s: %s", username, output)
	}
	return true, nil
}

func (s *NodeReadinessService) dialHost(host models.Host) (*ssh.Client, error) {
	if host.SSHUsername == "" || host.SSHCredentialEncrypted == "" {
		return nil, fmt.Errorf("宿主机 SSH 凭据未配置")
	}

	cred, err := s.encryptor.Decrypt(host.SSHCredentialEncrypted)
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

func runSSHCheck(client *ssh.Client, cmd string) (bool, string) {
	session, err := client.NewSession()
	if err != nil {
		return false, err.Error()
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return false, strings.TrimSpace(string(output))
	}
	return true, strings.TrimSpace(string(output))
}

func (r *NodeReadinessResult) addCheck(name string, ok bool, message string) {
	r.Checks = append(r.Checks, ReadinessCheck{Name: name, OK: ok, Message: message})
	if !ok {
		r.Missing = append(r.Missing, name)
	}
}

func checkOK(checks []ReadinessCheck, name string) bool {
	for _, check := range checks {
		if check.Name == name {
			return check.OK
		}
	}
	return false
}
