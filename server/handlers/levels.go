package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetLevels 读 <crmDir>/<relPath> 的 JSON 文件。
// 文件不存在或为空 → 返回按 defaultLevels 填充全空字符串的 map。
// 例如 defaultLevels=["A","B","C"] + 文件缺失 → {"A":"","B":"","C":""}。
// 损坏的 JSON → 500。
func GetLevels(crmDir, relPath string, defaultLevels []string) gin.HandlerFunc {
	fullPath := filepath.Join(crmDir, relPath)
	return func(c *gin.Context) {
		levels, err := ReadLevelFile(fullPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read: " + err.Error()})
			return
		}
		// 缺失/空文件时 ReadLevelFile 已返空 map；这里把 defaultLevels 的 key 全填上空字符串
		for _, k := range defaultLevels {
			if _, ok := levels[k]; !ok {
				levels[k] = ""
			}
		}
		c.JSON(http.StatusOK, levels)
	}
}

// PatchLevels PATCH <crmDir>/<relPath> 的 JSON 内容。
// Body keys 必须 == defaultLevels 集合（不多不少）；value 任意字符串。
// 替换式写入，原子写（tmp + rename），父目录自动建。
func PatchLevels(crmDir, relPath string, defaultLevels []string) gin.HandlerFunc {
	fullPath := filepath.Join(crmDir, relPath)
	allowedSet := make(map[string]bool, len(defaultLevels))
	for _, k := range defaultLevels {
		allowedSet[k] = true
	}
	kind := strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	return func(c *gin.Context) {
		var body map[string]string
		if err := c.ShouldBindJSON(&body); err != nil {
			L.Warn("patch levels: invalid body", zap.String("kind", kind), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body: " + err.Error()})
			return
		}
		if len(body) == 0 {
			L.Warn("patch levels: empty body", zap.String("kind", kind))
			c.JSON(http.StatusBadRequest, gin.H{"error": "empty body"})
			return
		}

		// 1) 校验：所有 keys 必须在 defaultLevels 内
		var badKeys []string
		for k := range body {
			if !allowedSet[k] {
				badKeys = append(badKeys, k)
			}
		}
		if len(badKeys) > 0 {
			sort.Strings(badKeys)
			L.Warn("patch levels: unknown keys", zap.String("kind", kind), zap.Strings("bad", badKeys))
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("unknown level key(s): %s (allowed: %s)",
					strings.Join(badKeys, ","), strings.Join(defaultLevels, ",")),
			})
			return
		}

		// 2) 校验：必须 defaultLevels 全部 keys 都齐（替换语义）
		if len(body) != len(defaultLevels) {
			L.Warn("patch levels: missing keys", zap.String("kind", kind), zap.Int("got", len(body)), zap.Int("want", len(defaultLevels)))
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("body must contain all %d levels %v, got %d",
					len(defaultLevels), defaultLevels, len(body)),
			})
			return
		}

		// 3) 落盘（原子写，父目录自动建）
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			L.Error("patch levels: mkdir failed", zap.String("kind", kind), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "mkdir: " + err.Error()})
			return
		}
		if err := WriteLevelFile(fullPath, body); err != nil {
			L.Error("patch levels: write failed", zap.String("kind", kind), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write: " + err.Error()})
			return
		}
		L.Info("patch levels", zap.String("kind", kind), zap.Int("n", len(body)))
		c.JSON(http.StatusOK, body)
	}
}
