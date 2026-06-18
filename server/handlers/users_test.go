package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// usersPath 写到 t.TempDir() 里,避免污染真实 ./users.json。
// 每个 test 自己写一个固定内容的初始 users.json,语义自包含。
func newUsersRouter(t *testing.T, initial string) (*gin.Engine, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")
	if initial != "" {
		if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
			t.Fatalf("write initial users.json: %v", err)
		}
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.POST("/login", PostLogin(path))
	api.GET("/users", GetUsers(path))
	api.POST("/users", PostUser(path))
	api.PATCH("/users/:account", PatchUser(path))
	return r, path
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body: %v; body=%s", err, w.Body.String())
	}
	return m
}

func readUsersFile(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read users.json: %v", err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse users.json: %v", err)
	}
	return m
}

// ===== PostLogin =====

// 账号正则失败 → 400
func TestPostLogin_InvalidAccount(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123"}`)

	cases := []string{"", "has space", "中", "ab!", strings.Repeat("a", 21), "a-b"}
	for _, acc := range cases {
		t.Run(fmt.Sprintf("account=%q", acc), func(t *testing.T) {
			w := doJSON(t, r, http.MethodPost, "/api/login", map[string]string{
				"account":  acc,
				"password": "admin123",
			})
			if w.Code != http.StatusBadRequest {
				t.Errorf("want 400, got %d; body=%s", w.Code, w.Body.String())
			}
		})
	}
}

// 密码超长 → 400
func TestPostLogin_InvalidPassword(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123"}`)

	w := doJSON(t, r, http.MethodPost, "/api/login", map[string]string{
		"account":  "admin",
		"password": strings.Repeat("a", 21),
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for 21-char password, got %d", w.Code)
	}
}

// 账号不存在 → 404
func TestPostLogin_AccountNotFound(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123"}`)

	w := doJSON(t, r, http.MethodPost, "/api/login", map[string]string{
		"account":  "ghost",
		"password": "admin123",
	})
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d; body=%s", w.Code, w.Body.String())
	}
	body := decodeJSON(t, w)
	if body["ok"] != false {
		t.Errorf("ok = %v, want false", body["ok"])
	}
}

// 密码错 → 401
func TestPostLogin_WrongPassword(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123"}`)

	w := doJSON(t, r, http.MethodPost, "/api/login", map[string]string{
		"account":  "admin",
		"password": "wrong",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d; body=%s", w.Code, w.Body.String())
	}
	body := decodeJSON(t, w)
	if body["ok"] != false {
		t.Errorf("ok = %v, want false", body["ok"])
	}
}

// 登录成功 → 200 + account
func TestPostLogin_Success(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123"}`)

	w := doJSON(t, r, http.MethodPost, "/api/login", map[string]string{
		"account":  "admin",
		"password": "admin123",
	})
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d; body=%s", w.Code, w.Body.String())
	}
	body := decodeJSON(t, w)
	if body["account"] != "admin" {
		t.Errorf("account = %v, want admin", body["account"])
	}
}

// ===== GetUsers =====

// 列出所有账号(无 password 字段)
func TestGetUsers_List(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123","alice":"pw1","bob":"pw2"}`)

	w := doJSON(t, r, http.MethodGet, "/api/users", nil)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	var arr []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &arr); err != nil {
		t.Fatalf("decode: %v; body=%s", err, w.Body.String())
	}
	if len(arr) != 3 {
		t.Errorf("len = %d, want 3; body=%s", len(arr), w.Body.String())
	}
	seen := map[string]bool{}
	for _, item := range arr {
		if _, has := item["password"]; has {
			t.Errorf("item 应不含 password 字段: %v", item)
		}
		if acc, _ := item["account"].(string); acc != "" {
			seen[acc] = true
		}
	}
	for _, want := range []string{"admin", "alice", "bob"} {
		if !seen[want] {
			t.Errorf("missing account %q in list", want)
		}
	}
}

