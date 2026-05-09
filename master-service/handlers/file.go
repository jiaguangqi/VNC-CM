// handlers/file.go - 文件传输 API
package handlers

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/models"
	"github.com/remote-desktop/master-service/services"
)

type FileHandler struct {
	encryptor *services.EncryptionService
}

type FileInfoItem struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
	IsDir   bool      `json:"is_dir"`
}

func NewFileHandler(encryptor *services.EncryptionService) *FileHandler {
	return &FileHandler{encryptor: encryptor}
}

// getSftpClient 获取指定桌面会话的 SFTP 客户端
func (h *FileHandler) getSftpClient(sessionID string, userID uuid.UUID) (*sftp.Client, *models.Session, func(), func(string) error, error) {
	var session models.Session
	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).
		Preload("Host").Preload("User").First(&session).Error; err != nil {
		return nil, nil, nil, nil, err
	}

	if session.Host.ID == uuid.Nil || session.User.Username == "" {
		return nil, nil, nil, nil, fmt.Errorf("会话缺少宿主机或用户信息")
	}

	host := session.Host
	linuxUser := session.User.Username

	cred, err := h.encryptor.Decrypt(host.SSHCredentialEncrypted)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("解密凭据失败: %w", err)
	}

	var authMethods []ssh.AuthMethod
	if host.SSHAuthType == "password" {
		authMethods = append(authMethods, ssh.Password(cred))
	} else {
		signer, err := ssh.ParsePrivateKey([]byte(cred))
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("解析私钥失败: %w", err)
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
		return nil, nil, nil, nil, fmt.Errorf("SSH 连接失败: %w", err)
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("SFTP 初始化失败: %w", err)
	}

	base := fmt.Sprintf("/home/%s", linuxUser)
	_ = sftpClient.MkdirAll(base)

	closeFn := func() {
		sftpClient.Close()
		client.Close()
	}

	runCmd := func(cmd string) error {
		session, err := client.NewSession()
		if err != nil {
			return err
		}
		defer session.Close()
		return session.Run(cmd)
	}

	return sftpClient, &session, closeFn, runCmd, nil
}

func resolveRemotePath(linuxUser, relPath string) string {
	base := fmt.Sprintf("/home/%s", linuxUser)
	absPath := filepath.Clean(filepath.Join(base, relPath))
	if !strings.HasPrefix(absPath, base) {
		return base
	}
	return absPath
}

// ListFiles 列出远程目录
func (h *FileHandler) ListFiles(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效用户ID"})
		return
	}
	sessionID := c.Param("id")
	relPath := c.Query("path")
	if relPath == "" {
		relPath = "."
	}

	sftpClient, session, closeFn, _, err := h.getSftpClient(sessionID, uid)
	if err != nil {
		status := http.StatusInternalServerError
		if err == gorm.ErrRecordNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	defer closeFn()

	absPath := resolveRemotePath(session.User.Username, relPath)
	info, err := sftpClient.Stat(absPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "目录不存在"})
		return
	}
	if !info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径不是目录"})
		return
	}
	entries, err := sftpClient.ReadDir(absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取目录失败: " + err.Error()})
		return
	}
	result := make([]FileInfoItem, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		result = append(result, FileInfoItem{
			Name:    name,
			Path:    filepath.Join(relPath, name),
			Size:    e.Size(),
			Mode:    e.Mode().String(),
			ModTime: e.ModTime(),
			IsDir:   e.IsDir(),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"path":    relPath,
		"entries": result,
	})
}

