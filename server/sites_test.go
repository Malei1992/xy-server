package main

// 本文件：sites.go 4 个端点（GET / POST / PATCH / DELETE）的端到端测试。
// 全部用 t.TempDir() 隔离，绝不污染真实 data/。
//
// 4 个端点：
//   GET    /api/target-sites[?q=xxx]   列出全部或模糊匹配
//   POST   /api/target-sites            body {name, url, country?, industry?, type?}
//                                       name+url 必填；name 唯一
//   PATCH  /api/target-sites?name=xxx   body 部分字段（除 name 外都可改）
//   DELETE /api/target-sites?name=xxx   按 name 精确删除

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

const (
	sitesRelPath = "target_sites.json"
)

// newSitesEngine 搭一个最小 router：含 target-sites 4 个端点。
func newSitesEngine(t *testing.T, crmDir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.GET("/target-sites", getSitesHandlerForTest(crmDir, sitesRelPath))
	api.POST("/target-sites", postSiteHandlerForTest(crmDir, sitesRelPath))
	api.PATCH("/target-sites", patchSiteHandlerForTest(crmDir, sitesRelPath))
	api.DELETE("/target-sites", deleteSiteHandlerForTest(crmDir, sitesRelPath))
	return r
}

// doSitesGet 发起 GET 请求（path 可含 query string）。
func doSitesGet(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// doSitesPost 发起 POST 请求，body 为 JSON 字符串。
func doSitesPost(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, "/api/target-sites", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// doSitesPatch 发起 PATCH 请求，query 是 name=x，body 是 JSON。
func doSitesPatch(t *testing.T, r *gin.Engine, name, body string) *httptest.ResponseRecorder {
	t.Helper()
	u := "/api/target-sites?name=" + url.QueryEscape(name)
	req, _ := http.NewRequest(http.MethodPatch, u, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// doSitesDelete 发起 DELETE 请求，query 是 name=x。
func doSitesDelete(t *testing.T, r *gin.Engine, name string) *httptest.ResponseRecorder {
	t.Helper()
	u := "/api/target-sites?name=" + url.QueryEscape(name)
	req, _ := http.NewRequest(http.MethodDelete, u, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// writeSitesJSON 在 dir/rel 路径下写一个 JSON fixture。
func writeSitesJSON(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// readSitesOnDisk 读 dir/rel 路径下的 JSON，返回 []TargetSite。
func readSitesOnDisk(t *testing.T, dir, rel string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	var out []map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

// ===== GET =====

// 1. GET 文件不存在 → 200 + []
func TestGetSitesFileMissing(t *testing.T) {
	crmDir := t.TempDir()
	r := newSitesEngine(t, crmDir)

	w := doSitesGet(t, r, "/api/target-sites")
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

// 2. GET 文件存在 → 200 + 全部记录
func TestGetSitesFileExists(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[
		{"name": "SET上市公司名录", "url": "https://set", "country": "泰国", "industry": "综合", "type": "download"},
		{"name": "BOI", "url": "https://boi"}
	]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesGet(t, r, "/api/target-sites")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 2 {
		t.Errorf("want 2 sites, got %d", len(body))
	}
}

// 3. GET?q= 模糊匹配（中文）
func TestGetSitesFuzzyChinese(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[
		{"name": "SET上市公司名录", "url": "https://set"},
		{"name": "BOI 投资促进局", "url": "https://boi"},
		{"name": "Tokyo Stock Exchange", "url": "https://tse"}
	]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesGet(t, r, "/api/target-sites?q="+url.QueryEscape("上市公司"))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 1 {
		t.Errorf("want 1 match, got %d: %v", len(body), body)
	}
	if len(body) == 1 && body[0]["name"] != "SET上市公司名录" {
		t.Errorf("want SET上市公司名录, got %v", body[0]["name"])
	}
}

// 4. GET?q= 模糊匹配（英文 + 多条匹配）
func TestGetSitesFuzzyEnglishMultiMatch(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[
		{"name": "SET-TH", "url": "https://set-th"},
		{"name": "SET-JP", "url": "https://set-jp"},
		{"name": "BOI", "url": "https://boi"}
	]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesGet(t, r, "/api/target-sites?q=SET")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 2 {
		t.Errorf("want 2 SET matches, got %d", len(body))
	}
}

// 5. GET?q= 无匹配 → 200 + []
func TestGetSitesFuzzyNoMatch(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[{"name": "alpha", "url": "https://a"}]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesGet(t, r, "/api/target-sites?q=zzzzz")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("want empty, got %v", body)
	}
}

// ===== POST =====

// 6. POST 合法 → 200 + 落盘
func TestPostSiteHappy(t *testing.T) {
	crmDir := t.TempDir()
	r := newSitesEngine(t, crmDir)

	body := `{"name": "SET上市公司名录", "url": "https://set", "country": "泰国", "industry": "综合", "type": "download"}`
	w := doSitesPost(t, r, body)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	// 落盘验证
	onDisk := readSitesOnDisk(t, crmDir, sitesRelPath)
	if len(onDisk) != 1 {
		t.Fatalf("want 1 site on disk, got %d", len(onDisk))
	}
	if onDisk[0]["name"] != "SET上市公司名录" {
		t.Errorf("want name=SET上市公司名录, got %v", onDisk[0]["name"])
	}
	if onDisk[0]["url"] != "https://set" {
		t.Errorf("want url=https://set, got %v", onDisk[0]["url"])
	}
	if onDisk[0]["country"] != "泰国" {
		t.Errorf("want country=泰国, got %v", onDisk[0]["country"])
	}
}

// 7. POST 缺 name → 400
func TestPostSiteMissingName(t *testing.T) {
	crmDir := t.TempDir()
	r := newSitesEngine(t, crmDir)

	body := `{"url": "https://set"}`
	w := doSitesPost(t, r, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "name") {
		t.Errorf("want error to mention 'name', got %s", w.Body.String())
	}
}

// 8. POST 缺 url → 400
func TestPostSiteMissingURL(t *testing.T) {
	crmDir := t.TempDir()
	r := newSitesEngine(t, crmDir)

	body := `{"name": "foo"}`
	w := doSitesPost(t, r, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "url") {
		t.Errorf("want error to mention 'url', got %s", w.Body.String())
	}
}

// 9. POST 重名 → 400
func TestPostSiteDuplicateName(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[{"name": "SET", "url": "https://set"}]`)
	r := newSitesEngine(t, crmDir)

	body := `{"name": "SET", "url": "https://set-2"}`
	w := doSitesPost(t, r, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "exists") && !strings.Contains(w.Body.String(), "duplicate") && !strings.Contains(w.Body.String(), "已存在") {
		t.Errorf("want error to mention 'exists/duplicate/已存在', got %s", w.Body.String())
	}
}

// 10. POST 非法 JSON → 400
func TestPostSiteInvalidJSON(t *testing.T) {
	crmDir := t.TempDir()
	r := newSitesEngine(t, crmDir)

	w := doSitesPost(t, r, `{not valid json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

// 11. POST 不带 country/industry/type → 200（可选项）
func TestPostSiteOptionalFieldsOmitted(t *testing.T) {
	crmDir := t.TempDir()
	r := newSitesEngine(t, crmDir)

	body := `{"name": "minimal", "url": "https://m"}`
	w := doSitesPost(t, r, body)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	onDisk := readSitesOnDisk(t, crmDir, sitesRelPath)
	if len(onDisk) != 1 {
		t.Fatalf("want 1, got %d", len(onDisk))
	}
	if _, has := onDisk[0]["country"]; has {
		t.Errorf("country should be omitted (empty string not serialized), got %v", onDisk[0])
	}
}

// 12. POST 第二个不同名 → 200 + 文件包含 2 条
func TestPostSiteAppends(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[{"name": "first", "url": "https://1"}]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesPost(t, r, `{"name": "second", "url": "https://2"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	onDisk := readSitesOnDisk(t, crmDir, sitesRelPath)
	if len(onDisk) != 2 {
		t.Errorf("want 2 sites, got %d", len(onDisk))
	}
}

// ===== PATCH =====

// 13. PATCH 合法 → 200 + 落盘修改成功
func TestPatchSiteHappy(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[
		{"name": "SET", "url": "https://old", "country": "泰国"}
	]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesPatch(t, r, "SET", `{"url": "https://new", "country": "日本"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	onDisk := readSitesOnDisk(t, crmDir, sitesRelPath)
	if len(onDisk) != 1 {
		t.Fatalf("want 1, got %d", len(onDisk))
	}
	if onDisk[0]["name"] != "SET" {
		t.Errorf("name should not change, got %v", onDisk[0]["name"])
	}
	if onDisk[0]["url"] != "https://new" {
		t.Errorf("want url=https://new, got %v", onDisk[0]["url"])
	}
	if onDisk[0]["country"] != "日本" {
		t.Errorf("want country=日本, got %v", onDisk[0]["country"])
	}
}

// 14. PATCH 不存在 name → 400
func TestPatchSiteNotFound(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[{"name": "SET", "url": "https://set"}]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesPatch(t, r, "nonexistent", `{"url": "https://new"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not found") && !strings.Contains(w.Body.String(), "不存在") {
		t.Errorf("want error to mention not found, got %s", w.Body.String())
	}
}

// 15. PATCH 缺 name query → 400
func TestPatchSiteMissingNameQuery(t *testing.T) {
	crmDir := t.TempDir()
	r := newSitesEngine(t, crmDir)

	req, _ := http.NewRequest(http.MethodPatch, "/api/target-sites", strings.NewReader(`{"url": "x"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "name") {
		t.Errorf("want error to mention 'name', got %s", w.Body.String())
	}
}

// 16. PATCH 改 name → 400（不允许改名，因为 name 是 identifier）
func TestPatchSiteRejectsNameChange(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[{"name": "SET", "url": "https://set"}]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesPatch(t, r, "SET", `{"name": "NEW"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "name") {
		t.Errorf("want error to mention 'name', got %s", w.Body.String())
	}
}

// 17. PATCH 空 body → 400
func TestPatchSiteEmptyBody(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[{"name": "SET", "url": "https://set"}]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesPatch(t, r, "SET", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

// 18. PATCH 部分字段（只改 url）→ 其他字段不变
func TestPatchSitePartialFields(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[
		{"name": "SET", "url": "https://old", "country": "泰国", "industry": "综合", "type": "download"}
	]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesPatch(t, r, "SET", `{"url": "https://new"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	onDisk := readSitesOnDisk(t, crmDir, sitesRelPath)
	if onDisk[0]["country"] != "泰国" || onDisk[0]["industry"] != "综合" || onDisk[0]["type"] != "download" {
		t.Errorf("other fields should be preserved, got %v", onDisk[0])
	}
	if onDisk[0]["url"] != "https://new" {
		t.Errorf("url should be updated, got %v", onDisk[0]["url"])
	}
}

// ===== DELETE =====

// 19. DELETE 合法 → 200 + 落盘删除
func TestDeleteSiteHappy(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[
		{"name": "SET", "url": "https://set"},
		{"name": "BOI", "url": "https://boi"}
	]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesDelete(t, r, "SET")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	onDisk := readSitesOnDisk(t, crmDir, sitesRelPath)
	if len(onDisk) != 1 {
		t.Fatalf("want 1 remaining, got %d", len(onDisk))
	}
	if onDisk[0]["name"] != "BOI" {
		t.Errorf("want remaining=BOI, got %v", onDisk[0])
	}
}

// 20. DELETE 不存在 → 400
func TestDeleteSiteNotFound(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[{"name": "SET", "url": "https://set"}]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesDelete(t, r, "ghost")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "not found") && !strings.Contains(w.Body.String(), "不存在") {
		t.Errorf("want error to mention not found, got %s", w.Body.String())
	}
}

// 21. DELETE 缺 name query → 400
func TestDeleteSiteMissingNameQuery(t *testing.T) {
	crmDir := t.TempDir()
	r := newSitesEngine(t, crmDir)

	req, _ := http.NewRequest(http.MethodDelete, "/api/target-sites", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "name") {
		t.Errorf("want error to mention 'name', got %s", w.Body.String())
	}
}

// 22. DELETE 中文 name（带 URL 编码）→ 200
func TestDeleteSiteChineseName(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[
		{"name": "SET上市公司名录", "url": "https://set"}
	]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesDelete(t, r, "SET上市公司名录")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	onDisk := readSitesOnDisk(t, crmDir, sitesRelPath)
	if len(onDisk) != 0 {
		t.Errorf("want empty after delete, got %v", onDisk)
	}
}

// ===== 隔离性 / 副作用 =====

// 23. PATCH 不应残留 .tmp
func TestPatchSiteLeavesNoTmp(t *testing.T) {
	crmDir := t.TempDir()
	writeSitesJSON(t, crmDir, sitesRelPath, `[{"name": "SET", "url": "https://set"}]`)
	r := newSitesEngine(t, crmDir)

	w := doSitesPatch(t, r, "SET", `{"url": "https://new"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if _, err := os.Stat(filepath.Join(crmDir, sitesRelPath) + ".tmp"); !os.IsNotExist(err) {
		t.Errorf(".tmp should not exist, stat err=%v", err)
	}
}

// 24. POST 不应残留 .tmp
func TestPostSiteLeavesNoTmp(t *testing.T) {
	crmDir := t.TempDir()
	r := newSitesEngine(t, crmDir)

	w := doSitesPost(t, r, `{"name": "x", "url": "https://x"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if _, err := os.Stat(filepath.Join(crmDir, sitesRelPath) + ".tmp"); !os.IsNotExist(err) {
		t.Errorf(".tmp should not exist, stat err=%v", err)
	}
}
