package main

// 本文件：customers.go（GET /api/customers）的端到端测试。
// 全部用 t.TempDir() 隔离，绝不污染真实 data/。
//
// 端点行为：
//   GET /api/customers → 列出所有客户档案（crm/customers/*.json 原文拼成数组）
//
// 行为约定：
//   - 目录不存在 / 空 → 200 + []
//   - 单个文件损坏 / 空文件 / 非 .json / 子目录 → 跳过，不影响其他文件
//   - 按文件名（id）升序排序，输出稳定

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gin-gonic/gin"
)

const customersListRelDir = "crm/customers"

// newCustomersListEngine 搭一个最小 router：含 GET /api/customers。
func newCustomersListEngine(t *testing.T, crmDir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.GET("/customers", listCustomersHandlerForTest(crmDir))
	return r
}

// doCustomersListGet 发起 GET /api/customers 请求。
func doCustomersListGet(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/api/customers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// writeCustomerFile 在 dir/customers/{id}.json 写一段 JSON。
func writeCustomerFile(t *testing.T, dir, id, content string) {
	t.Helper()
	full := filepath.Join(dir, customersListRelDir, id+".json")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// ===== 基础场景 =====

// 1. 客户目录不存在 → 200 + []
func TestListCustomersDirMissing(t *testing.T) {
	crmDir := t.TempDir()
	r := newCustomersListEngine(t, crmDir)

	w := doCustomersListGet(t, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body == nil {
		t.Errorf("want non-nil empty array, got null")
	}
	if len(body) != 0 {
		t.Errorf("want empty, got %v", body)
	}
}

// 2. 客户目录为空 → 200 + []
func TestListCustomersDirEmpty(t *testing.T) {
	crmDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(crmDir, customersListRelDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	r := newCustomersListEngine(t, crmDir)

	w := doCustomersListGet(t, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body == nil {
		t.Errorf("want non-nil empty array, got null")
	}
	if len(body) != 0 {
		t.Errorf("want empty, got %v", body)
	}
}

// 3. 多文件 → 全部解析，按 id 升序
func TestListCustomersMultiple(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerFile(t, crmDir, "CUST-1", `{
		"id":"CUST-1","basic":{"name":"A","country":"泰国","industry":"X","contacts":"a@x","phones":""}
	}`)
	writeCustomerFile(t, crmDir, "CUST-2", `{
		"id":"CUST-2","basic":{"name":"B","country":"越南","industry":"Y","contacts":"b@x","phones":""}
	}`)

	w := doCustomersListGet(t, newCustomersListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("want 2, got %d", len(body))
	}
	if body[0]["id"] != "CUST-1" || body[1]["id"] != "CUST-2" {
		t.Errorf("want sorted by id, got %v %v", body[0]["id"], body[1]["id"])
	}
}

// 4. 损坏 JSON → 跳过
func TestListCustomersSkipsCorrupt(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerFile(t, crmDir, "CUST-good", `{"id":"CUST-good","basic":{"name":"G"}}`)
	writeCustomerFile(t, crmDir, "CUST-bad", `{this is not valid json`)

	w := doCustomersListGet(t, newCustomersListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("want 1 (corrupt skipped), got %d: %v", len(body), body)
	}
	if body[0]["id"] != "CUST-good" {
		t.Errorf("want CUST-good, got %v", body[0]["id"])
	}
}

// 5. 非 .json 后缀 → 跳过
func TestListCustomersIgnoresNonJSON(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerFile(t, crmDir, "CUST-good", `{"id":"CUST-good","basic":{"name":"G"}}`)
	if err := os.WriteFile(filepath.Join(crmDir, customersListRelDir, "README.md"), []byte("# x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	w := doCustomersListGet(t, newCustomersListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Errorf("want 1, got %d", len(body))
	}
}

// 6. 子目录 → 跳过
func TestListCustomersIgnoresSubdirs(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerFile(t, crmDir, "CUST-good", `{"id":"CUST-good","basic":{"name":"G"}}`)
	if err := os.MkdirAll(filepath.Join(crmDir, customersListRelDir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	w := doCustomersListGet(t, newCustomersListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Errorf("want 1, got %d", len(body))
	}
}

// 7. 空文件 → 跳过
func TestListCustomersSkipsEmptyFile(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerFile(t, crmDir, "CUST-good", `{"id":"CUST-good","basic":{"name":"G"}}`)
	if err := os.WriteFile(filepath.Join(crmDir, customersListRelDir, "CUST-empty.json"), []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	w := doCustomersListGet(t, newCustomersListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Errorf("want 1 (empty skipped), got %d", len(body))
	}
}

// 8. 中文 customer name round-trip
func TestListCustomersChineseRoundTrip(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerFile(t, crmDir, "CUST-1", `{
		"id":"CUST-1",
		"basic":{"name":"泰国正大集团","country":"泰国","industry":"综合","contacts":"a@cp.co.th","phones":"+66"},
		"engagement":{"status":"active","intent_level":"A"},
		"prospecting":{"grade":"A","source_extracted_at":"2026-06-15T10:00:00Z"}
	}`)

	w := doCustomersListGet(t, newCustomersListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("want 1, got %d", len(body))
	}
	basic := body[0]["basic"].(map[string]any)
	if basic["name"] != "泰国正大集团" {
		t.Errorf("name lost: %v", basic["name"])
	}
	engagement := body[0]["engagement"].(map[string]any)
	if engagement["intent_level"] != "A" {
		t.Errorf("intent_level lost: %v", engagement["intent_level"])
	}
}

// 9. 4 条按 id 字母升序
func TestListCustomersSortedByID(t *testing.T) {
	crmDir := t.TempDir()
	ids := []string{"CUST-c", "CUST-a", "CUST-d", "CUST-b"}
	for _, id := range ids {
		writeCustomerFile(t, crmDir, id, `{"id":"`+id+`","basic":{"name":"`+id+`"}}`)
	}

	w := doCustomersListGet(t, newCustomersListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := make([]string, 0, len(body))
	for _, b := range body {
		got = append(got, b["id"].(string))
	}
	want := []string{"CUST-a", "CUST-b", "CUST-c", "CUST-d"}
	if !stringSlicesEqual(got, want) {
		t.Errorf("want sorted %v, got %v", want, got)
	}
	// sanity: 触发 sort 包使用
	_ = sort.Strings
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 10. 不以 CUST 开头的 .json → 跳过（list 跳过，single GET 返 404）
func TestListCustomersIgnoresNonCUSTPrefix(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerFile(t, crmDir, "CUST-good", `{"id":"CUST-good","basic":{"name":"G"}}`)
	// 几个非 CUST 前缀的 .json：TEMP.json / PRJ-stowaway.json / cust-lowercase.json
	if err := os.WriteFile(filepath.Join(crmDir, customersListRelDir, "TEMP.json"),
		[]byte(`{"id":"TEMP","basic":{"name":"stowaway"}}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, customersListRelDir, "PRJ-stowaway.json"),
		[]byte(`{"id":"PRJ-stowaway"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, customersListRelDir, "cust-lowercase.json"),
		[]byte(`{"id":"cust-lowercase"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	w := doCustomersListGet(t, newCustomersListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("want 1 (only CUST-good), got %d: %v", len(body), body)
	}
	if body[0]["id"] != "CUST-good" {
		t.Errorf("want CUST-good, got %v", body[0]["id"])
	}
}

// 11. 全是非 CUST 前缀 → 200 + []
func TestListCustomersEmptyWhenAllNonCUST(t *testing.T) {
	crmDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(crmDir, customersListRelDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, customersListRelDir, "TEMP.json"),
		[]byte(`{"id":"TEMP"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, customersListRelDir, "PRJ-1.json"),
		[]byte(`{"id":"PRJ-1"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	w := doCustomersListGet(t, newCustomersListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body == nil {
		t.Errorf("want non-nil empty array, got null")
	}
	if len(body) != 0 {
		t.Errorf("want empty, got %v", body)
	}
}
