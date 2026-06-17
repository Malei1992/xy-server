package main

// 本文件：projects.go（GET /api/projects）的端到端测试。
// 全部用 t.TempDir() 隔离，绝不污染真实 data/。
//
// 端点行为：
//   GET /api/projects → 列出所有商机，按 customer_id 反查客户，
//   组合项目字段 + 客户名 / 邮箱返回。意向等级来自项目自身的 intent_level 字段。
//
// 行为约定：
//   - 项目目录不存在 / 空 → 200 + []
//   - 项目 customer_id 在客户目录里找不到 → 仍返回记录，但 customer 字段为空字符串
//   - 损坏的项目文件 → 跳过；损坏的客户文件 → 该条 customer 字段空，不影响其他记录
//   - 文件存在但 customers 目录不存在 → 项目记录仍返回，customer 字段空

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

const (
	projectsRelDir = "crm/projects"
	customersRelDir = "crm/customers"
)

// newProjectsEngine 搭一个最小 router：含 GET /api/projects。
func newProjectsEngine(t *testing.T, crmDir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.GET("/projects", getProjectsHandlerForTest(crmDir, projectsRelDir, customersRelDir))
	return r
}

// doProjectsGet 发起 GET /api/projects 请求。
func doProjectsGet(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// writeProjectJSON 在 dir/projects/{id}.json 写一段 JSON。
func writeProjectJSON(t *testing.T, dir, id, content string) {
	t.Helper()
	full := filepath.Join(dir, projectsRelDir, id+".json")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// writeCustomerJSON 在 dir/customers/{id}.json 写一段 JSON。
func writeCustomerJSON(t *testing.T, dir, id, content string) {
	t.Helper()
	full := filepath.Join(dir, customersRelDir, id+".json")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// ===== 基础场景 =====

// 1. 项目目录不存在 → 200 + []
func TestGetProjectsDirMissing(t *testing.T) {
	crmDir := t.TempDir()
	r := newProjectsEngine(t, crmDir)

	w := doProjectsGet(t, r)
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

// 2. 项目目录为空 → 200 + []
func TestGetProjectsDirEmpty(t *testing.T) {
	crmDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(crmDir, projectsRelDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	r := newProjectsEngine(t, crmDir)

	w := doProjectsGet(t, r)
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

// 3. 单个项目 + 匹配的客户 → 字段全部合并
func TestGetProjectsJoinCustomer(t *testing.T) {
	crmDir := t.TempDir()
	writeProjectJSON(t, crmDir, "PRJ-1", `{
		"id": "PRJ-1",
		"project_name": "华为泰国数据中心",
		"customer_id": "CUST-1",
		"status": "谈判中",
		"intent_level": "A",
		"assigned_to": "张三",
		"notes": "客户对价格敏感",
		"created_at": "2026-06-15T10:00:00Z",
		"updated_at": "2026-06-16T10:00:00Z"
	}`)
	writeCustomerJSON(t, crmDir, "CUST-1", `{
		"id": "CUST-1",
		"basic": {
			"name": "Siam Cement Group",
			"contacts": "contact@example.com",
			"country": "泰国",
			"industry": "工业"
		}
	}`)

	w := doProjectsGet(t, newProjectsEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("want 1 project, got %d", len(body))
	}
	p := body[0]
	if p["project_name"] != "华为泰国数据中心" {
		t.Errorf("project_name lost: %v", p["project_name"])
	}
	if p["customer_name"] != "Siam Cement Group" {
		t.Errorf("customer_name not joined: %v", p["customer_name"])
	}
	if p["intent_level"] != "A" {
		t.Errorf("intent_level not from project: %v", p["intent_level"])
	}
	if p["customer_email"] != "contact@example.com" {
		t.Errorf("customer_email not joined: %v", p["customer_email"])
	}
	if p["status"] != "谈判中" {
		t.Errorf("status lost: %v", p["status"])
	}
	if p["assigned_to"] != "张三" {
		t.Errorf("assigned_to lost: %v", p["assigned_to"])
	}
	if p["notes"] != "客户对价格敏感" {
		t.Errorf("notes lost: %v", p["notes"])
	}
	if p["customer_id"] != "CUST-1" {
		t.Errorf("customer_id lost: %v", p["customer_id"])
	}
}

// 3b. 客户侧即使写了 engagement.intent_level，响应里的 intent_level 也来自项目自身
//     （防回归：保证 join 时不读 customer 的意向等级）
func TestGetProjectsIntentLevelFromProjectNotCustomer(t *testing.T) {
	crmDir := t.TempDir()
	writeProjectJSON(t, crmDir, "PRJ-1", `{
		"id": "PRJ-1",
		"project_name": "x",
		"customer_id": "CUST-1",
		"status": "跟进中",
		"intent_level": "B",
		"created_at": "2026-06-16T10:00:00Z",
		"updated_at": "2026-06-16T10:00:00Z"
	}`)
	// 客户里故意写一个不同的 intent_level，期望响应 intent_level 仍是项目的 B
	writeCustomerJSON(t, crmDir, "CUST-1", `{
		"id": "CUST-1",
		"basic": {"name": "N1", "contacts": "e@x"},
		"engagement": {"intent_level": "S"}
	}`)

	w := doProjectsGet(t, newProjectsEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body[0]["intent_level"] != "B" {
		t.Errorf("intent_level should come from project, got %v", body[0]["intent_level"])
	}
}

// 4. customer_id 在客户目录里不存在 → 项目记录仍返回，customer 字段为空字符串
func TestGetProjectsMissingCustomer(t *testing.T) {
	crmDir := t.TempDir()
	writeProjectJSON(t, crmDir, "PRJ-1", `{
		"id": "PRJ-1",
		"project_name": "x",
		"customer_id": "CUST-ghost",
		"status": "跟进中",
		"intent_level": "C",
		"created_at": "2026-06-16T10:00:00Z",
		"updated_at": "2026-06-16T10:00:00Z"
	}`)

	w := doProjectsGet(t, newProjectsEngine(t, crmDir))
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
	if p["customer_name"] != "" {
		t.Errorf("want empty customer_name, got %v", p["customer_name"])
	}
	if p["customer_email"] != "" {
		t.Errorf("want empty customer_email, got %v", p["customer_email"])
	}
	if p["intent_level"] != "C" {
		t.Errorf("intent_level should still be present from project, got %v", p["intent_level"])
	}
}

// 5. 多条项目 → 全部返回，按 id 排序
func TestGetProjectsMultiple(t *testing.T) {
	crmDir := t.TempDir()
	writeProjectJSON(t, crmDir, "PRJ-2", `{
		"id": "PRJ-2",
		"project_name": "p2",
		"customer_id": "CUST-2",
		"status": "签约中",
		"intent_level": "S",
		"created_at": "2026-06-15T10:00:00Z",
		"updated_at": "2026-06-16T10:00:00Z"
	}`)
	writeProjectJSON(t, crmDir, "PRJ-1", `{
		"id": "PRJ-1",
		"project_name": "p1",
		"customer_id": "CUST-1",
		"status": "跟进中",
		"intent_level": "A",
		"created_at": "2026-06-15T10:00:00Z",
		"updated_at": "2026-06-16T10:00:00Z"
	}`)
	writeCustomerJSON(t, crmDir, "CUST-1", `{"id":"CUST-1","basic":{"name":"N1","contacts":"e1"}}`)
	writeCustomerJSON(t, crmDir, "CUST-2", `{"id":"CUST-2","basic":{"name":"N2","contacts":"e2"}}`)

	w := doProjectsGet(t, newProjectsEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("want 2, got %d", len(body))
	}
	// 按 id 升序
	if body[0]["id"] != "PRJ-1" || body[1]["id"] != "PRJ-2" {
		t.Errorf("want sorted by id, got %v %v", body[0]["id"], body[1]["id"])
	}
	if body[0]["customer_name"] != "N1" {
		t.Errorf("PRJ-1 customer_name wrong: %v", body[0]["customer_name"])
	}
	if body[1]["customer_name"] != "N2" {
		t.Errorf("PRJ-2 customer_name wrong: %v", body[1]["customer_name"])
	}
	if body[0]["intent_level"] != "A" {
		t.Errorf("PRJ-1 intent_level wrong: %v", body[0]["intent_level"])
	}
	if body[1]["intent_level"] != "S" {
		t.Errorf("PRJ-2 intent_level wrong: %v", body[1]["intent_level"])
	}
}

// 6. 损坏的项目文件 → 跳过；损坏的客户文件 → 该条 customer 字段空
func TestGetProjectsSkipsCorrupt(t *testing.T) {
	crmDir := t.TempDir()
	writeProjectJSON(t, crmDir, "PRJ-good", `{
		"id":"PRJ-good","project_name":"好","customer_id":"CUST-1","status":"跟进中",
		"intent_level":"B",
		"created_at":"2026-06-16T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)
	writeProjectJSON(t, crmDir, "PRJ-bad", `{this is not valid json`)
	writeCustomerJSON(t, crmDir, "CUST-1", `{"id":"CUST-1","basic":{"name":"N1","contacts":"e@x"}}`)
	writeCustomerJSON(t, crmDir, "CUST-corrupt", `{not valid`)

	w := doProjectsGet(t, newProjectsEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("want 1 (corrupt project skipped), got %d: %v", len(body), body)
	}
	if body[0]["id"] != "PRJ-good" {
		t.Errorf("want PRJ-good, got %v", body[0]["id"])
	}
	if body[0]["customer_name"] != "N1" {
		t.Errorf("customer_name should still join, got %v", body[0]["customer_name"])
	}
	if body[0]["intent_level"] != "B" {
		t.Errorf("intent_level should be B, got %v", body[0]["intent_level"])
	}
}

// 7. 中文项目 + 客户 round-trip
func TestGetProjectsChineseRoundTrip(t *testing.T) {
	crmDir := t.TempDir()
	writeProjectJSON(t, crmDir, "PRJ-1", `{
		"id":"PRJ-1",
		"project_name":"华为泰国数据中心项目",
		"customer_id":"CUST-1",
		"status":"谈判中",
		"intent_level":"A",
		"assigned_to":"张三",
		"notes":"客户对价格敏感，需要进一步沟通",
		"created_at":"2026-06-15T10:00:00Z",
		"updated_at":"2026-06-16T10:00:00Z"
	}`)
	writeCustomerJSON(t, crmDir, "CUST-1", `{
		"id":"CUST-1",
		"basic":{"name":"泰国正大集团","contacts":"contact@cp.co.th","country":"泰国","industry":"综合"}
	}`)

	w := doProjectsGet(t, newProjectsEngine(t, crmDir))
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
	if p["project_name"] != "华为泰国数据中心项目" {
		t.Errorf("project_name lost: %v", p["project_name"])
	}
	if p["customer_name"] != "泰国正大集团" {
		t.Errorf("customer_name lost: %v", p["customer_name"])
	}
	if p["intent_level"] != "A" {
		t.Errorf("intent_level lost: %v", p["intent_level"])
	}
	if p["customer_email"] != "contact@cp.co.th" {
		t.Errorf("customer_email lost: %v", p["customer_email"])
	}
}

// 8. 客户目录不存在 → 项目记录仍返回，customer 字段空
func TestGetProjectsCustomerDirMissing(t *testing.T) {
	crmDir := t.TempDir()
	writeProjectJSON(t, crmDir, "PRJ-1", `{
		"id":"PRJ-1","project_name":"x","customer_id":"CUST-1","status":"跟进中",
		"intent_level":"C",
		"created_at":"2026-06-16T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)

	w := doProjectsGet(t, newProjectsEngine(t, crmDir))
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
	if p["customer_name"] != "" || p["customer_email"] != "" {
		t.Errorf("customer fields should be empty when customer dir missing, got %v", p)
	}
	if p["intent_level"] != "C" {
		t.Errorf("intent_level should come from project, got %v", p["intent_level"])
	}
}

// 9. 验证排序稳定：4 条项目按 id 字母升序
func TestGetProjectsSortedByID(t *testing.T) {
	crmDir := t.TempDir()
	ids := []string{"PRJ-c", "PRJ-a", "PRJ-d", "PRJ-b"}
	for _, id := range ids {
		writeProjectJSON(t, crmDir, id, `{
			"id":"`+id+`","project_name":"`+id+`","customer_id":"CUST-1","status":"跟进中",
			"intent_level":"A",
			"created_at":"2026-06-16T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
		}`)
	}

	w := doProjectsGet(t, newProjectsEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	gotIDs := []string{body[0]["id"].(string), body[1]["id"].(string), body[2]["id"].(string), body[3]["id"].(string)}
	wantIDs := []string{"PRJ-a", "PRJ-b", "PRJ-c", "PRJ-d"}
	if !equalStringSlices(gotIDs, wantIDs) {
		t.Errorf("want sorted %v, got %v", wantIDs, gotIDs)
	}
}

func equalStringSlices(a, b []string) bool {
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

// 10. 客户缺 basic → 不 panic，customer 字段空
func TestGetProjectsMalformedCustomer(t *testing.T) {
	crmDir := t.TempDir()
	writeProjectJSON(t, crmDir, "PRJ-1", `{
		"id":"PRJ-1","project_name":"x","customer_id":"CUST-1","status":"跟进中",
		"intent_level":"A",
		"created_at":"2026-06-16T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)
	// 客户只有 id，没有 basic
	writeCustomerJSON(t, crmDir, "CUST-1", `{"id":"CUST-1"}`)

	w := doProjectsGet(t, newProjectsEngine(t, crmDir))
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
	if p["customer_name"] != "" {
		t.Errorf("customer_name should be empty, got %v", p["customer_name"])
	}
	if p["customer_email"] != "" {
		t.Errorf("customer_email should be empty, got %v", p["customer_email"])
	}
	if p["intent_level"] != "A" {
		t.Errorf("intent_level should come from project, got %v", p["intent_level"])
	}
	// sanity: 不要触发 sort 包未使用告警
	_ = sort.Strings
}

// 9. 不以 PRJ 开头的 .json → 跳过（哪怕内容是合法 JSON）
func TestGetProjectsIgnoresNonPRJPrefix(t *testing.T) {
	crmDir := t.TempDir()
	writeProjectJSON(t, crmDir, "PRJ-good", `{
		"id":"PRJ-good","project_name":"好的","customer_id":"CUST-1","status":"跟进中",
		"updated_at":"2026-06-16T10:00:00Z"
	}`)
	// 几个非 PRJ 前缀的 .json：TEMP.json / TASK-stowaway.json / prj-lowercase.json
	if err := os.WriteFile(filepath.Join(crmDir, projectsRelDir, "TEMP.json"),
		[]byte(`{"id":"TEMP","project_name":"stowaway","customer_id":"CUST-1","status":"跟进中","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, projectsRelDir, "TASK-stowaway.json"),
		[]byte(`{"id":"TASK-stowaway","project_name":"stowaway","customer_id":"CUST-1","status":"跟进中","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, projectsRelDir, "prj-lowercase.json"),
		[]byte(`{"id":"prj-lowercase","project_name":"stowaway","customer_id":"CUST-1","status":"跟进中","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	w := doProjectsGet(t, newProjectsEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("want 1 (only PRJ-good), got %d: %v", len(body), body)
	}
	if body[0]["id"] != "PRJ-good" {
		t.Errorf("want PRJ-good, got %v", body[0]["id"])
	}
}

// 10. 全是非 PRJ 前缀 → 200 + []
func TestGetProjectsEmptyWhenAllNonPRJ(t *testing.T) {
	crmDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(crmDir, projectsRelDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, projectsRelDir, "TEMP.json"),
		[]byte(`{"id":"TEMP","project_name":"x","customer_id":"CUST-1","status":"跟进中","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, projectsRelDir, "TASK-1.json"),
		[]byte(`{"id":"TASK-1","project_name":"x","customer_id":"CUST-1","status":"跟进中","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	w := doProjectsGet(t, newProjectsEngine(t, crmDir))
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
