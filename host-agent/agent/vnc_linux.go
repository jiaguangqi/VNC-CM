package agent

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var vncResolutionPattern = regexp.MustCompile(`^\d{3,5}x\d{3,5}$`)

func (a *Agent) createLinuxVNCDesktop(payload createDesktopPayload) error {
	if payload.Protocol != "vnc" {
		return fmt.Errorf("不支持的协议: %s", payload.Protocol)
	}
	if payload.Display <= 0 || payload.Port <= 0 || payload.WSPort <= 0 {
		return fmt.Errorf("VNC display/port 参数无效")
	}
	if !vncResolutionPattern.MatchString(payload.Resolution) {
		return fmt.Errorf("分辨率格式无效: %s", payload.Resolution)
	}

	vncBin, vncPassBin, effectiveBackend := vncBinaries(payload.VNCBackend)
	payload.VNCOptions = ""
	desktopCmd := "gnome-session"
	if payload.DesktopEnv == "xfce" {
		desktopCmd = "startxfce4"
	}
	colorDepth := payload.ColorDepth
	if colorDepth == 0 {
		colorDepth = 24
	}

	qUser := shellQuote(payload.Username)
	vncDir := fmt.Sprintf("/home/%s/.vnc", payload.Username)
	passwdPath := fmt.Sprintf("%s/rdp-%d.passwd", vncDir, payload.Display)
	qVNCDir := shellQuote(vncDir)
	qPasswdPath := shellQuote(passwdPath)
	qXstartupPath := shellQuote(vncDir + "/xstartup")

	commands := []string{
		fmt.Sprintf("id %s >/dev/null", qUser),
		fmt.Sprintf("install -d -m 700 -o %[1]s -g %[1]s %[2]s", qUser, qVNCDir),
		fmt.Sprintf("printf %%s %s | HOME=%s %s -f > %s && chown %s:%s %s && chmod 600 %s",
			shellQuote(payload.Password), shellQuote("/home/"+payload.Username), shellQuote(vncPassBin), qPasswdPath, qUser, qUser, qPasswdPath, qPasswdPath),
		fmt.Sprintf("printf '%%s\n' %s > %s && chmod 755 %s && chown %s:%s %s",
			shellQuoteArgs(agentXstartupLines(desktopCmd, payload.PerformanceProfile)), qXstartupPath, qXstartupPath, qUser, qUser, qXstartupPath),
	}

	for _, command := range commands {
		if output, err := runShell(command); err != nil {
			return fmt.Errorf("准备 VNC 会话失败: %w, output: %s", err, output)
		}
	}

	securityTypes := "-SecurityTypes VncAuth"
	if effectiveBackend == "turbovnc" {
		securityTypes = "-securitytypes None,Vnc"
	}
	payload.VNCOptions = strings.TrimSpace(payload.VNCOptions + " -rfbauth " + passwdPath)
	startInner := fmt.Sprintf("%s :%d -geometry %s -depth %d %s %s >/dev/null 2>&1 && echo success",
		vncBin, payload.Display, payload.Resolution, colorDepth, securityTypes, payload.VNCOptions)
	startCmd := fmt.Sprintf("su - %s -c %s", qUser, shellQuote(startInner))
	if output, err := runShell(startCmd); err != nil {
		return fmt.Errorf("启动 vncserver 失败: %w, output: %s", err, output)
	}

	wsCmd := websockifyStartCommand(payload.WSPort, payload.Port)
	if output, err := runShell(wsCmd); err != nil {
		return fmt.Errorf("启动 websockify 失败: %w, output: %s", err, output)
	}

	return nil
}

func (a *Agent) terminateLinuxVNCDesktop(payload terminateDesktopPayload) error {
	if payload.Display <= 0 {
		return nil
	}

	vncBin, _, _ := vncBinaries(payload.VNCBackend)
	qUser := shellQuote(payload.Username)
	stopInner := fmt.Sprintf("%s -kill :%d", vncBin, payload.Display)
	commands := []string{
		fmt.Sprintf("su - %s -c %s >/dev/null 2>&1 || true", qUser, shellQuote(stopInner)),
		fmt.Sprintf("pkill -f '[w]ebsockify.*%d' >/dev/null 2>&1 || true", payload.WSPort),
		fmt.Sprintf("rm -f %s >/dev/null 2>&1 || true", shellQuote(fmt.Sprintf("/home/%s/.vnc/rdp-%d.passwd", payload.Username, payload.Display))),
	}
	for _, command := range commands {
		if output, err := runShell(command); err != nil {
			return fmt.Errorf("清理 VNC 会话失败: %w, output: %s", err, output)
		}
	}
	return nil
}

func vncBinaries(backend string) (string, string, string) {
	if backend == "tigervnc" {
		return "vncserver", "vncpasswd", "tigervnc"
	}
	if _, err := os.Stat("/opt/TurboVNC/bin/Xvnc"); err == nil {
		return "/opt/TurboVNC/bin/vncserver", "/opt/TurboVNC/bin/vncpasswd", "turbovnc"
	}
	return "vncserver", "vncpasswd", "tigervnc"
}

func websockifyStartCommand(wsPort, vncPort int) string {
	return fmt.Sprintf(`log=$(mktemp /tmp/websockify-%d.XXXXXX); nohup websockify --web=/opt/noVNC --cert=/dev/null %d localhost:%d >"$log" 2>&1 & check_port() { if command -v ss >/dev/null 2>&1; then ss -ltn | grep -Eq "[:.]%d[[:space:]]"; else netstat -ltn 2>/dev/null | grep -Eq "[:.]%d[[:space:]]"; fi; }; for i in $(seq 1 10); do sleep 1; if check_port; then rm -f "$log"; exit 0; fi; done; cat "$log"; rm -f "$log"; exit 1`,
		wsPort, wsPort, vncPort, wsPort, wsPort)
}

func runShell(command string) (string, error) {
	output, err := exec.Command("sh", "-lc", command).CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func shellQuoteArgs(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, shellQuote(value))
	}
	return strings.Join(quoted, " ")
}

func agentXstartupLines(desktopCmd, profile string) []string {
	lines := []string{
		"#!/bin/sh",
		"unset SESSION_MANAGER",
		"unset DBUS_SESSION_BUS_ADDRESS",
	}
	if profile == "low_bandwidth" {
		lines = append(lines,
			"gsettings set org.gnome.desktop.interface enable-animations false >/dev/null 2>&1 || true",
			"xfconf-query -c xfwm4 -p /general/use_compositing -s false >/dev/null 2>&1 || true",
		)
	}
	return append(lines, "exec "+desktopCmd)
}
