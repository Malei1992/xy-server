package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 写一个测试用 .env fixture，返回路径
func writeTestEnv(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// newPatchEngine 构造一个带 PATCH 路由的 gin engine
func newPatchEngine(t *testing.T, envPath string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.PATCH("/config", patchConfigHandlerForTest(envPath))
	return r
}

func doPatch(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ---------- Happy path ----------

func TestPatchConfig_Happy(t *testing.T) {
	envPath := writeTestEnv(t, `SMTP_HOST=old.host
SMTP_PORT=587
SMTP_USERNAME=user
IMAP_HOST=imap.old
IMAP_PORT=993
EMAIL_REQUIRE_REVIEW=true
REVIEWER_EMAIL=old@example.com
`)

	r := newPatchEngine(t, envPath)
	body := `{"SMTP_HOST":"new.host","IMAP_PORT":"143"}`
	w := doPatch(t, r, body)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		OK      bool              `json:"ok"`
		Env     map[string]string `json:"env"`
		Updated []string          `json:"updated"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Errorf("want ok=true, got false")
	}
	if resp.Env["SMTP_HOST"] != "new.host" {
		t.Errorf("want SMTP_HOST=new.host, got %q", resp.Env["SMTP_HOST"])
	}
	if resp.Env["IMAP_PORT"] != "143" {
		t.Errorf("want IMAP_PORT=143, got %q", resp.Env["IMAP_PORT"])
	}
	// 其他 keys 应保留
	if resp.Env["SMTP_PORT"] != "587" {
		t.Errorf("want SMTP_PORT=587 preserved, got %q", resp.Env["SMTP_PORT"])
	}
	if resp.Env["SMTP_USERNAME"] != "user" {
		t.Errorf("want SMTP_USERNAME=user preserved, got %q", resp.Env["SMTP_USERNAME"])
	}
	// updated 应包含这两个 key，顺序无所谓
	updatedSet := map[string]bool{}
	for _, k := range resp.Updated {
		updatedSet[k] = true
	}
	if !updatedSet["SMTP_HOST"] || !updatedSet["IMAP_PORT"] {
		t.Errorf("want updated=[SMTP_HOST,IMAP_PORT], got %v", resp.Updated)
	}

	// 重新读 .env 校验持久化
	gotEnv := readTestEnvFile(t, envPath)
	if gotEnv["SMTP_HOST"] != "new.host" || gotEnv["IMAP_PORT"] != "143" {
		t.Errorf("file not updated correctly: %v", gotEnv)
	}
	if gotEnv["SMTP_PORT"] != "587" {
		t.Errorf("SMTP_PORT lost: %v", gotEnv)
	}
}

// ---------- 未知 key ----------

func TestPatchConfig_UnknownKey(t *testing.T) {
	envPath := writeTestEnv(t, "SMTP_HOST=foo\n")
	r := newPatchEngine(t, envPath)
	w := doPatch(t, r, `{"SMTP_HOOST":"x"}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SMTP_HOOST") || !strings.Contains(w.Body.String(), "unknown or non-editable key") {
		t.Errorf("want error mentioning SMTP_HOOST, got %s", w.Body.String())
	}
}

// ---------- 非数字端口 ----------

func TestPatchConfig_BadPort(t *testing.T) {
	envPath := writeTestEnv(t, "SMTP_HOST=foo\n")
	r := newPatchEngine(t, envPath)
	w := doPatch(t, r, `{"SMTP_PORT":"abc"}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SMTP_PORT") || !strings.Contains(w.Body.String(), "numeric") {
		t.Errorf("want error mentioning SMTP_PORT and numeric, got %s", w.Body.String())
	}
}

// ---------- 空 body ----------

func TestPatchConfig_EmptyBody(t *testing.T) {
	envPath := writeTestEnv(t, "SMTP_HOST=foo\n")
	r := newPatchEngine(t, envPath)
	w := doPatch(t, r, `{}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "empty body") {
		t.Errorf("want 'empty body' in error, got %s", w.Body.String())
	}
}

// ---------- 无效 JSON ----------

func TestPatchConfig_InvalidJSON(t *testing.T) {
	envPath := writeTestEnv(t, "SMTP_HOST=foo\n")
	r := newPatchEngine(t, envPath)
	w := doPatch(t, r, `"not json"`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid json body") {
		t.Errorf("want 'invalid json body' in error, got %s", w.Body.String())
	}
}

// ---------- 原子性 ----------

func TestPatchConfig_AtomicNoTmpLeft(t *testing.T) {
	envPath := writeTestEnv(t, "SMTP_HOST=foo\n")
	r := newPatchEngine(t, envPath)
	w := doPatch(t, r, `{"SMTP_HOST":"bar"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(envPath + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("want .tmp to not exist, stat err = %v", err)
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Errorf("want .env to exist, stat err = %v", err)
	}
}

// ---------- 无变化 ----------

func TestPatchConfig_NoChange(t *testing.T) {
	envPath := writeTestEnv(t, "SMTP_HOST=foo\nSMTP_PORT=587\n")
	r := newPatchEngine(t, envPath)
	w := doPatch(t, r, `{"SMTP_HOST":"foo"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		OK      bool              `json:"ok"`
		Env     map[string]string `json:"env"`
		Updated []string          `json:"updated"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Errorf("want ok=true")
	}
	if len(resp.Updated) != 0 {
		t.Errorf("want updated empty, got %v", resp.Updated)
	}
}

// ---------- bool 字段 ----------

func TestPatchConfig_BoolValid(t *testing.T) {
	envPath := writeTestEnv(t, "EMAIL_REQUIRE_REVIEW=true\n")
	r := newPatchEngine(t, envPath)
	w := doPatch(t, r, `{"EMAIL_REQUIRE_REVIEW":"yes"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchConfig_BoolInvalid(t *testing.T) {
	envPath := writeTestEnv(t, "EMAIL_REQUIRE_REVIEW=true\n")
	r := newPatchEngine(t, envPath)
	w := doPatch(t, r, `{"EMAIL_REQUIRE_REVIEW":"abc"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// ---------- 缺失文件 ----------

func TestPatchConfig_CreateFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	// 不预写文件

	r := newPatchEngine(t, envPath)
	w := doPatch(t, r, `{"SMTP_HOST":"created.host"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	// 文件应已创建
	got := readTestEnvFile(t, envPath)
	if got["SMTP_HOST"] != "created.host" {
		t.Errorf("want SMTP_HOST=created.host, got %v", got)
	}
}

// ---------- 白名单外 key ----------

func TestPatchConfig_NonWhitelistKey(t *testing.T) {
	envPath := writeTestEnv(t, "SMTP_HOST=foo\n")
	r := newPatchEngine(t, envPath)
	w := doPatch(t, r, `{"PATH":"/etc/passwd"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "PATH") || !strings.Contains(w.Body.String(), "unknown or non-editable key") {
		t.Errorf("want error about PATH, got %s", w.Body.String())
	}
}

// ---------- helper ----------

func readTestEnvFile(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return parseEnv(string(data))
}