// UploadFile 上传文件，支持 relativePath 以保留目录结构
//
// 参数：
//   - file:           文件内容（multipart）
//   - path:           基础上传目录（相对于用户家目录）
//   - relativePath:   可选。文件的相对路径，如 "dir1/dir2/file.txt"
//                     若提供，会自动创建子目录并保留目录结构
func (h *FileHandler) UploadFile(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效用户ID"})
		return
	}
	sessionID := c.Param("id")

	// 基础路径（相对于家目录）
	baseRelPath := c.PostForm("path")
	if baseRelPath == "" {
		baseRelPath = "."
	}

	// 文件的相对路径（可选，用于保留目录结构）
	relativeFilePath := c.PostForm("relativePath")
	// 去掉开头可能的 ./
	relativeFilePath = strings.TrimPrefix(relativeFilePath, "./")
	relativeFilePath = strings.TrimPrefix(relativeFilePath, "/")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件"})
		return
	}
	defer file.Close()

	sftpClient, session, closeFn, runCmd, err := h.getSftpClient(sessionID, uid)
	if err != nil {
		status := http.StatusInternalServerError
		if err == gorm.ErrRecordNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	defer closeFn()

	// 计算最终上传目录
	baseAbsPath := resolveRemotePath(session.User.Username, baseRelPath)
	uploadDir := baseAbsPath
	uploadFileName := filepath.Base(header.Filename)

	if relativeFilePath != "" {
		dirPart := filepath.Dir(relativeFilePath)
		if dirPart != "." && dirPart != "" && dirPart != "/" {
			uploadDir = filepath.Join(baseAbsPath, dirPart)
		}
		uploadFileName = filepath.Base(relativeFilePath)
	}

	// 确保目标目录存在（含中间目录）
	if err := sftpClient.MkdirAll(uploadDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败: " + err.Error()})
		return
	}

	remoteFilePath := filepath.Join(uploadDir, uploadFileName)
	remoteFile, err := sftpClient.Create(remoteFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建远程文件失败: " + err.Error()})
		return
	}
	defer remoteFile.Close()

	written, err := io.Copy(remoteFile, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败: " + err.Error()})
		return
	}

	// 将文件及所在目录树所有权递归改为目标用户
	if runCmd != nil {
		chownErr := runCmd(fmt.Sprintf("chown -R %s:%s %s",
			session.User.Username, session.User.Username, uploadDir))
		if chownErr != nil {
			// 记录日志但不阻断响应
			fmt.Printf("[file] chown failed for %s: %v\n", uploadDir, chownErr)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "上传成功",
		"file":    uploadFileName,
		"size":    written,
		"path":    remoteFilePath,
	})
}

// DownloadFile 下载远程文件/目录
func (h *FileHandler) DownloadFile(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效用户ID"})
		return
	}
	sessionID := c.Param("id")
	relPath := c.Query("path")
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少路径参数"})
		return
	}
	sftpClient, session, closeFn, _, err := h.getSftpClient(sessionID, uid)
	if err != nil {
		status := http.StatusInternalServerError
		if err == gorm.ErrRecordNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	defer closeFn()

	absPath := resolveRemotePath(session.User.Username, relPath)
	info, err := sftpClient.Stat(absPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	if info.IsDir() {
		zipTemp, err := os.CreateTemp("", "download-*.zip")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时文件失败"})
			return
		}
		defer os.Remove(zipTemp.Name())
		defer zipTemp.Close()
		zw := zip.NewWriter(zipTemp)
		if err := zipRemoteDir(sftpClient, absPath, "", zw); err != nil {
			zw.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "打包失败: " + err.Error()})
			return
		}
		zw.Close()
		stat, _ := zipTemp.Stat()
		zipTemp.Seek(0, 0)
		baseName := filepath.Base(relPath) + ".zip"
		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", baseName))
		c.Header("Content-Length", fmt.Sprintf("%d", stat.Size()))
		io.Copy(c.Writer, zipTemp)
		c.Status(http.StatusOK)
		return
	}

	remoteFile, err := sftpClient.Open(absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "打开文件失败: " + err.Error()})
		return
	}
	defer remoteFile.Close()
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(relPath)))
	c.Header("Content-Length", fmt.Sprintf("%d", info.Size()))
	io.Copy(c.Writer, remoteFile)
	c.Status(http.StatusOK)
}

