package main

// 本文件：opportunities.go（GET /api/opportunities）的端到端测试。
// 全部用 t.TempDir() 隔离，绝不污染真实 data/。
//
// 端点行为：
//   GET /api/opportunities → 列出所有公开信息（crm/opportunities/*.json），
//   按 customer_id 反查客户档案拿 customer_name 后一起返回。
//
// 行为约定：
//   - 商机目录不存在 / 空 → 200 + []
//   - customer_id 在客户目录里找不到 → 仍返回记录，customer_name 字段空字符串
//   - 损坏的商机文件 → 跳过；损坏的客户文件 → 该条 customer_name 字段空
//   - 文件存在但 customers 目录不存在 → 商机记录仍返回，customer_name 字段空
//   - 仅读取 OPP 开头的 .json 文件

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

const (
	opportunitiesListRelDir  = "crm/opportunities"
	opportunitiesCustomersDir = "crm/customers"
)

// newOpportunitiesListEngine 搭一个最小 router：含 GET /api/opportunities。
func newOpportunitiesListEngine(t *testing.T, crmDir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.GET("/opportunities", listOpportunitiesHandlerForTest(crmDir, opportunitiesListRelDir, opportunitiesCustomersDir))
	return r
}

// doOpportunitiesListGet 发起 GET /api/opportunities 请求。
func doOpportunitiesListGet(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/api/opportunities", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// writeOpportunityJSON 在 dir/opportunities/{id}.json 写一段 JSON。
func writeOpportunityJSON(t *testing.T, dir, id, content string) {
	t.Helper()
	full := filepath.Join(dir, opportunitiesListRelDir, id+".json")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// writeCustomerJSONForOpportunities 在 dir/customers/{id}.json 写一段 JSON。
func writeCustomerJSONForOpportunities(t *testing.T, dir, id, content string) {
	t.Helper()
	full := filepath.Join(dir, opportunitiesCustomersDir, id+".json")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// ===== 基础场景 =====

// 1. 商机目录不存在 → 200 + []
func TestListOpportunitiesDirMissing(t *testing.T) {
	crmDir := t.TempDir()
	r := newOpportunitiesListEngine(t, crmDir)

	w := doOpportunitiesListGet(t, r)
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

// 2. 商机目录为空 → 200 + []
func TestListOpportunitiesDirEmpty(t *testing.T) {
	crmDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(crmDir, opportunitiesListRelDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	r := newOpportunitiesListEngine(t, crmDir)

	w := doOpportunitiesListGet(t, r)
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

// 3. 单个商机 + 匹配的客户 → 字段全部合并
func TestListOpportunitiesJoinCustomer(t *testing.T) {
	crmDir := t.TempDir()
	writeOpportunityJSON(t, crmDir, "OPP-1", `{
		"id":"OPP-1",
		"opportunity_name":"新厂投资",
		"customer_id":"CUST-1",
		"opportunity_info":"占地 200 亩，预计投资 5 亿美元",
		"source_url":"https://example.com/news/123",
		"source_type":"新闻搜索",
		"status":"待评估",
		"notes":"与张三跟进重叠",
		"created_at":"2026-06-15T10:00:00Z",
		"updated_at":"2026-06-16T10:00:00Z"
	}`)
	writeCustomerJSONForOpportunities(t, crmDir, "CUST-1", `{
		"id":"CUST-1",
		"basic":{"name":"Siam Cement Group","contacts":"contact@example.com","country":"泰国","industry":"工业"}
	}`)

	w := doOpportunitiesListGet(t, newOpportunitiesListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("want 1, got %d", len(body))
	}
	p := body[0]
	if p["opportunity_name"] != "新厂投资" {
		t.Errorf("opportunity_name lost: %v", p["opportunity_name"])
	}
	if p["source_type"] != "新闻搜索" {
		t.Errorf("source_type lost: %v", p["source_type"])
	}
	if p["status"] != "待评估" {
		t.Errorf("status lost: %v", p["status"])
	}
	if p["customer_name"] != "Siam Cement Group" {
		t.Errorf("customer_name not joined: %v", p["customer_name"])
	}
	if p["source_url"] != "https://example.com/news/123" {
		t.Errorf("source_url lost: %v", p["source_url"])
	}
	if p["notes"] != "与张三跟进重叠" {
		t.Errorf("notes lost: %v", p["notes"])
	}
}

// 4. customer_id 在客户目录里不存在 → 商机记录仍返回，customer_name 字段空
func TestListOpportunitiesMissingCustomer(t *testing.T) {
	crmDir := t.TempDir()
	writeOpportunityJSON(t, crmDir, "OPP-1", `{
		"id":"OPP-1","opportunity_name":"x","customer_id":"CUST-ghost",
		"source_type":"行业报告","status":"待评估",
		"created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)

	w := doOpportunitiesListGet(t, newOpportunitiesListEngine(t, crmDir))
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
	if body[0]["customer_name"] != "" {
		t.Errorf("want empty customer_name, got %v", body[0]["customer_name"])
	}
}

// 5. 损坏的商机文件 → 跳过；损坏的客户文件 → 该条 customer_name 字段空
func TestListOpportunitiesSkipsCorrupt(t *testing.T) {
	crmDir := t.TempDir()
	writeOpportunityJSON(t, crmDir, "OPP-good", `{
		"id":"OPP-good","opportunity_name":"好","customer_id":"CUST-1",
		"source_type":"新闻搜索","status":"待评估",
		"created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)
	writeOpportunityJSON(t, crmDir, "OPP-bad", `{this is not valid json`)
	writeCustomerJSONForOpportunities(t, crmDir, "CUST-1", `{"id":"CUST-1","basic":{"name":"N1"}}`)
	writeCustomerJSONForOpportunities(t, crmDir, "CUST-corrupt", `{not valid`)

	w := doOpportunitiesListGet(t, newOpportunitiesListEngine(t, crmDir))
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
	if body[0]["id"] != "OPP-good" {
		t.Errorf("want OPP-good, got %v", body[0]["id"])
	}
	if body[0]["customer_name"] != "N1" {
		t.Errorf("customer_name should still join, got %v", body[0]["customer_name"])
	}
}

// 6. 客户目录不存在 → 商机记录仍返回，customer_name 字段空
func TestListOpportunitiesCustomerDirMissing(t *testing.T) {
	crmDir := t.TempDir()
	writeOpportunityJSON(t, crmDir, "OPP-1", `{
		"id":"OPP-1","opportunity_name":"x","customer_id":"CUST-1",
		"source_type":"行业报告","status":"待评估",
		"created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)

	w := doOpportunitiesListGet(t, newOpportunitiesListEngine(t, crmDir))
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
	if body[0]["customer_name"] != "" {
		t.Errorf("customer_name should be empty when customer dir missing, got %v", body[0]["customer_name"])
	}
}

// 7. 中文 round-trip
func TestListOpportunitiesChineseRoundTrip(t *testing.T) {
	crmDir := t.TempDir()
	writeOpportunityJSON(t, crmDir, "OPP-1", `{
		"id":"OPP-1",
		"opportunity_name":"泰国正大集团拟新建食品加工厂",
		"opportunity_info":"占地约 200 亩，预计投资 5 亿美元，2027 年投产",
		"source_url":"https://example.com/news/456",
		"source_type":"招标公告",
		"status":"跟进中",
		"notes":"已发邀请邮件，等回复",
		"customer_id":"CUST-1",
		"created_at":"2026-06-15T10:00:00Z",
		"updated_at":"2026-06-16T11:00:00Z"
	}`)
	writeCustomerJSONForOpportunities(t, crmDir, "CUST-1", `{
		"id":"CUST-1",
		"basic":{"name":"泰国正大集团","contacts":"contact@cp.co.th"}
	}`)

	w := doOpportunitiesListGet(t, newOpportunitiesListEngine(t, crmDir))
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
	p := body[0]
	if p["opportunity_name"] != "泰国正大集团拟新建食品加工厂" {
		t.Errorf("opportunity_name lost: %v", p["opportunity_name"])
	}
	if p["opportunity_info"] != "占地约 200 亩，预计投资 5 亿美元，2027 年投产" {
		t.Errorf("opportunity_info lost: %v", p["opportunity_info"])
	}
	if p["customer_name"] != "泰国正大集团" {
		t.Errorf("customer_name lost: %v", p["customer_name"])
	}
	if p["notes"] != "已发邀请邮件，等回复" {
		t.Errorf("notes lost: %v", p["notes"])
	}
}

// 8. 4 条按 id 升序
func TestListOpportunitiesSortedByID(t *testing.T) {
	crmDir := t.TempDir()
	ids := []string{"OPP-c", "OPP-a", "OPP-d", "OPP-b"}
	for _, id := range ids {
		writeOpportunityJSON(t, crmDir, id, `{
			"id":"`+id+`","opportunity_name":"`+id+`","source_type":"新闻搜索","status":"待评估",
			"created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
		}`)
	}

	w := doOpportunitiesListGet(t, newOpportunitiesListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := []string{body[0]["id"].(string), body[1]["id"].(string), body[2]["id"].(string), body[3]["id"].(string)}
	want := []string{"OPP-a", "OPP-b", "OPP-c", "OPP-d"}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("want sorted %v, got %v", want, got)
			break
		}
	}
}

// 9. 不以 OPP 开头的 .json → 跳过
func TestListOpportunitiesIgnoresNonOPPPrefix(t *testing.T) {
	crmDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(crmDir, opportunitiesListRelDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeOpportunityJSON(t, crmDir, "OPP-good", `{
		"id":"OPP-good","opportunity_name":"好的","source_type":"新闻搜索","status":"待评估",
		"created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)
	// 几个非 OPP 前缀的 .json：TEMP.json / PRJ-stowaway.json / opp-lowercase.json
	if err := os.WriteFile(filepath.Join(crmDir, opportunitiesListRelDir, "TEMP.json"),
		[]byte(`{"id":"TEMP","opportunity_name":"stowaway","source_type":"新闻搜索","status":"待评估","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, opportunitiesListRelDir, "PRJ-stowaway.json"),
		[]byte(`{"id":"PRJ-stowaway","opportunity_name":"stowaway","source_type":"新闻搜索","status":"待评估","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, opportunitiesListRelDir, "opp-lowercase.json"),
		[]byte(`{"id":"opp-lowercase","opportunity_name":"stowaway","source_type":"新闻搜索","status":"待评估","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	w := doOpportunitiesListGet(t, newOpportunitiesListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("want 1 (only OPP-good), got %d: %v", len(body), body)
	}
	if body[0]["id"] != "OPP-good" {
		t.Errorf("want OPP-good, got %v", body[0]["id"])
	}
}

// 10. 全是非 OPP 前缀 → 200 + []
func TestListOpportunitiesEmptyWhenAllNonOPP(t *testing.T) {
	crmDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(crmDir, opportunitiesListRelDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, opportunitiesListRelDir, "TEMP.json"),
		[]byte(`{"id":"TEMP","opportunity_name":"x","source_type":"新闻搜索","status":"待评估","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, opportunitiesListRelDir, "PRJ-1.json"),
		[]byte(`{"id":"PRJ-1","opportunity_name":"x","source_type":"新闻搜索","status":"待评估","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	w := doOpportunitiesListGet(t, newOpportunitiesListEngine(t, crmDir))
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