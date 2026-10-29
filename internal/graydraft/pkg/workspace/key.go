package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
)

// HeaderName 为前端 fetch 与 Go 服务端约定的工作区密钥头。
const HeaderName = "X-Graydraft-Workspace-Key"

const (
	minKeyLen = 1
	maxKeyLen = 512
)

// ErrInvalidKey 表示密钥为空、过长或仅空白。
var ErrInvalidKey = errors.New("invalid workspace key")

// NormalizeKey 校验并返回规范化后的密钥（trim）。
func NormalizeKey(raw string) (string, error) {
	k := strings.TrimSpace(raw)
	if len(k) < minKeyLen || len(k) > maxKeyLen {
		return "", ErrInvalidKey
	}
	return k, nil
}

// DirSegment 返回 dataDir/workspaces 下的单段目录名（SHA256 hex），避免把明文密钥写进路径。
func DirSegment(normalizedKey string) string {
	sum := sha256.Sum256([]byte(normalizedKey))
	return hex.EncodeToString(sum[:])
}

// Root 返回该密钥对应的工作区根目录（绝对/相对由 dataDir 决定）。
func Root(dataDir, normalizedKey string) string {
	return filepath.Join(dataDir, "workspaces", DirSegment(normalizedKey))
}
