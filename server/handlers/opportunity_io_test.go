package handlers

// 本文件：opportunity_io.go 的单元测试。
// 覆盖 ReadOpportunities：缺目录 / 空目录 / 多文件 / 损坏文件 / 非 .json 文件 / 子目录 / 空文件 / 中文 round-trip / 前缀过滤。
// 覆盖 WriteOpportunity：创建新文件 / 覆盖旧文件 / 并发写不损坏。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// 1. 目录不存在 → 返空 slice + nil error
func TestReadOpportunitiesDirMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")

	opps, err := ReadOpportunities(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if opps == nil {
		t.Errorf("want non-nil empty slice, got nil")
	}
	if len(opps) != 0 {
		t.Errorf("want 0, got %d", len(opps))
	}
}

// 2. 目录存在但为空 → 返空 slice + nil error
func TestReadOpportunitiesDirEmpty(t *testing.T) {
	dir := t.TempDir()
	opps, err := ReadOpportunities(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(opps) != 0 {
		t.Errorf("want 0, got %d", len(opps))
	}
}

// 3. 多文件 → 全部解析
func TestReadOpportunitiesMultiple(t *testing.T) {
	dir := t.TempDir()
	writeOpportunityFile(t, dir, "OPP-1", `{
		"id":"OPP-1","opportunity_name":"新厂投资","customer_id":"CUST-1",
		"source_type":"新闻搜索","status":"待评估",
		"created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)
	writeOpportunityFile(t, dir, "OPP-2", `{
		"id":"OPP-2","opportunity_name":"扩建项目","customer_id":"CUST-2",
		"source_type":"行业报告","status":"跟进中",
		"created_at":"2026-06-15T11:00:00Z","updated_at":"2026-06-16T11:00:00Z"
	}`)

	opps, err := ReadOpportunities(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(opps) != 2 {
		t.Fatalf("want 2, got %d", len(opps))
	}
	ids := map[string]bool{}
	for _, o := range opps {
		ids[o.ID] = true
	}
	if !ids["OPP-1"] || !ids["OPP-2"] {
		t.Errorf("want OPP-1 and OPP-2, got %v", ids)
	}
}

// 4. 损坏的 JSON 文件 → 跳过
func TestReadOpportunitiesSkipsCorrupt(t *testing.T) {
	dir := t.TempDir()
	writeOpportunityFile(t, dir, "OPP-good", `{"id":"OPP-good","opportunity_name":"好","source_type":"新闻搜索","status":"待评估","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`)
	writeOpportunityFile(t, dir, "OPP-bad", `{this is not valid json`)

	opps, err := ReadOpportunities(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("want 1 (corrupt skipped), got %d", len(opps))
	}
	if opps[0].ID != "OPP-good" {
		t.Errorf("want OPP-good, got %q", opps[0].ID)
	}
}

// 5. 非 .json 后缀 → 跳过
func TestReadOpportunitiesIgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	writeOpportunityFile(t, dir, "OPP-good", `{"id":"OPP-good","opportunity_name":"x","source_type":"新闻搜索","status":"待评估","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	opps, err := ReadOpportunities(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(opps) != 1 {
		t.Errorf("want 1, got %d", len(opps))
	}
}

// 6. 子目录 → 跳过
func TestReadOpportunitiesIgnoresSubdirs(t *testing.T) {
	dir := t.TempDir()
	writeOpportunityFile(t, dir, "OPP-good", `{"id":"OPP-good","opportunity_name":"x","source_type":"新闻搜索","status":"待评估","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`)
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	opps, err := ReadOpportunities(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(opps) != 1 {
		t.Errorf("want 1, got %d", len(opps))
	}
}

// 7. 空文件 → 跳过
func TestReadOpportunitiesSkipsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeOpportunityFile(t, dir, "OPP-good", `{"id":"OPP-good","opportunity_name":"x","source_type":"新闻搜索","status":"待评估","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`)
	if err := os.WriteFile(filepath.Join(dir, "OPP-empty.json"), []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	opps, err := ReadOpportunities(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(opps) != 1 {
		t.Errorf("want 1 (empty skipped), got %d", len(opps))
	}
}

// 8. 中文 opportunity_name + opportunity_info round-trip
func TestReadOpportunitiesChineseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeOpportunityFile(t, dir, "OPP-1", `{
		"id":"OPP-1",
		"opportunity_name":"泰国正大集团拟新建食品加工厂",
		"opportunity_info":"占地约 200 亩，预计投资 5 亿美元，2027 年投产",
		"source_url":"https://example.com/news/123",
		"source_type":"新闻搜索",
		"status":"待评估",
		"notes":"与张三跟进的客户重叠",
		"customer_id":"CUST-1",
		"created_at":"2026-06-15T10:00:00Z",
		"updated_at":"2026-06-16T10:00:00Z"
	}`)

	opps, err := ReadOpportunities(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("want 1, got %d", len(opps))
	}
	if opps[0].OpportunityName != "泰国正大集团拟新建食品加工厂" {
		t.Errorf("opportunity_name lost: %q", opps[0].OpportunityName)
	}
	if opps[0].OpportunityInfo != "占地约 200 亩，预计投资 5 亿美元，2027 年投产" {
		t.Errorf("opportunity_info lost: %q", opps[0].OpportunityInfo)
	}
	if opps[0].SourceURL != "https://example.com/news/123" {
		t.Errorf("source_url lost: %q", opps[0].SourceURL)
	}
	if opps[0].Notes != "与张三跟进的客户重叠" {
		t.Errorf("notes lost: %q", opps[0].Notes)
	}
}

// 9. 不以 OPP 开头的 .json → 跳过（哪怕内容是合法 JSON）
func TestReadOpportunitiesIgnoresNonOPPPrefix(t *testing.T) {
	dir := t.TempDir()
	writeOpportunityFile(t, dir, "OPP-good", `{"id":"OPP-good","opportunity_name":"x","source_type":"新闻搜索","status":"待评估","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`)
	// 几个非 OPP 前缀的 .json：TEMP.json / PRJ-stowaway.json / opp-lowercase.json
	if err := os.WriteFile(filepath.Join(dir, "TEMP.json"), []byte(`{"id":"TEMP"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "PRJ-stowaway.json"), []byte(`{"id":"PRJ-stowaway"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "opp-lowercase.json"), []byte(`{"id":"opp-lowercase"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	opps, err := ReadOpportunities(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("want 1 (only OPP-good), got %d: %v", len(opps), opps)
	}
	if opps[0].ID != "OPP-good" {
		t.Errorf("want OPP-good, got %q", opps[0].ID)
	}
}

// 10. 全是非 OPP 前缀 → 返空
func TestReadOpportunitiesEmptyWhenAllNonOPP(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "TEMP.json"), []byte(`{"id":"TEMP"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "PRJ-1.json"), []byte(`{"id":"PRJ-1"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	opps, err := ReadOpportunities(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if opps == nil {
		t.Errorf("want non-nil empty slice, got nil")
	}
	if len(opps) != 0 {
		t.Errorf("want 0, got %d: %v", len(opps), opps)
	}
}

// ===== WriteOpportunity 原子写 + mutex 保护 =====

// 11. 写一个新文件 → 读出来字段对得上
func TestWriteOpportunity_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	opp := Opportunity{
		ID:              "OPP-1",
		OpportunityName: "正大新厂",
		SourceType:      "新闻搜索",
		Status:          "待评估",
		UpdatedAt:       "2026-06-22T10:00:00Z",
	}

	if err := WriteOpportunity(dir, opp); err != nil {
		t.Fatalf("WriteOpportunity: %v", err)
	}

	opps, err := ReadOpportunities(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("want 1, got %d", len(opps))
	}
	if opps[0].ID != "OPP-1" {
		t.Errorf("id = %q, want OPP-1", opps[0].ID)
	}
	if opps[0].Status != "待评估" {
		t.Errorf("status = %q, want 待评估", opps[0].Status)
	}
	if opps[0].OpportunityName != "正大新厂" {
		t.Errorf("opportunity_name = %q, want 正大新厂", opps[0].OpportunityName)
	}
}

// 12. 覆盖已存在的文件 → 新内容生效
func TestWriteOpportunity_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	old := Opportunity{ID: "OPP-1", OpportunityName: "旧", SourceType: "新闻搜索", Status: "待评估", UpdatedAt: "2026-06-15T10:00:00Z"}
	if err := WriteOpportunity(dir, old); err != nil {
		t.Fatalf("seed: %v", err)
	}

	newOpp := Opportunity{ID: "OPP-1", OpportunityName: "新", SourceType: "新闻搜索", Status: "已转化", UpdatedAt: "2026-06-22T10:00:00Z"}
	if err := WriteOpportunity(dir, newOpp); err != nil {
		t.Fatalf("overwrite: %v", err)
	}

	opps, err := ReadOpportunities(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("want 1, got %d", len(opps))
	}
	if opps[0].Status != "已转化" {
		t.Errorf("status = %q, want 已转化", opps[0].Status)
	}
	if opps[0].OpportunityName != "新" {
		t.Errorf("opportunity_name = %q, want 新", opps[0].OpportunityName)
	}
}

// 13. 并发写同一个 id 不会损坏文件(`-race` 配合看)
func TestWriteOpportunity_ConcurrentSameID(t *testing.T) {
	dir := t.TempDir()
	const goroutines = 8
	const opsPerG = 25

	statuses := []string{"待评估", "跟进中", "已转化", "已关闭"}

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerG; i++ {
				opp := Opportunity{
					ID:         "OPP-shared",
					SourceType: "新闻搜索",
					Status:     statuses[(g+i)%len(statuses)],
					UpdatedAt:  "2026-06-22T10:00:00Z",
				}
				if err := WriteOpportunity(dir, opp); err != nil {
					t.Errorf("WriteOpportunity g=%d i=%d: %v", g, i, err)
					return
				}
			}
		}()
	}
	wg.Wait()

	data, err := os.ReadFile(filepath.Join(dir, "OPP-shared.json"))
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	var opp Opportunity
	if err := json.Unmarshal(data, &opp); err != nil {
		t.Fatalf("parse final file: %v; raw=%s", err, data)
	}
	if opp.ID != "OPP-shared" {
		t.Errorf("id = %q, want OPP-shared", opp.ID)
	}
	valid := false
	for _, s := range statuses {
		if opp.Status == s {
			valid = true
			break
		}
	}
	if !valid {
		t.Errorf("status = %q, want one of %v", opp.Status, statuses)
	}
}

// helper: 在 dir/{id}.json 写一段 JSON 内容
func writeOpportunityFile(t *testing.T, dir, id, content string) {
	t.Helper()
	full := filepath.Join(dir, id+".json")
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}