// 文件不存在时 GetUsers 仍要返 200 + 空数组
// newUsersRouter 在 initial="" 时不创建文件,正好对应"首次启动"场景
func TestGetUsers_EmptyOrMissing(t *testing.T) {
	r, _ := newUsersRouter(t, "")
	w := doJSON(t, r, http.MethodGet, "/api/users", nil)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var arr []any
	if err := json.Unmarshal(w.Body.Bytes(), &arr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(arr) != 0 {
		t.Errorf("len = %d, want 0", len(arr))
	}
}

// ===== PostUser =====

// 重名 → 409
func TestPostUser_Duplicate(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123"}`)

	w := doJSON(t, r, http.MethodPost, "/api/users", map[string]string{
		"account":  "admin",
		"password": "newpw",
	})
	if w.Code != http.StatusConflict {
		t.Errorf("want 409, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 校验失败 → 400(账号或密码)
func TestPostUser_InvalidInput(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123"}`)

	cases := []struct {
		name string
		body map[string]string
	}{
		{"bad account", map[string]string{"account": "ab!", "password": "pw1"}},
		{"password too long", map[string]string{"account": "alice", "password": strings.Repeat("x", 21)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(t, r, http.MethodPost, "/api/users", tc.body)
			if w.Code != http.StatusBadRequest {
				t.Errorf("want 400, got %d; body=%s", w.Code, w.Body.String())
			}
		})
	}
}

// 创建成功 → 201,文件里有新账号
func TestPostUser_Success(t *testing.T) {
	r, path := newUsersRouter(t, `{"admin":"admin123"}`)

	w := doJSON(t, r, http.MethodPost, "/api/users", map[string]string{
		"account":  "alice",
		"password": "pw1",
	})
	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d; body=%s", w.Code, w.Body.String())
	}
	body := decodeJSON(t, w)
	if body["account"] != "alice" {
		t.Errorf("account = %v, want alice", body["account"])
	}
	users := readUsersFile(t, path)
	if users["alice"] != "pw1" {
		t.Errorf("alice in file = %q, want pw1", users["alice"])
	}
	if users["admin"] != "admin123" {
		t.Errorf("admin 被覆盖: %q", users["admin"])
	}
}

// ===== PatchUser =====

// 旧密码错 → 401
func TestPatchUser_WrongOldPassword(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123"}`)

	w := doJSON(t, r, http.MethodPatch, "/api/users/admin", map[string]string{
		"oldPassword":         "wrong",
		"newPassword":         "newpw",
		"confirmNewPassword":  "newpw",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 新密码 != 确认 → 400
func TestPatchUser_PasswordMismatch(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123"}`)

	w := doJSON(t, r, http.MethodPatch, "/api/users/admin", map[string]string{
		"oldPassword":         "admin123",
		"newPassword":         "newpw",
		"confirmNewPassword":  "different",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 新密码长度超限 → 400
func TestPatchUser_NewPasswordTooLong(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123"}`)

	w := doJSON(t, r, http.MethodPatch, "/api/users/admin", map[string]string{
		"oldPassword":         "admin123",
		"newPassword":         strings.Repeat("a", 21),
		"confirmNewPassword":  strings.Repeat("a", 21),
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 账号不存在 → 404
func TestPatchUser_AccountNotFound(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123"}`)

	w := doJSON(t, r, http.MethodPatch, "/api/users/ghost", map[string]string{
		"oldPassword":         "admin123",
		"newPassword":         "newpw",
		"confirmNewPassword":  "newpw",
	})
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 改密码成功 → 200,后续用新密码能登
func TestPatchUser_Success(t *testing.T) {
	r, _ := newUsersRouter(t, `{"admin":"admin123"}`)

	w := doJSON(t, r, http.MethodPatch, "/api/users/admin", map[string]string{
		"oldPassword":         "admin123",
		"newPassword":         "newpw",
		"confirmNewPassword":  "newpw",
	})
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d; body=%s", w.Code, w.Body.String())
	}

	// 旧密码登不上
	wOld := doJSON(t, r, http.MethodPost, "/api/login", map[string]string{
		"account":  "admin",
		"password": "admin123",
	})
	if wOld.Code != http.StatusUnauthorized {
		t.Errorf("旧密码应失败,got %d; body=%s", wOld.Code, wOld.Body.String())
	}

	// 新密码能登
	wNew := doJSON(t, r, http.MethodPost, "/api/login", map[string]string{
		"account":  "admin",
		"password": "newpw",
	})
	if wNew.Code != http.StatusOK {
		t.Errorf("新密码应成功,got %d; body=%s", wNew.Code, wNew.Body.String())
	}
}

// ===== SeedDefaultAdmin =====

// 文件不存在 → 写入 admin
func TestSeedDefaultAdmin_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")

	if err := SeedDefaultAdmin(path); err != nil {
		t.Fatalf("SeedDefaultAdmin: %v", err)
	}
	users := readUsersFile(t, path)
	if users["admin"] != "admin123" {
		t.Errorf("admin = %q, want admin123", users["admin"])
	}
}

// 文件已存在有内容 → 不覆盖
func TestSeedDefaultAdmin_KeepsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")
	if err := os.WriteFile(path, []byte(`{"alice":"secret"}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := SeedDefaultAdmin(path); err != nil {
		t.Fatalf("SeedDefaultAdmin: %v", err)
	}
	users := readUsersFile(t, path)
	if users["alice"] != "secret" {
		t.Errorf("alice lost: %v", users)
	}
	if _, ok := users["admin"]; ok {
		t.Errorf("admin 不应被注入: %v", users)
	}
}

// 文件存在但内容是 {} 空对象 → 视为"无账号",写入 admin
func TestSeedDefaultAdmin_EmptyObjectSeeds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := SeedDefaultAdmin(path); err != nil {
		t.Fatalf("SeedDefaultAdmin: %v", err)
	}
	users := readUsersFile(t, path)
	if users["admin"] != "admin123" {
		t.Errorf("admin = %q, want admin123 (空对象应被 seed)", users["admin"])
	}
}

// ===== 并发 =====

// 多 goroutine 同时 create / patch / login,文件不应损坏
// 跑 `go test -race` 时能稳定通过即代表锁工作正常
func TestUsers_ConcurrentWrites(t *testing.T) {
	r, path := newUsersRouter(t, `{"admin":"admin123"}`)

	const writers = 8
	const ops = 20

	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			acc := fmt.Sprintf("user%d", i)
			// 1) create
			wCreate := doJSON(t, r, http.MethodPost, "/api/users", map[string]string{
				"account":  acc,
				"password": "pw",
			})
			if wCreate.Code != http.StatusCreated {
				t.Errorf("create %s: code=%d body=%s", acc, wCreate.Code, wCreate.Body.String())
				return
			}
			// 2) 多次 patch(跟踪当前密码,后续 patch 用上一轮新密码当旧密码)
			currentPw := "pw"
			for j := 0; j < ops; j++ {
				newPw := fmt.Sprintf("pw%d", j)
				wPatch := doJSON(t, r, http.MethodPatch, "/api/users/"+acc, map[string]string{
					"oldPassword":        currentPw,
					"newPassword":        newPw,
					"confirmNewPassword": newPw,
				})
				if wPatch.Code != http.StatusOK {
					t.Errorf("patch %s: code=%d body=%s", acc, wPatch.Code, wPatch.Body.String())
					return
				}
				currentPw = newPw
			}
		}()
	}
	wg.Wait()

	// 文件应仍是合法 JSON,且至少有 1+writers 个 key
	users := readUsersFile(t, path)
	if users["admin"] != "admin123" {
		t.Errorf("admin password changed: %v", users)
	}
	if len(users) < writers+1 {
		t.Errorf("users map 只有 %d 个,期望至少 %d", len(users), writers+1)
	}
}

// 跨账号并发 patch:多 goroutine 同时改不同账号的密码。
// 验证全局 usersMu 保护下不会串号(每个账号的最终密码应等于它自己最后一轮的 newPassword)。
// 跑 `go test -race` 时能稳定通过即代表锁工作正常。
func TestUsers_ConcurrentPatchesDifferentAccounts(t *testing.T) {
	r, path := newUsersRouter(t, `{"admin":"admin123"}`)

	const accounts = 12
	const ops = 10

	// 预创建 accounts 个账号
	for i := 0; i < accounts; i++ {
		acc := fmt.Sprintf("u%d", i)
		w := doJSON(t, r, http.MethodPost, "/api/users", map[string]string{
			"account":  acc,
			"password": "initial",
		})
		if w.Code != http.StatusCreated {
			t.Fatalf("precreate %s: code=%d body=%s", acc, w.Code, w.Body.String())
		}
	}

	// 每个 goroutine 改自己那个账号,跟踪 currentPw,最后一轮的 newPassword 写入一个期望表
	expected := make([]string, accounts)
	var wg sync.WaitGroup
	for i := 0; i < accounts; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			acc := fmt.Sprintf("u%d", i)
			currentPw := "initial"
			var lastPw string
			for j := 0; j < ops; j++ {
				newPw := fmt.Sprintf("pw_i%d_o%d", i, j)
				w := doJSON(t, r, http.MethodPatch, "/api/users/"+acc, map[string]string{
					"oldPassword":        currentPw,
					"newPassword":        newPw,
					"confirmNewPassword": newPw,
				})
				if w.Code != http.StatusOK {
					t.Errorf("patch %s j=%d: code=%d body=%s", acc, j, w.Code, w.Body.String())
					return
				}
				currentPw = newPw
				lastPw = newPw
			}
			expected[i] = lastPw
		}()
	}
	wg.Wait()

	// 校验每个账号的最终密码 = 它自己最后一轮的 newPassword(没串号)
	users := readUsersFile(t, path)
	for i := 0; i < accounts; i++ {
		acc := fmt.Sprintf("u%d", i)
		if users[acc] != expected[i] {
			t.Errorf("账号 %s 最终密码 = %q,期望 %q(可能串号)", acc, users[acc], expected[i])
		}
	}
	// admin 没被动过
	if users["admin"] != "admin123" {
		t.Errorf("admin password changed: %v", users)
	}
}
