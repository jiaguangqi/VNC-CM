// services/encryption.go - AES-256-GCM 凭据加密解密服务

package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// EncryptionService 凭据加密服务
type EncryptionService struct {
	masterKey []byte
}

// NewEncryptionService 创建加密服务，masterKey 必须恰好 32 字节
func NewEncryptionService(masterKey string) (*EncryptionService, error) {
	key := []byte(masterKey)
	if len(key) == 0 {
		return nil, fmt.Errorf("CREDENTIAL_MASTER_KEY 环境变量未设置")
	}
	// 若密钥长度不足 32 字节，使用 SHA-256 派生；若超过则截断至 32 字节
	if len(key) != 32 {
		key = deriveKey(key, 32)
	}
	return &EncryptionService{masterKey: key}, nil
}

// Encrypt 加密明文，返回 base64 编码的 ciphertext (含 nonce)
func (s *EncryptionService) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密 base64 编码的 ciphertext，返回明文
func (s *EncryptionService) Decrypt(ciphertextB64 string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("密文长度不足")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}

	return string(plaintext), nil
}

// deriveKey 简单密钥派生：循环填充或截断至目标长度
func deriveKey(key []byte, length int) []byte {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = key[i%len(key)]
	}
	return result
}
