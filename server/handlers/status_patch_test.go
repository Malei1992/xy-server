package handlers

// 本文件：PATCH /api/{projects,tasks,opportunities}/:id/status 的端点测试。
// 覆盖：happy path / 状态枚举校验 / ID 前缀校验 / 不存在 ID / 损坏 JSON 文件 / 非法 JSON body / 并发。
//
// 测试夹具：
//   - crmDir 是 t.TempDir() 临时目录
//   - 在 crmDir/{projects,tasks,opportunities} 下预创建 JSON 文件
//   - gin 路由直接挂 PatchXxxStatus，绕过 main.go 路由注册

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ===== 夹具 helper =====

// 跟 main.go paths.go 里 ProjectsRelDir / TasksRelDir / OpportunitiesRelDir 同值。
// 复制到这里是因为它们在 package main,handlers 包拿不到。
const (
	testProjectsRelDir      = "crm/projects"
	testTasksRelDir         = "crm/tasks"
	testOpportunitiesRelDir = "crm/opportunities"
)

// newStatusPatchRouter 注册 3 个 PATCH 路由到 crmDir。
// crm/projects / crm/tasks / crm/opportunities 都建空目录。
func newStatusPatchRouter(t *testing.T, crmDir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.PATCH("/projects/:id/status", PatchProjectStatus(crmDir, testProjectsRelDir))
	api.PATCH("/tasks/:id/status", PatchTaskStatus(crmDir, testTasksRelDir))
	api.PATCH("/opportunities/:id/status", PatchOpportunityStatus(crmDir, testOpportunitiesRelDir))
	return r
}

// seedProject 写一个 project JSON 到 crm/projects/{id}.json,返回写入的初始 updated_at(测试启动前 1 小时)。
// PATCH 后 handler 会刷新 updated_at 到 now() — 一定晚于 seedTime,断言时直接对比。
func seedProject(t *testing.T, crmDir, id string, status string) string {
	t.Helper()
	dir := filepath.Join(crmDir, testProjectsRelDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	seedTime := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	p := Project{
		ID:        id,
		Status:    status,
		UpdatedAt: seedTime,
	}
	if err := WriteProject(dir, p); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return seedTime
}

// seedTask 写一个 task JSON 到 crm/tasks/{id}.json,返回写入的初始 updated_at。
func seedTask(t *testing.T, crmDir, id string, status string) string {
	t.Helper()
	dir := filepath.Join(crmDir, testTasksRelDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	seedTime := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	tk := Task{
		ID:        id,
		Type:      "compliance_blocked",
		Priority:  "P1",
		Status:    status,
		UpdatedAt: seedTime,
	}
	if err := WriteTask(dir, tk); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return seedTime
}

// seedOpportunity 写一个 opportunity JSON 到 crm/opportunities/{id}.json,返回写入的初始 updated_at。
func seedOpportunity(t *testing.T, crmDir, id string, status string) string {
	t.Helper()
	dir := filepath.Join(crmDir, testOpportunitiesRelDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	seedTime := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	o := Opportunity{
		ID:         id,
		SourceType: "新闻搜索",
		Status:     status,
		UpdatedAt:  seedTime,
	}
	if err := WriteOpportunity(dir, o); err != nil {
		t.Fatalf("seed opportunity: %v", err)
	}
	return seedTime
}

// doPatch 发 PATCH 请求,body 是 map 或 raw bytes(传 nil → 不带 body)。
func doPatch(t *testing.T, r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		switch v := body.(type) {
		case []byte:
			rdr = bytes.NewReader(v)
		default:
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal body: %v", err)
			}
			rdr = bytes.NewReader(b)
		}
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(http.MethodPatch, path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// readProjectFile 读 crm/projects/{id}.json → Project。
func readProjectFile(t *testing.T, crmDir, id string) Project {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(crmDir, testProjectsRelDir, id+".json"))
	if err != nil {
		t.Fatalf("read project: %v", err)
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("parse project: %v; raw=%s", err, data)
	}
	return p
}

func readTaskFile(t *testing.T, crmDir, id string) Task {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(crmDir, testTasksRelDir, id+".json"))
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	var tk Task
	if err := json.Unmarshal(data, &tk); err != nil {
		t.Fatalf("parse task: %v; raw=%s", err, data)
	}
	return tk
}

func readOpportunityFile(t *testing.T, crmDir, id string) Opportunity {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(crmDir, testOpportunitiesRelDir, id+".json"))
	if err != nil {
		t.Fatalf("read opportunity: %v", err)
	}
	var o Opportunity
	if err := json.Unmarshal(data, &o); err != nil {
		t.Fatalf("parse opportunity: %v; raw=%s", err, data)
	}
	return o
}

// ===== PatchProjectStatus =====

// happy path: 改 status + 写回 + updated_at 变化
func TestPatchProjectStatus_Success(t *testing.T) {
	crmDir := t.TempDir()
	seedTime := seedProject(t, crmDir, "PRJ-1", "跟进中")
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/projects/PRJ-1/status", map[string]string{"status": "谈判中"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d; body=%s", w.Code, w.Body.String())
	}

	// 文件里 status 已变,updated_at 是新时间戳(一定晚于 seedTime)
	p := readProjectFile(t, crmDir, "PRJ-1")
	if p.Status != "谈判中" {
		t.Errorf("status = %q, want 谈判中", p.Status)
	}
	oldTime, _ := time.Parse(time.RFC3339, seedTime)
	newTime, err := time.Parse(time.RFC3339, p.UpdatedAt)
	if err != nil {
		t.Fatalf("parse updated_at: %v; raw=%q", err, p.UpdatedAt)
	}
	if !newTime.After(oldTime) {
		t.Errorf("updated_at = %q, should be after seedTime %q", p.UpdatedAt, seedTime)
	}

	// response body 检查
	body := decodeJSON(t, w)
	if body["status"] != "谈判中" {
		t.Errorf("body.status = %v, want 谈判中", body["status"])
	}
	if body["ok"] != true {
		t.Errorf("body.ok = %v, want true", body["ok"])
	}
}

