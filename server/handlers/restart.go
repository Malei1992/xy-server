package handlers

import (
	"bytes"
	"net/http"
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PostRestart 按顺序执行 commands 里的每条命令，把所有 stdout/stderr 合并返回。
// 任一条命令失败就整体返 500，但前面已执行的命令不回滚（docker compose up -d 本身幂等）。
// 设计成接收 [][]string 而非硬编码 docker 命令，方便测试时传入 echo/false 替身。
func PostRestart(commands [][]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var buf bytes.Buffer
		anyFailed := false
		for i, args := range commands {
			if len(args) == 0 {
				continue
			}
			cmd := exec.Command(args[0], args[1:]...)
			out, err := cmd.CombinedOutput()
			buf.WriteString("$ ")
			buf.WriteString(strings.Join(args, " "))
			buf.WriteByte('\n')
			buf.Write(out)
			if len(out) > 0 && out[len(out)-1] != '\n' {
				buf.WriteByte('\n')
			}
			if err != nil {
				anyFailed = true
				L.Error("restart step failed",
					zap.Int("step", i),
					zap.Strings("cmd", args),
					zap.Error(err),
					zap.String("output", string(out)),
				)
			} else {
				L.Info("restart step ok",
					zap.Int("step", i),
					zap.Strings("cmd", args),
					zap.Int("output_size", len(out)),
				)
			}
		}
		if anyFailed {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":  "restart failed",
				"output": buf.String(),
			})
			return
		}
		L.Info("restart ok", zap.Int("steps", len(commands)))
		c.JSON(http.StatusOK, gin.H{"ok": true, "output": buf.String()})
	}
}
