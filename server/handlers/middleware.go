package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestLogger gin 中间件:每个 HTTP 请求记一条 Info 日志。
// 字段: method / path / status / dur_ms。
// 在 main.go 里 r.Use(handlers.RequestLogger()) 启用。
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		L.Info("http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("dur_ms", time.Since(start)),
		)
	}
}
