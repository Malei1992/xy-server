package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTestEngine 用 testdata/ 下的 fixture 搭一个 router。
func newTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 直接复用真实路由逻辑，但数据源指向 testdata。
	const testCRM = "testdata"
	const testEnv = "testdata/.env"

	api := r.Group("/api")
	api.GET("/index", indexHandlerForTest(testCRM))
	api.GET("/customers/:id", customerHandlerForTest(testCRM))
	api.GET("/emails/:id", emailHandlerForTest(testCRM))
	api.GET("/config", configHandlerForTest(testEnv))

	r.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

// ----- /healthz -----

func TestHealthz(t *testing.T) {
	r := newTestEngine(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("want body 'ok', got %q", w.Body.String())
	}
}

// ----- /api/index -----

func TestIndexHappy(t *testing.T) {
	r := newTestEngine(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/index", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("want application/json, got %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	customers, ok := body["customers"].([]any)
	if !ok || len(customers) != 2 {
		t.Fatalf("want customers len=2, got %v", body["customers"])
	}
}

func TestIndexNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/index", indexHandlerForTest("testdata/does-not-exist"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/index", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("want error field, got %v", body)
	}
}

// ----- /api/customers/:id -----

func TestCustomerHappy(t *testing.T) {
	r := newTestEngine(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/customers/CUST-test-001", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["id"] != "CUST-test-001" {
		t.Fatalf("want id=CUST-test-001, got %v", body["id"])
	}
}

func TestCustomerNotFound(t *testing.T) {
	r := newTestEngine(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/customers/NOT-EXIST", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("want error field, got %v", body)
	}
}

func TestCustomerPathTraversal(t *testing.T) {
	r := newTestEngine(t)
	w := httptest.NewRecorder()
	// ../etc/passwd 在 url 里要编码成 ..%2Fetc%2Fpasswd
	// gin 的 :id 只匹配单段，所以带 / 的 id 会被路由层直接 404
	req, _ := http.NewRequest(http.MethodGet, "/api/customers/..%2F..%2Fetc%2Fpasswd", nil)
	r.ServeHTTP(w, req)

	// 既不能 200；要么 safeJoin 拦下 → 400，要么路由不匹配 → 404
	if w.Code == http.StatusOK {
		t.Fatalf("path traversal must not return 200, body=%s", w.Body.String())
	}
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Fatalf("want 400 or 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCustomerDotDotID(t *testing.T) {
	// 单独的 ".." id 经过 +".json" 后变成 "...json"——不是路径穿越，
	// 应被当成不存在的文件，返回 404 而不是 200。
	//（slashed traversal 在 TestCustomerPathTraversal 里覆盖）
	r := newTestEngine(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/customers/..", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 (file does not exist), got %d body=%s", w.Code, w.Body.String())
	}
}

// GetCustomer 在 id 不以 CUST 开头时直接 404，哪怕 crm/customers/{id}.json 存在。
// 防御性过滤：避免 stray 文件被当成客户档案返回。
func TestCustomerRejectsNonCUSTPrefix(t *testing.T) {
	// 用 temp dir 造一个客户目录，里面放一个非 CUST 开头的 TEMP.json
	crmDir := t.TempDir()
	customersDir := crmDir + "/crm/customers"
	if err := os.MkdirAll(customersDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(customersDir+"/TEMP.json",
		[]byte(`{"id":"TEMP","basic":{"name":"stowaway"}}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/customers/:id", customerHandlerForTest(crmDir))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/customers/TEMP", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for non-CUST id, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("want error field, got %v", body)
	}
}

// ----- /api/emails/:id -----

func TestEmailHappy(t *testing.T) {
	r := newTestEngine(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/emails/MSG-test-001", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["id"] != "MSG-test-001" {
		t.Fatalf("want id=MSG-test-001, got %v", body["id"])
	}
}

func TestEmailNotFound(t *testing.T) {
	r := newTestEngine(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/emails/NO-SUCH-EMAIL", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("want error field, got %v", body)
	}
}

// ----- /api/config -----

func TestConfigHappy(t *testing.T) {
	r := newTestEngine(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/config", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.Env["KEY1"] != "value1" {
		t.Fatalf("want KEY1=value1, got %q", body.Env["KEY1"])
	}
	if body.Env["KEY2"] != "quoted value" {
		t.Fatalf("want KEY2=quoted value, got %q", body.Env["KEY2"])
	}
	if body.Env["KEY3"] != "single quoted" {
		t.Fatalf("want KEY3=single quoted, got %q", body.Env["KEY3"])
	}
	// 值里有等号时，只在第一个出现的位置切
	if body.Env["KEY_WITH_EQ"] != "a=b=c" {
		t.Fatalf("want KEY_WITH_EQ=a=b=c, got %q", body.Env["KEY_WITH_EQ"])
	}
}

func TestConfigMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/config", configHandlerForTest("testdata/does-not-exist.env"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/config", nil)
	r.ServeHTTP(w, req)

	// .env 缺失不应返回 404，应返回 200 + 空 env
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(body.Env) != 0 {
		t.Fatalf("want empty env map, got %v", body.Env)
	}
}