func zipRemoteDir(sftpClient *sftp.Client, absDir, zipPrefix string, zw *zip.Writer) error {
	entries, err := sftpClient.ReadDir(absDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		absPath := filepath.Join(absDir, name)
		zipName := path.Join(zipPrefix, name)
		if e.IsDir() {
			if err := zipRemoteDir(sftpClient, absPath, zipName, zw); err != nil {
				return err
			}
		} else {
			f, err := sftpClient.Open(absPath)
			if err != nil {
				return err
			}
			defer f.Close()
			w, err := zw.Create(zipName)
			if err != nil {
				return err
			}
			_, err = io.Copy(w, f)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// isNoSuchFile 判断 SFTP 错误是否等于 "文件不存在"
// 不同 SFTP 服务器返回的错误码和消息格式不一致，故对消息做字符串兜底匹配
func isNoSuchFile(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// 字符串匹配兜底（容错各种服务器表述）
	if strings.Contains(msg, "no such file") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "not found") {
		return true
	}
	return false
}

// removeDirectoryRecursive 递归删除 SFTP 目录及其内容
// 对符号链接、硬链接及并发删除等边界情况做了容错处理
func removeDirectoryRecursive(sftpClient *sftp.Client, dirPath string) error {
	entries, err := sftpClient.ReadDir(dirPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())

		// 用 Lstat（不跟随符号链接）来获取文件真实信息。
		// 如果 entry 是 broken symlink，Stat 会返回 "no such file"，
		// 但 Lstat 能正确返回 symlink 自身的信息。
		info, statErr := sftpClient.Lstat(fullPath)
		if statErr != nil {
			if isNoSuchFile(statErr) {
				continue
			}
			return fmt.Errorf("lstat %q: %w", entry.Name(), statErr)
		}

		if info.IsDir() {
			// 递归删除子目录
			if err := removeDirectoryRecursive(sftpClient, fullPath); err != nil {
				return err
			}
			// 删除空子目录（容错 "no such file"）
			if err := sftpClient.RemoveDirectory(fullPath); err != nil {
				if !isNoSuchFile(err) {
					return fmt.Errorf("rmdir %q: %w", entry.Name(), err)
				}
			}
		} else {
			// 删除文件（容错 "no such file"）
			if err := sftpClient.Remove(fullPath); err != nil {
				if !isNoSuchFile(err) {
					return fmt.Errorf("remove %q: %w", entry.Name(), err)
				}
			}
		}
	}
	// 删除当前目录自身
	return sftpClient.RemoveDirectory(dirPath)
}

// DeleteFile 删除远程文件或目录
func (h *FileHandler) DeleteFile(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效用户ID"})
		return
	}
	sessionID := c.Param("id")
	relPath := c.Query("path")
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少路径参数"})
		return
	}
	sftpClient, session, closeFn, _, err := h.getSftpClient(sessionID, uid)
	if err != nil {
		status := http.StatusInternalServerError
		if err == gorm.ErrRecordNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	defer closeFn()
	absPath := resolveRemotePath(session.User.Username, relPath)
	fi, err := sftpClient.Stat(absPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	if fi.IsDir() {
		err = removeDirectoryRecursive(sftpClient, absPath)
	} else {
		err = sftpClient.Remove(absPath)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// Mkdir 创建远程目录
func (h *FileHandler) Mkdir(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效用户ID"})
		return
	}
	sessionID := c.Param("id")
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sftpClient, session, closeFn, runCmd, err := h.getSftpClient(sessionID, uid)
	if err != nil {
		status := http.StatusInternalServerError
		if err == gorm.ErrRecordNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	defer closeFn()
	absPath := resolveRemotePath(session.User.Username, req.Path)
	if err := sftpClient.MkdirAll(absPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败: " + err.Error()})
		return
	}
	if runCmd != nil {
		_ = runCmd(fmt.Sprintf("chown %s:%s %s",
			session.User.Username, session.User.Username, absPath))
	}
	c.JSON(http.StatusOK, gin.H{"message": "创建目录成功"})
}
