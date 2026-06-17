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
	"go.uber.org/zap"

	"crm-server/handlers"
)

// TestEndToEnd_LogsWritten 模拟一次请求,验证日志文件里出现:
//   - middleware 的 http 日志
//   - handler 自己的业务日志(get customer)
// 端到端:InitLogger → gin engine → r.Use(RequestLogger) → handler 调 L.Info
func TestEndToEnd_LogsWritten(t *testing.T) {
	logsDir := t.TempDir()
	if err := handlers.InitLogger(logsDir); err != nil {
		t.Fatalf("InitLogger: %v", err)
	}
	t.Cleanup(handlers.ResetLoggerForTest)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(handlers.RequestLogger())
	base := t.TempDir()
	crmDir := filepath.Join(base, "crm")
	r.GET("/api/customers/:id", customerHandlerForTest(base))
	r.GET("/api/healthz", func(c *gin.Context) { c.String(200, "ok") })

	// 准备客户文件
	customersDir := filepath.Join(crmDir, "customers")
	if err := os.MkdirAll(customersDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(customersDir, "CUST-smoke-1.json"),
		[]byte(`{"id":"CUST-smoke-1","basic":{"name":"smoke"}}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// 健康检查
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/healthz", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("healthz want 200, got %d", w.Code)
	}

	// 客户 GET（命中）
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/customers/CUST-smoke-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("customer want 200, got %d body=%s", w.Code, w.Body.String())
	}

	// 客户 GET（404）
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/customers/CUST-NOT-EXIST", nil)
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("customer 404 want 404, got %d", w.Code)
	}

	// 同步刷盘
	if err := handlers.L.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	// 找今天的日志文件
	entries, _ := os.ReadDir(logsDir)
	var logFile string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".log") {
			logFile = filepath.Join(logsDir, e.Name())
			break
		}
	}
	if logFile == "" {
		t.Fatalf("没找到日志文件 in %s", logsDir)
	}
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	content := string(data)

	// 断言:
	// 1) middleware 写了 3 条 http 日志
	httpCount := strings.Count(content, `"msg":"http"`)
	if httpCount != 3 {
		t.Errorf("期望 3 条 http 日志,实际 %d\n文件内容:\n%s", httpCount, content)
	}
	// 2) handler 写了 "get customer" + "get customer: not found"
	if !strings.Contains(content, `"msg":"get customer"`) {
		t.Errorf("缺 get customer 日志,内容:\n%s", content)
	}
	if !strings.Contains(content, `"msg":"get customer: not found"`) {
		t.Errorf("缺 not found 日志,内容:\n%s", content)
	}
	// 3) caller 字段含 handler 文件名
	if !strings.Contains(content, "handlers/customers.go") {
		t.Errorf("缺 caller 文件名,内容:\n%s", content)
	}

	// 4) 解一条 http 日志确认字段完整
	var entry map[string]any
	for _, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, `"msg":"http"`) {
			continue
		}
		_ = json.Unmarshal([]byte(line), &entry)
		break
	}
	if entry == nil {
		t.Fatalf("没解出 http 日志 entry")
	}
	for _, key := range []string{"ts", "level", "msg", "caller", "method", "path", "status", "dur_ms"} {
		if _, ok := entry[key]; !ok {
			t.Errorf("http 日志缺字段 %q, entry: %v", key, entry)
		}
	}
	// caller 是 server 路径下的相对文件
	if caller, _ := entry["caller"].(string); !strings.Contains(caller, "middleware.go") {
		t.Errorf("middleware 日志 caller 应该含 middleware.go, 实际 %q", caller)
	}

	// 静默使用 zap 字段
	_ = zap.String("k", "v")
}
