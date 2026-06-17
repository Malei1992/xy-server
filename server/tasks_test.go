package main

// 本文件：tasks.go（GET /api/tasks）的端到端测试。
// 全部用 t.TempDir() 隔离，绝不污染真实 data/。
//
// 端点行为：
//   GET /api/tasks → 列出所有代办任务（crm/tasks/*.json），
//   按 customer_id 反查客户档案拿 customer_name 后一起返回。
//
// 行为约定：
//   - 任务目录不存在 / 空 → 200 + []
//   - customer_id 在客户目录里找不到 → 仍返回记录，customer_name 字段空字符串
//   - 损坏的任务文件 → 跳过；损坏的客户文件 → 该条 customer_name 字段空
//   - 文件存在但 customers 目录不存在 → 任务记录仍返回，customer_name 字段空

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
	tasksListRelDir    = "crm/tasks"
	tasksCustomersDir  = "crm/customers"
)

// newTasksListEngine 搭一个最小 router：含 GET /api/tasks。
func newTasksListEngine(t *testing.T, crmDir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.GET("/tasks", listTasksHandlerForTest(crmDir, tasksListRelDir, tasksCustomersDir))
	return r
}

// doTasksListGet 发起 GET /api/tasks 请求。
func doTasksListGet(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/api/tasks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// writeTaskJSON 在 dir/tasks/{id}.json 写一段 JSON。
func writeTaskJSON(t *testing.T, dir, id, content string) {
	t.Helper()
	full := filepath.Join(dir, tasksListRelDir, id+".json")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// writeCustomerJSONForTasks 在 dir/customers/{id}.json 写一段 JSON。
func writeCustomerJSONForTasks(t *testing.T, dir, id, content string) {
	t.Helper()
	full := filepath.Join(dir, tasksCustomersDir, id+".json")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// ===== 基础场景 =====

// 1. 任务目录不存在 → 200 + []
func TestListTasksDirMissing(t *testing.T) {
	crmDir := t.TempDir()
	r := newTasksListEngine(t, crmDir)

	w := doTasksListGet(t, r)
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

// 2. 任务目录为空 → 200 + []
func TestListTasksDirEmpty(t *testing.T) {
	crmDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(crmDir, tasksListRelDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	r := newTasksListEngine(t, crmDir)

	w := doTasksListGet(t, r)
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

// 3. 单个任务 + 匹配的客户 → 字段全部合并
func TestListTasksJoinCustomer(t *testing.T) {
	crmDir := t.TempDir()
	writeTaskJSON(t, crmDir, "TASK-1", `{
		"id":"TASK-1",
		"title":"合规文件缺失",
		"type":"compliance_blocked",
		"priority":"P1",
		"status":"pending",
		"description":"客户缺 BOI 投资促进证明",
		"customer_id":"CUST-1",
		"assigned_to":"张三",
		"created_at":"2026-06-15T10:00:00Z",
		"updated_at":"2026-06-16T10:00:00Z"
	}`)
	writeCustomerJSONForTasks(t, crmDir, "CUST-1", `{
		"id":"CUST-1",
		"basic":{"name":"Siam Cement Group","contacts":"contact@example.com","country":"泰国","industry":"工业"}
	}`)

	w := doTasksListGet(t, newTasksListEngine(t, crmDir))
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
	if p["title"] != "合规文件缺失" {
		t.Errorf("title lost: %v", p["title"])
	}
	if p["type"] != "compliance_blocked" {
		t.Errorf("type lost: %v", p["type"])
	}
	if p["priority"] != "P1" {
		t.Errorf("priority lost: %v", p["priority"])
	}
	if p["status"] != "pending" {
		t.Errorf("status lost: %v", p["status"])
	}
	if p["customer_name"] != "Siam Cement Group" {
		t.Errorf("customer_name not joined: %v", p["customer_name"])
	}
	if p["assigned_to"] != "张三" {
		t.Errorf("assigned_to lost: %v", p["assigned_to"])
	}
}

// 4. customer_id 在客户目录里不存在 → 任务记录仍返回，customer_name 字段空
func TestListTasksMissingCustomer(t *testing.T) {
	crmDir := t.TempDir()
	writeTaskJSON(t, crmDir, "TASK-1", `{
		"id":"TASK-1","title":"x","type":"anomaly_alert","priority":"P0","status":"pending",
		"customer_id":"CUST-ghost",
		"created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)

	w := doTasksListGet(t, newTasksListEngine(t, crmDir))
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

// 5. 损坏的任务文件 → 跳过；损坏的客户文件 → 该条 customer_name 字段空
func TestListTasksSkipsCorrupt(t *testing.T) {
	crmDir := t.TempDir()
	writeTaskJSON(t, crmDir, "TASK-good", `{
		"id":"TASK-good","title":"好","type":"compliance_blocked","priority":"P1","status":"pending",
		"customer_id":"CUST-1",
		"created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)
	writeTaskJSON(t, crmDir, "TASK-bad", `{this is not valid json`)
	writeCustomerJSONForTasks(t, crmDir, "CUST-1", `{"id":"CUST-1","basic":{"name":"N1"}}`)
	writeCustomerJSONForTasks(t, crmDir, "CUST-corrupt", `{not valid`)

	w := doTasksListGet(t, newTasksListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("want 1 (corrupt task skipped), got %d: %v", len(body), body)
	}
	if body[0]["id"] != "TASK-good" {
		t.Errorf("want TASK-good, got %v", body[0]["id"])
	}
	if body[0]["customer_name"] != "N1" {
		t.Errorf("customer_name should still join, got %v", body[0]["customer_name"])
	}
}

// 6. 客户目录不存在 → 任务记录仍返回，customer_name 字段空
func TestListTasksCustomerDirMissing(t *testing.T) {
	crmDir := t.TempDir()
	writeTaskJSON(t, crmDir, "TASK-1", `{
		"id":"TASK-1","title":"x","type":"anomaly_alert","priority":"P0","status":"pending",
		"customer_id":"CUST-1",
		"created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)

	w := doTasksListGet(t, newTasksListEngine(t, crmDir))
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

// 7. 中文 title + description round-trip
func TestListTasksChineseRoundTrip(t *testing.T) {
	crmDir := t.TempDir()
	writeTaskJSON(t, crmDir, "TASK-1", `{
		"id":"TASK-1",
		"title":"合规文件缺失：泰国 BOI",
		"description":"客户缺少投资促进委员会证明，请尽快补充",
		"type":"compliance_blocked",
		"priority":"P1",
		"status":"pending",
		"customer_id":"CUST-1",
		"assigned_to":"张三",
		"resolution":"已补充 BOI 证书，状态恢复",
		"resolved_at":"2026-06-16T11:00:00Z",
		"created_at":"2026-06-15T10:00:00Z",
		"updated_at":"2026-06-16T11:00:00Z"
	}`)
	writeCustomerJSONForTasks(t, crmDir, "CUST-1", `{
		"id":"CUST-1",
		"basic":{"name":"泰国正大集团","contacts":"contact@cp.co.th"}
	}`)

	w := doTasksListGet(t, newTasksListEngine(t, crmDir))
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
	if p["title"] != "合规文件缺失：泰国 BOI" {
		t.Errorf("title lost: %v", p["title"])
	}
	if p["description"] != "客户缺少投资促进委员会证明，请尽快补充" {
		t.Errorf("description lost: %v", p["description"])
	}
	if p["customer_name"] != "泰国正大集团" {
		t.Errorf("customer_name lost: %v", p["customer_name"])
	}
	if p["assigned_to"] != "张三" {
		t.Errorf("assigned_to lost: %v", p["assigned_to"])
	}
	if p["resolution"] != "已补充 BOI 证书，状态恢复" {
		t.Errorf("resolution lost: %v", p["resolution"])
	}
}

// 8. 4 条按 id 升序
func TestListTasksSortedByID(t *testing.T) {
	crmDir := t.TempDir()
	ids := []string{"TASK-c", "TASK-a", "TASK-d", "TASK-b"}
	for _, id := range ids {
		writeTaskJSON(t, crmDir, id, `{
			"id":"`+id+`","title":"`+id+`","type":"compliance_blocked","priority":"P1","status":"pending",
			"created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
		}`)
	}

	w := doTasksListGet(t, newTasksListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := []string{body[0]["id"].(string), body[1]["id"].(string), body[2]["id"].(string), body[3]["id"].(string)}
	want := []string{"TASK-a", "TASK-b", "TASK-c", "TASK-d"}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("want sorted %v, got %v", want, got)
			break
		}
	}
}

// 9. 不以 TASK 开头的 .json → 跳过（哪怕内容是合法 JSON）
func TestListTasksIgnoresNonTASKPrefix(t *testing.T) {
	crmDir := t.TempDir()
	writeTaskJSON(t, crmDir, "TASK-good", `{
		"id":"TASK-good","title":"好","type":"compliance_blocked","priority":"P1","status":"pending",
		"created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)
	// 几个非 TASK 前缀的 .json：TEMP.json / PRJ-stowaway.json / task-lowercase.json
	if err := os.WriteFile(filepath.Join(crmDir, tasksListRelDir, "TEMP.json"),
		[]byte(`{"id":"TEMP","title":"stowaway","type":"compliance_blocked","priority":"P1","status":"pending","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, tasksListRelDir, "PRJ-stowaway.json"),
		[]byte(`{"id":"PRJ-stowaway","title":"stowaway","type":"compliance_blocked","priority":"P1","status":"pending","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, tasksListRelDir, "task-lowercase.json"),
		[]byte(`{"id":"task-lowercase","title":"stowaway","type":"compliance_blocked","priority":"P1","status":"pending","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	w := doTasksListGet(t, newTasksListEngine(t, crmDir))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("want 1 (only TASK-good), got %d: %v", len(body), body)
	}
	if body[0]["id"] != "TASK-good" {
		t.Errorf("want TASK-good, got %v", body[0]["id"])
	}
}

// 10. 全是非 TASK 前缀 → 200 + []
func TestListTasksEmptyWhenAllNonTASK(t *testing.T) {
	crmDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(crmDir, tasksListRelDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, tasksListRelDir, "TEMP.json"),
		[]byte(`{"id":"TEMP","title":"x","type":"compliance_blocked","priority":"P1","status":"pending","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crmDir, tasksListRelDir, "PRJ-1.json"),
		[]byte(`{"id":"PRJ-1","title":"x","type":"compliance_blocked","priority":"P1","status":"pending","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	w := doTasksListGet(t, newTasksListEngine(t, crmDir))
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
