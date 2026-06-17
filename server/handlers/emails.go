package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetEmail 返回 <crmDir>/crm/emails/{id}.json。
// 路径非法 → 400；文件不存在 → 404；其他 IO 错 → 500。
func GetEmail(crmDir string, safeJoin func(root, rel string) (string, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		full, err := safeJoin(crmDir+"/crm/emails", id+".json")
		if err != nil {
			L.Warn("get email: safeJoin failed", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		data, err := readJSONFile(full)
		if err != nil {
			status := http.StatusInternalServerError
			if err.Error() == "file not found" {
				status = http.StatusNotFound
				L.Warn("get email: not found", zap.String("id", id))
			} else {
				L.Error("get email: read failed", zap.String("id", id), zap.Error(err))
			}
			c.JSON(status, gin.H{"error": "email " + id + " not found"})
			return
		}
		L.Info("get email", zap.String("id", id))
		c.Data(http.StatusOK, "application/json; charset=utf-8", data)
	}
}