// status 不在枚举内 → 400
func TestPatchProjectStatus_InvalidStatus(t *testing.T) {
	crmDir := t.TempDir()
	seedProject(t, crmDir, "PRJ-1", "跟进中")
	r := newStatusPatchRouter(t, crmDir)

	cases := []string{"", "已完成", "已暂停", "P0", "跟踪中"} // 5 个非法值
	for _, s := range cases {
		t.Run(fmt.Sprintf("status=%q", s), func(t *testing.T) {
			w := doPatch(t, r, "/api/projects/PRJ-1/status", map[string]string{"status": s})
			if w.Code != http.StatusBadRequest {
				t.Errorf("status=%q want 400, got %d; body=%s", s, w.Code, w.Body.String())
			}
		})
	}

	// 文件不应被破坏
	p := readProjectFile(t, crmDir, "PRJ-1")
	if p.Status != "跟进中" {
		t.Errorf("status = %q, want 跟进中(不应被非法 PATCH 覆盖)", p.Status)
	}
}

// ID 不以 PRJ 开头 → 400
func TestPatchProjectStatus_InvalidIDPrefix(t *testing.T) {
	crmDir := t.TempDir()
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/projects/foo/status", map[string]string{"status": "谈判中"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for non-PRJ id, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 不存在的 id(前缀正确)→ 404
func TestPatchProjectStatus_NotFound(t *testing.T) {
	crmDir := t.TempDir()
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/projects/PRJ-ghost/status", map[string]string{"status": "谈判中"})
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404 for missing file, got %d; body=%s", w.Code, w.Body.String())
	}
}

// JSON body 非法 → 400
func TestPatchProjectStatus_InvalidJSONBody(t *testing.T) {
	crmDir := t.TempDir()
	seedProject(t, crmDir, "PRJ-1", "跟进中")
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/projects/PRJ-1/status", []byte(`{not valid json`))
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for bad json, got %d; body=%s", w.Code, w.Body.String())
	}

	// 文件不应被破坏
	p := readProjectFile(t, crmDir, "PRJ-1")
	if p.Status != "跟进中" {
		t.Errorf("status = %q, want 跟进中", p.Status)
	}
}

