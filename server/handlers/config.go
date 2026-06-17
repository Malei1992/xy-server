package handlers

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetConfig 返回解析后的 .env 内容。
// .env 不存在 → 返回空 map（前端需要看到 200 + 空对象，不希望 404）。
// .env 存在但读失败 → 500。
func GetConfig(envPath string, parse func(text string) map[string]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := os.ReadFile(envPath)
		if err != nil {
			if os.IsNotExist(err) {
				L.Info("get config: .env not found, return empty")
				c.JSON(http.StatusOK, gin.H{"env": map[string]string{}})
				return
			}
			L.Error("get config: read .env failed", zap.String("path", envPath), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"env": parse(string(data))})
	}
}

// editableKeys 白名单：PATCH /api/config 只允许修改这些 key。
// 扩展时只需在这里加 key + 在 validateValue 里加校验规则。
var editableKeys = map[string]bool{
	"SMTP_HOST":            true,
	"SMTP_PORT":            true,
	"SMTP_USERNAME":        true,
	"SMTP_PASSWORD":        true,
	"IMAP_HOST":            true,
	"IMAP_PORT":            true,
	"IMAP_USERNAME":        true,
	"IMAP_PASSWORD":        true,
	"EMAIL_REQUIRE_REVIEW": true,
	"REVIEWER_EMAIL":       true,
}

// numericRegex 端口字段必须为数字（允许空字符串）。
var numericRegex = regexp.MustCompile(`^\d+$`)

// PatchConfig 处理 PATCH /api/config。
// Body 形如 {"KEY": "value", ...}；KEY 必须在 editableKeys 内。
// 成功返回 {ok:true, env: <完整当前 .env>, updated: [<变化的 keys>]}。
func PatchConfig(envPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body map[string]string
		if err := c.ShouldBindJSON(&body); err != nil {
			L.Warn("patch config: invalid body", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body: " + err.Error()})
			return
		}
		if len(body) == 0 {
			L.Warn("patch config: empty body")
			c.JSON(http.StatusBadRequest, gin.H{"error": "empty body"})
			return
		}

		// 1) 校验 key 都在白名单
		for k := range body {
			if !editableKeys[k] {
				L.Warn("patch config: unknown key", zap.String("key", k))
				c.JSON(http.StatusBadRequest, gin.H{"error": "unknown or non-editable key: " + k})
				return
			}
		}

		// 2) 校验 value
		for k, v := range body {
			if err := validateValue(k, v); err != nil {
				L.Warn("patch config: value invalid", zap.String("key", k), zap.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s: %s", k, err.Error())})
				return
			}
		}

		// 3) 读当前 .env
		current, err := ReadEnvFile(envPath)
		if err != nil {
			L.Error("patch config: read .env failed", zap.String("path", envPath), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read .env: " + err.Error()})
			return
		}

		// 4) 应用 patch，收集真正变化的 keys
		updated := []string{}
		for k, v := range body {
			if current[k] != v {
				current[k] = v
				updated = append(updated, k)
			}
		}

		// 5) 写回（原子写）
		if err := WriteEnvFile(envPath, current); err != nil {
			L.Error("patch config: write .env failed", zap.String("path", envPath), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write .env: " + err.Error()})
			return
		}

		L.Info("patch config", zap.Strings("updated", updated))
		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"env":     current,
			"updated": updated,
		})
	}
}

// validateValue 对特定 key 做格式校验；空字符串视为"未设置"，一律通过（让用户能清空）。
// 未列出的 key 默认接受任意非空字符串。
func validateValue(key, value string) error {
	switch key {
	case "SMTP_PORT", "IMAP_PORT":
		if value == "" {
			return nil
		}
		if !numericRegex.MatchString(value) {
			return fmt.Errorf("must be numeric")
		}
	case "EMAIL_REQUIRE_REVIEW":
		if value == "" {
			return nil
		}
		switch strings.ToLower(value) {
		case "true", "false", "1", "0", "yes", "no":
			// ok
		default:
			return fmt.Errorf("must be true/false/1/0/yes/no")
		}
	case "REVIEWER_EMAIL":
		if value == "" {
			return nil
		}
		if !strings.Contains(value, "@") {
			return fmt.Errorf("must contain @")
		}
	}
	return nil
}