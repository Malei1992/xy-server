package main

// 本文件：customers.go（PATCH /api/customers/:id）的端到端测试。
// 全部用 t.TempDir() 隔离，绝不污染真实 data/。
//
// 端点行为：
//   PATCH /api/customers/:id → 更新客户 basic.contacts 和/或 basic.phones
//   body 接受 string 或 []string 两种形态；返回更新后的完整客户 JSON。

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

// newCustomersPatchEngine 搭一个最小 router：含 PATCH /api/customers/:id。
func newCustomersPatchEngine(t *testing.T, crmDir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.PATCH("/customers/:id", patchCustomerHandlerForTest(crmDir))
	return r
}

// doCustomersPatch 发起 PATCH /api/customers/:id 请求。
func doCustomersPatch(t *testing.T, r *gin.Engine, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPatch, "/api/customers/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// patchReadCustomerFile 读回客户 JSON 文件用于断言。
func patchReadCustomerFile(t *testing.T, crmDir, id string) map[string]any {
	t.Helper()
	full := filepath.Join(crmDir, customersListRelDir, id+".json")
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return doc
}

// writeCustomerPatchFixture 写入一个测试用客户 JSON 文件。
func writeCustomerPatchFixture(t *testing.T, crmDir, id, content string) {
	t.Helper()
	writeCustomerFile(t, crmDir, id, content)
}

// ----- Happy path -----

// 1. PATCH contacts as string → 200 + 验证文件落盘
func TestPatchCustomerContactsString(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerPatchFixture(t, crmDir, "CUST-001", `{
		"id":"CUST-001",
		"basic":{"name":"Test Co","country":"泰国","industry":"X","contacts":"old@a.com","phones":"+66"}
	}`)

	r := newCustomersPatchEngine(t, crmDir)
	body := `{"contacts":"new@b.com"}`
	w := doCustomersPatch(t, r, "CUST-001", body)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	// 验证响应含更新后的 contacts
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	basic := resp["basic"].(map[string]any)
	if basic["contacts"] != "new@b.com" {
		t.Errorf("want contacts=new@b.com, got %v", basic["contacts"])
	}

	// 验证文件落盘
	doc := patchReadCustomerFile(t, crmDir, "CUST-001")
	got := doc["basic"].(map[string]any)
	if got["contacts"] != "new@b.com" {
		t.Errorf("disk: want contacts=new@b.com, got %v", got["contacts"])
	}
	if got["phones"] != "+66" {
		t.Errorf("disk: phones should be preserved, got %v", got["phones"])
	}
}

// 2. PATCH phones as string → 200
func TestPatchCustomerPhonesString(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerPatchFixture(t, crmDir, "CUST-002", `{
		"id":"CUST-002",
		"basic":{"name":"Test Co","country":"泰国","contacts":"a@x.com","phones":"+66"}
	}`)

	r := newCustomersPatchEngine(t, crmDir)
	w := doCustomersPatch(t, r, "CUST-002", `{"phones":"+86"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	doc := patchReadCustomerFile(t, crmDir, "CUST-002")
	basic := doc["basic"].(map[string]any)
	if basic["phones"] != "+86" {
		t.Errorf("want phones=+86, got %v", basic["phones"])
	}
	if basic["contacts"] != "a@x.com" {
		t.Errorf("contacts should be preserved, got %v", basic["contacts"])
	}
}

// 3. PATCH contacts as array → 200
func TestPatchCustomerContactsArray(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerPatchFixture(t, crmDir, "CUST-003", `{
		"id":"CUST-003",
		"basic":{"name":"Array Co","contacts":"old@a.com","phones":"+66"}
	}`)

	r := newCustomersPatchEngine(t, crmDir)
	w := doCustomersPatch(t, r, "CUST-003", `{"contacts":["a@x.com","b@y.com"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	doc := patchReadCustomerFile(t, crmDir, "CUST-003")
	basic := doc["basic"].(map[string]any)
	contacts, ok := basic["contacts"].([]any)
	if !ok {
		t.Fatalf("want contacts as array, got %T %v", basic["contacts"], basic["contacts"])
	}
	if len(contacts) != 2 || contacts[0] != "a@x.com" || contacts[1] != "b@y.com" {
		t.Errorf("want [a@x.com b@y.com], got %v", contacts)
	}
}

// 4. PATCH both contacts & phones → 200
func TestPatchCustomerBoth(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerPatchFixture(t, crmDir, "CUST-004", `{
		"id":"CUST-004",
		"basic":{"name":"Both Co","contacts":"old@a.com","phones":"+66"}
	}`)

	r := newCustomersPatchEngine(t, crmDir)
	w := doCustomersPatch(t, r, "CUST-004", `{"contacts":"new@c.com","phones":"+86"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	doc := patchReadCustomerFile(t, crmDir, "CUST-004")
	basic := doc["basic"].(map[string]any)
	if basic["contacts"] != "new@c.com" {
		t.Errorf("want contacts=new@c.com, got %v", basic["contacts"])
	}
	if basic["phones"] != "+86" {
		t.Errorf("want phones=+86, got %v", basic["phones"])
	}
}

// 5. PATCH both as arrays → 200
func TestPatchCustomerBothArrays(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerPatchFixture(t, crmDir, "CUST-005", `{
		"id":"CUST-005",
		"basic":{"name":"Arrays Co","contacts":"old@a.com","phones":"+66"}
	}`)

	r := newCustomersPatchEngine(t, crmDir)
	w := doCustomersPatch(t, r, "CUST-005", `{"contacts":["c1@x.com","c2@y.com"],"phones":["+86","+87"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	doc := patchReadCustomerFile(t, crmDir, "CUST-005")
	basic := doc["basic"].(map[string]any)
	contacts, ok := basic["contacts"].([]any)
	if !ok {
		t.Fatalf("want contacts as array, got %T %v", basic["contacts"], basic["contacts"])
	}
	if len(contacts) != 2 || contacts[0] != "c1@x.com" {
		t.Errorf("contacts: want [c1@x.com ...], got %v", contacts)
	}
	phones, ok := basic["phones"].([]any)
	if !ok {
		t.Fatalf("want phones as array, got %T %v", basic["phones"], basic["phones"])
	}
	if len(phones) != 2 || phones[0] != "+86" {
		t.Errorf("phones: want [+86 ...], got %v", phones)
	}
}

// ----- Error cases -----

// 6. body 为空（空 JSON）→ 400
func TestPatchCustomerEmptyBody(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerPatchFixture(t, crmDir, "CUST-006", `{"id":"CUST-006","basic":{"name":"N","contacts":"a"}}`)

	r := newCustomersPatchEngine(t, crmDir)
	w := doCustomersPatch(t, r, "CUST-006", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "at least one of contacts or phones is required") {
		t.Errorf("want 'at least one of contacts or phones is required', got %s", w.Body.String())
	}
}

// 7. body JSON 格式错误 → 400
func TestPatchCustomerInvalidJSON(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerPatchFixture(t, crmDir, "CUST-007", `{"id":"CUST-007","basic":{"name":"N"}}`)

	r := newCustomersPatchEngine(t, crmDir)
	w := doCustomersPatch(t, r, "CUST-007", `not json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid json body") {
		t.Errorf("want 'invalid json body', got %s", w.Body.String())
	}
}

// 8. customer_id 不以 CUST 开头 → 404
func TestPatchCustomerNonCUSTPrefix(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerPatchFixture(t, crmDir, "CUST-008", `{"id":"CUST-008","basic":{"name":"N","contacts":"a"}}`)

	r := newCustomersPatchEngine(t, crmDir)
	w := doCustomersPatch(t, r, "OTHER-008", `{"contacts":"b"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// 9. 客户文件不存在 → 404
func TestPatchCustomerNotFound(t *testing.T) {
	crmDir := t.TempDir()

	r := newCustomersPatchEngine(t, crmDir)
	w := doCustomersPatch(t, r, "CUST-009", `{"contacts":"b"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// 10. 保留原有字段不被覆盖
func TestPatchCustomerPreservesOtherFields(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerPatchFixture(t, crmDir, "CUST-010", `{
		"id":"CUST-010",
		"basic":{"name":"Preserve Co","country":"泰国","industry":"综合","contacts":"a@x.com","phones":"+66"},
		"engagement":{"status":"active","intent_level":"A"},
		"prospecting":{"grade":"A"}
	}`)

	r := newCustomersPatchEngine(t, crmDir)
	w := doCustomersPatch(t, r, "CUST-010", `{"contacts":"new@z.com"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	doc := patchReadCustomerFile(t, crmDir, "CUST-010")
	// 检查 basic 下其他字段
	basic := doc["basic"].(map[string]any)
	if basic["name"] != "Preserve Co" {
		t.Errorf("name should be preserved, got %v", basic["name"])
	}
	if basic["country"] != "泰国" {
		t.Errorf("country should be preserved, got %v", basic["country"])
	}
	if basic["industry"] != "综合" {
		t.Errorf("industry should be preserved, got %v", basic["industry"])
	}
	// 检查 basic 外的顶级字段
	if doc["id"] != "CUST-010" {
		t.Errorf("id should be preserved, got %v", doc["id"])
	}
	eng := doc["engagement"].(map[string]any)
	if eng["status"] != "active" {
		t.Errorf("engagement.status should be preserved, got %v", eng["status"])
	}
}

// 11. 中文 contacts → 200（round-trip）
func TestPatchCustomerChineseContacts(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerPatchFixture(t, crmDir, "CUST-011", `{
		"id":"CUST-011",
		"basic":{"name":"中文公司","contacts":"old@中文.com","phones":"+86"}
	}`)

	r := newCustomersPatchEngine(t, crmDir)
	w := doCustomersPatch(t, r, "CUST-011", `{"contacts":"新联系人@公司.cn"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	// 响应验证
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	basic := resp["basic"].(map[string]any)
	if basic["contacts"] != "新联系人@公司.cn" {
		t.Errorf("want contacts=新联系人@公司.cn, got %v", basic["contacts"])
	}

	// 文件落盘验证
	doc := patchReadCustomerFile(t, crmDir, "CUST-011")
	got := doc["basic"].(map[string]any)
	if got["contacts"] != "新联系人@公司.cn" {
		t.Errorf("disk: want contacts=新联系人@公司.cn, got %v", got["contacts"])
	}
	if got["phones"] != "+86" {
		t.Errorf("phone should be preserved, got %v", got["phones"])
	}
}

// 12. contacts 传了非法类型（数字）→ 400
func TestPatchCustomerInvalidContactsType(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerPatchFixture(t, crmDir, "CUST-012", `{
		"id":"CUST-012",
		"basic":{"name":"N","contacts":"a","phones":"b"}
	}`)

	r := newCustomersPatchEngine(t, crmDir)
	w := doCustomersPatch(t, r, "CUST-012", `{"contacts":12345}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "contacts: must be a string or array of strings") {
		t.Errorf("want contacts type error, got %s", w.Body.String())
	}
}

// 13. 原子写无 .tmp 残留
func TestPatchCustomerAtomicNoTmp(t *testing.T) {
	crmDir := t.TempDir()
	writeCustomerPatchFixture(t, crmDir, "CUST-013", `{
		"id":"CUST-013",
		"basic":{"name":"Atomic Co","contacts":"a@x.com","phones":"+66"}
	}`)

	r := newCustomersPatchEngine(t, crmDir)
	w := doCustomersPatch(t, r, "CUST-013", `{"contacts":"b@y.com"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	full := filepath.Join(crmDir, customersListRelDir, "CUST-013.json")
	if _, err := os.Stat(full + ".tmp"); !os.IsNotExist(err) {
		t.Errorf(".tmp should not exist, stat err=%v", err)
	}
	if _, err := os.Stat(full); err != nil {
		t.Errorf("target file should exist: %v", err)
	}
}
