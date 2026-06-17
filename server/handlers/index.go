package handlers

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetIndex 返回 <crmDir>/crm/index.json 的内容。
// 文件不存在或读失败 → 404（按契约）。
func GetIndex(crmDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := crmDir + "/crm/index.json"
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				L.Warn("get index: not found", zap.String("path", path))
				c.JSON(http.StatusNotFound, gin.H{"error": "index.json not found"})
				return
			}
			L.Error("get index: read failed", zap.String("path", path), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", data)
	}
}