// 文件损坏(不是合法 JSON)→ 404(沿用 ReadProjects 跳过策略 → handler 找不到目标)
func TestPatchProjectStatus_CorruptFile(t *testing.T) {
	crmDir := t.TempDir()
	dir := filepath.Join(crmDir, testProjectsRelDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// 直接写一个损坏 JSON,绕过 WriteProject(它会 marshal 失败)
	if err := os.WriteFile(filepath.Join(dir, "PRJ-corrupt.json"), []byte(`{this is not valid json`), 0o644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/projects/PRJ-corrupt/status", map[string]string{"status": "谈判中"})
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404 for corrupt file, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 8 goroutine 同时 PATCH 同一 id,无 panic / 损坏,文件最终 status 是合法枚举之一
func TestPatchProjectStatus_Concurrent(t *testing.T) {
	crmDir := t.TempDir()
	seedProject(t, crmDir, "PRJ-shared", "跟进中")
	r := newStatusPatchRouter(t, crmDir)

	const goroutines = 8
	statuses := []string{"跟进中", "谈判中", "签约中", "已落地", "已关闭"}

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := statuses[g%len(statuses)]
			w := doPatch(t, r, "/api/projects/PRJ-shared/status", map[string]string{"status": s})
			if w.Code != http.StatusOK {
				t.Errorf("g=%d status=%q: code=%d body=%s", g, s, w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()

	// 文件应是合法 JSON 且 status 落在枚举内
	p := readProjectFile(t, crmDir, "PRJ-shared")
	valid := false
	for _, s := range statuses {
		if p.Status == s {
			valid = true
			break
		}
	}
	if !valid {
		t.Errorf("final status = %q, want one of %v", p.Status, statuses)
	}
}

// ===== PatchTaskStatus =====

// happy path
func TestPatchTaskStatus_Success(t *testing.T) {
	crmDir := t.TempDir()
	seedTime := seedTask(t, crmDir, "TASK-1", "pending")
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/tasks/TASK-1/status", map[string]string{"status": "in_progress"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d; body=%s", w.Code, w.Body.String())
	}

	tk := readTaskFile(t, crmDir, "TASK-1")
	if tk.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress", tk.Status)
	}
	oldTime, _ := time.Parse(time.RFC3339, seedTime)
	newTime, _ := time.Parse(time.RFC3339, tk.UpdatedAt)
	if !newTime.After(oldTime) {
		t.Errorf("updated_at = %q should be after seedTime %q", tk.UpdatedAt, seedTime)
	}
}

// status 不在英文枚举内 → 400(中文值不允许,英文值只允许 4 个)
func TestPatchTaskStatus_InvalidStatus(t *testing.T) {
	crmDir := t.TempDir()
	seedTask(t, crmDir, "TASK-1", "pending")
	r := newStatusPatchRouter(t, crmDir)

	cases := []string{"", "done", "跟进中", "已解决", "PENDING"} // 5 个非法值
	for _, s := range cases {
		t.Run(fmt.Sprintf("status=%q", s), func(t *testing.T) {
			w := doPatch(t, r, "/api/tasks/TASK-1/status", map[string]string{"status": s})
			if w.Code != http.StatusBadRequest {
				t.Errorf("status=%q want 400, got %d; body=%s", s, w.Code, w.Body.String())
			}
		})
	}

	tk := readTaskFile(t, crmDir, "TASK-1")
	if tk.Status != "pending" {
		t.Errorf("status = %q, want pending(不应被非法 PATCH 覆盖)", tk.Status)
	}
}

// ID 不以 TASK 开头 → 400
func TestPatchTaskStatus_InvalidIDPrefix(t *testing.T) {
	crmDir := t.TempDir()
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/tasks/foo/status", map[string]string{"status": "resolved"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for non-TASK id, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 不存在 id → 404
func TestPatchTaskStatus_NotFound(t *testing.T) {
	crmDir := t.TempDir()
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/tasks/TASK-ghost/status", map[string]string{"status": "resolved"})
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d; body=%s", w.Code, w.Body.String())
	}
}

// JSON body 非法 → 400
func TestPatchTaskStatus_InvalidJSONBody(t *testing.T) {
	crmDir := t.TempDir()
	seedTask(t, crmDir, "TASK-1", "pending")
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/tasks/TASK-1/status", []byte(`{not json`))
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 文件损坏 → 404
func TestPatchTaskStatus_CorruptFile(t *testing.T) {
	crmDir := t.TempDir()
	dir := filepath.Join(crmDir, testTasksRelDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "TASK-corrupt.json"), []byte(`{not valid`), 0o644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/tasks/TASK-corrupt/status", map[string]string{"status": "resolved"})
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404 for corrupt file, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 8 goroutine 并发
func TestPatchTaskStatus_Concurrent(t *testing.T) {
	crmDir := t.TempDir()
	seedTask(t, crmDir, "TASK-shared", "pending")
	r := newStatusPatchRouter(t, crmDir)

	const goroutines = 8
	statuses := []string{"pending", "in_progress", "resolved", "dismissed"}

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := statuses[g%len(statuses)]
			w := doPatch(t, r, "/api/tasks/TASK-shared/status", map[string]string{"status": s})
			if w.Code != http.StatusOK {
				t.Errorf("g=%d status=%q: code=%d body=%s", g, s, w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()

	tk := readTaskFile(t, crmDir, "TASK-shared")
	valid := false
	for _, s := range statuses {
		if tk.Status == s {
			valid = true
			break
		}
	}
	if !valid {
		t.Errorf("final status = %q, want one of %v", tk.Status, statuses)
	}
}

// ===== PatchOpportunityStatus =====

// happy path
func TestPatchOpportunityStatus_Success(t *testing.T) {
	crmDir := t.TempDir()
	seedTime := seedOpportunity(t, crmDir, "OPP-1", "待评估")
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/opportunities/OPP-1/status", map[string]string{"status": "跟进中"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d; body=%s", w.Code, w.Body.String())
	}

	o := readOpportunityFile(t, crmDir, "OPP-1")
	if o.Status != "跟进中" {
		t.Errorf("status = %q, want 跟进中", o.Status)
	}
	oldTime, _ := time.Parse(time.RFC3339, seedTime)
	newTime, _ := time.Parse(time.RFC3339, o.UpdatedAt)
	if !newTime.After(oldTime) {
		t.Errorf("updated_at = %q should be after seedTime %q", o.UpdatedAt, seedTime)
	}
}

// status 不在中文枚举内 → 400
func TestPatchOpportunityStatus_InvalidStatus(t *testing.T) {
	crmDir := t.TempDir()
	seedOpportunity(t, crmDir, "OPP-1", "待评估")
	r := newStatusPatchRouter(t, crmDir)

	cases := []string{"", "已签约", "pending", "P0", "评僧中"} // 5 个非法值
	for _, s := range cases {
		t.Run(fmt.Sprintf("status=%q", s), func(t *testing.T) {
			w := doPatch(t, r, "/api/opportunities/OPP-1/status", map[string]string{"status": s})
			if w.Code != http.StatusBadRequest {
				t.Errorf("status=%q want 400, got %d; body=%s", s, w.Code, w.Body.String())
			}
		})
	}

	o := readOpportunityFile(t, crmDir, "OPP-1")
	if o.Status != "待评估" {
		t.Errorf("status = %q, want 待评估(不应被非法 PATCH 覆盖)", o.Status)
	}
}

// ID 不以 OPP 开头 → 400
func TestPatchOpportunityStatus_InvalidIDPrefix(t *testing.T) {
	crmDir := t.TempDir()
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/opportunities/foo/status", map[string]string{"status": "跟进中"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for non-OPP id, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 不存在 id → 404
func TestPatchOpportunityStatus_NotFound(t *testing.T) {
	crmDir := t.TempDir()
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/opportunities/OPP-ghost/status", map[string]string{"status": "跟进中"})
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d; body=%s", w.Code, w.Body.String())
	}
}

// JSON body 非法 → 400
func TestPatchOpportunityStatus_InvalidJSONBody(t *testing.T) {
	crmDir := t.TempDir()
	seedOpportunity(t, crmDir, "OPP-1", "待评估")
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/opportunities/OPP-1/status", []byte(`{not json`))
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 文件损坏 → 404
func TestPatchOpportunityStatus_CorruptFile(t *testing.T) {
	crmDir := t.TempDir()
	dir := filepath.Join(crmDir, testOpportunitiesRelDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "OPP-corrupt.json"), []byte(`{not valid`), 0o644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	r := newStatusPatchRouter(t, crmDir)

	w := doPatch(t, r, "/api/opportunities/OPP-corrupt/status", map[string]string{"status": "跟进中"})
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404 for corrupt file, got %d; body=%s", w.Code, w.Body.String())
	}
}

// 8 goroutine 并发
func TestPatchOpportunityStatus_Concurrent(t *testing.T) {
	crmDir := t.TempDir()
	seedOpportunity(t, crmDir, "OPP-shared", "待评估")
	r := newStatusPatchRouter(t, crmDir)

	const goroutines = 8
	statuses := []string{"待评估", "跟进中", "已转化", "已关闭"}

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := statuses[g%len(statuses)]
			w := doPatch(t, r, "/api/opportunities/OPP-shared/status", map[string]string{"status": s})
			if w.Code != http.StatusOK {
				t.Errorf("g=%d status=%q: code=%d body=%s", g, s, w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()

	o := readOpportunityFile(t, crmDir, "OPP-shared")
	valid := false
	for _, s := range statuses {
		if o.Status == s {
			valid = true
			break
		}
	}
	if !valid {
		t.Errorf("final status = %q, want one of %v", o.Status, statuses)
	}
}
