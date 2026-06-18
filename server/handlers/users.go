package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// usersMu 序列化对 users.json 的 read-modify-write。
// 沿用 openclaw_config.go 的全局单锁模式:并发写不会丢更新,rename 失败时 .tmp 清理。
var usersMu sync.Mutex

// accountRegex 账号白名单:英文 / 数字 / 下划线,1-20 字符。
// 锚定 ^ 和 $ 避免部分匹配。
var accountRegex = regexp.MustCompile(`^[A-Za-z0-9_]{1,20}$`)

// validateAccount 校验账号格式。
func validateAccount(s string) error {
	if !accountRegex.MatchString(s) {
		return fmt.Errorf("账号必须是 1-20 位英文/数字/下划线")
	}
	return nil
}

// validatePassword 校验密码长度(1-20 字符,不做复杂度)。
func validatePassword(s string) error {
	if l := len(s); l < 1 || l > 20 {
		return fmt.Errorf("密码长度必须 1-20 字符,实际 %d", l)
	}
	return nil
}

// loadUsers 读 users.json 解析成 map。文件不存在 → (空 map, nil) 以便启动 seed 流程。
// JSON 损坏 → 报错(让运维介入,避免静默丢数据)。
func loadUsers(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	var users map[string]string
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if users == nil {
		users = map[string]string{}
	}
	return users, nil
}

// saveUsers 把 users map 原子写回 path:先写 .tmp,再 rename。
// 按 key 字母序排序,输出稳定,便于 diff 和人读。
func saveUsers(path string, users map[string]string) error {
	keys := make([]string, 0, len(users))
	for k := range users {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 手写 JSON 避免额外 import;键值都加双引号,内嵌 " 转义为 \"。
	var buf []byte
	buf = append(buf, '{')
	for i, k := range keys {
		if i > 0 {
			buf = append(buf, ',')
		}
		kb, _ := json.Marshal(k)
		vb, _ := json.Marshal(users[k])
		buf = append(buf, kb...)
		buf = append(buf, ':')
		buf = append(buf, vb...)
	}
	buf = append(buf, '}', '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// SeedDefaultAdmin 在启动时调用:文件不存在或为空时写入默认 admin 账号。
// 文件存在且有内容 → 保持不变(避免覆盖已有账号)。
// 写入后打 INFO 日志,方便运维确认 seed 是否触发。
//
// 并发安全:本函数 + 4 个 handler 都用全局 usersMu 保护,**仅保证单进程内的
// read-modify-write 原子性**。若未来引入"外部进程 / 命令行工具"直接改 users.json,
// 进程内锁挡不住跨进程竞争,需要改用文件锁(flock)或外部互斥。
func SeedDefaultAdmin(usersPath string) error {
	usersMu.Lock()
	defer usersMu.Unlock()

	users, err := loadUsers(usersPath)
	if err != nil {
		return err
	}
	if len(users) > 0 {
		L.Info("users: existing users.json kept, skip seed", zap.String("path", usersPath))
		return nil
	}
	if err := saveUsers(usersPath, map[string]string{"admin": "admin123"}); err != nil {
		return err
	}
	L.Info("users: seeded default admin/admin123", zap.String("path", usersPath))
	return nil
}

// loginRequest / patchUserRequest 是 handler 用的请求体。
type loginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type patchUserRequest struct {
	OldPassword        string `json:"oldPassword"`
	NewPassword        string `json:"newPassword"`
	ConfirmNewPassword string `json:"confirmNewPassword"`
}

// PostLogin 处理 POST /api/login。
//   - 校验失败 → 400
//   - 账号不存在 → 404
//   - 密码错 → 401
//   - 成功 → 200 + {ok:true, account:<account>}
func PostLogin(usersPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body: " + err.Error()})
			return
		}
		if err := validateAccount(req.Account); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := validatePassword(req.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		usersMu.Lock()
		defer usersMu.Unlock()

		users, err := loadUsers(usersPath)
		if err != nil {
			L.Error("login: load users failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		pw, exists := users[req.Account]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "账号不存在"})
			return
		}
		if pw != req.Password {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "账号或密码错误"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "account": req.Account})
	}
}

// GetUsers 处理 GET /api/users,只返账号列表(不含密码)。
// 文件不存在 → 返空数组(启动早期 / 刚清空时正常)。
func GetUsers(usersPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		usersMu.Lock()
		defer usersMu.Unlock()

		users, err := loadUsers(usersPath)
		if err != nil {
			L.Error("list users: load failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out := make([]gin.H, 0, len(users))
		// 按 key 字母序输出,前端展示更稳定
		keys := make([]string, 0, len(users))
		for k := range users {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out = append(out, gin.H{"account": k})
		}
		c.JSON(http.StatusOK, out)
	}
}

// PostUser 处理 POST /api/users,新建账号。
//   - 校验失败 → 400
//   - 账号已存在 → 409
//   - 成功 → 201
func PostUser(usersPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body: " + err.Error()})
			return
		}
		if err := validateAccount(req.Account); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := validatePassword(req.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		usersMu.Lock()
		defer usersMu.Unlock()

		users, err := loadUsers(usersPath)
		if err != nil {
			L.Error("create user: load failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if _, exists := users[req.Account]; exists {
			c.JSON(http.StatusConflict, gin.H{"ok": false, "error": "账号已存在"})
			return
		}
		users[req.Account] = req.Password
		if err := saveUsers(usersPath, users); err != nil {
			L.Error("create user: save failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		L.Info("users: created", zap.String("account", req.Account))
		c.JSON(http.StatusCreated, gin.H{"ok": true, "account": req.Account})
	}
}

// DeleteUser 处理 DELETE /api/users/:account,删除指定账号。
//   - 路径 account 不合法 → 400
//   - 账号不存在 → 404
//   - 成功 → 200 + {ok:true}
//
// 不做"不能删自己"校验:后端无 token 不知道当前登录者,前端隐藏自己账号的删除按钮
// 来实现"自己不能删自己"的产品约束。
func DeleteUser(usersPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		account := c.Param("account")
		if err := validateAccount(account); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		usersMu.Lock()
		defer usersMu.Unlock()

		users, err := loadUsers(usersPath)
		if err != nil {
			L.Error("delete user: load failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if _, exists := users[account]; !exists {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "账号不存在"})
			return
		}
		delete(users, account)
		if err := saveUsers(usersPath, users); err != nil {
			L.Error("delete user: save failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		L.Info("users: deleted", zap.String("account", account))
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// PatchUser 处理 PATCH /api/users/:account,改密码。
//   - 校验失败 / newPassword != confirmNewPassword → 400
//   - 旧密码错 → 401
//   - 账号不存在 → 404
//   - 成功 → 200
func PatchUser(usersPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		account := c.Param("account")
		if err := validateAccount(account); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var req patchUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body: " + err.Error()})
			return
		}
		if err := validatePassword(req.NewPassword); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.NewPassword != req.ConfirmNewPassword {
			c.JSON(http.StatusBadRequest, gin.H{"error": "新密码与确认不一致"})
			return
		}

		usersMu.Lock()
		defer usersMu.Unlock()

		users, err := loadUsers(usersPath)
		if err != nil {
			L.Error("patch user: load failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		oldPw, exists := users[account]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "账号不存在"})
			return
		}
		if oldPw != req.OldPassword {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "旧密码错误"})
			return
		}
		users[account] = req.NewPassword
		if err := saveUsers(usersPath, users); err != nil {
			L.Error("patch user: save failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		L.Info("users: password changed", zap.String("account", account))
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
