package handlers

// 本文件：project_io.go 的单元测试。
// 覆盖 ReadProjects：缺目录 / 空目录 / 多文件 / 损坏文件 / 非 .json 文件 / 子目录。

import (
	"os"
	"path/filepath"
	"testing"
)

// 1. 目录不存在 → 返空 slice + nil error
func TestReadProjectsDirMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")

	projects, err := ReadProjects(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if projects == nil {
		t.Errorf("want non-nil empty slice, got nil")
	}
	if len(projects) != 0 {
		t.Errorf("want 0, got %d", len(projects))
	}
}

// 2. 目录存在但为空 → 返空 slice + nil error
func TestReadProjectsDirEmpty(t *testing.T) {
	dir := t.TempDir()
	projects, err := ReadProjects(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("want 0, got %d", len(projects))
	}
}

// 3. 多文件 → 全部解析
func TestReadProjectsMultiple(t *testing.T) {
	dir := t.TempDir()
	// 写 2 个项目
	writeProjectFile(t, dir, "PRJ-1", `{
		"id": "PRJ-1",
		"project_name": "项目1",
		"customer_id": "CUST-1",
		"status": "跟进中",
		"updated_at": "2026-06-16T10:00:00Z"
	}`)
	writeProjectFile(t, dir, "PRJ-2", `{
		"id": "PRJ-2",
		"project_name": "项目2",
		"customer_id": "CUST-2",
		"status": "谈判中",
		"updated_at": "2026-06-16T11:00:00Z"
	}`)

	projects, err := ReadProjects(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("want 2, got %d", len(projects))
	}
	// 至少要解析到这 2 个 ID
	ids := map[string]bool{}
	for _, p := range projects {
		ids[p.ID] = true
	}
	if !ids["PRJ-1"] || !ids["PRJ-2"] {
		t.Errorf("want PRJ-1 and PRJ-2, got %v", ids)
	}
}

// 4. 损坏的 JSON 文件 → 跳过（不影响其他文件）
func TestReadProjectsSkipsCorrupt(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "PRJ-good", `{"id":"PRJ-good","project_name":"好的","customer_id":"CUST-1","status":"跟进中","updated_at":"2026-06-16T10:00:00Z"}`)
	writeProjectFile(t, dir, "PRJ-bad", `{this is not valid json`)

	projects, err := ReadProjects(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("want 1 (corrupt skipped), got %d", len(projects))
	}
	if projects[0].ID != "PRJ-good" {
		t.Errorf("want PRJ-good, got %q", projects[0].ID)
	}
}

// 5. 非 .json 后缀的文件 → 跳过
func TestReadProjectsIgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "PRJ-good", `{"id":"PRJ-good","project_name":"x","customer_id":"CUST-1","status":"跟进中","updated_at":"2026-06-16T10:00:00Z"}`)
	// 写一个 .bak 文件
	if err := os.WriteFile(filepath.Join(dir, "PRJ-bad.bak"), []byte("not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// 写一个 .DS_Store
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	projects, err := ReadProjects(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("want 1 (only PRJ-good), got %d", len(projects))
	}
}

// 6. 子目录 → 跳过（不算 project）
func TestReadProjectsIgnoresSubdirs(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "PRJ-good", `{"id":"PRJ-good","project_name":"x","customer_id":"CUST-1","status":"跟进中","updated_at":"2026-06-16T10:00:00Z"}`)
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	projects, err := ReadProjects(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("want 1, got %d", len(projects))
	}
}

// 7. 空文件 → 跳过
func TestReadProjectsSkipsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "PRJ-good", `{"id":"PRJ-good","project_name":"x","customer_id":"CUST-1","status":"跟进中","updated_at":"2026-06-16T10:00:00Z"}`)
	if err := os.WriteFile(filepath.Join(dir, "PRJ-empty.json"), []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	projects, err := ReadProjects(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("want 1 (empty skipped), got %d", len(projects))
	}
}

// 8. 中文 project_name + notes round-trip
func TestReadProjectsChineseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "PRJ-1", `{
		"id": "PRJ-1",
		"project_name": "华为泰国数据中心项目",
		"customer_id": "CUST-1",
		"status": "谈判中",
		"assigned_to": "张三",
		"notes": "客户对价格敏感，需要进一步沟通",
		"updated_at": "2026-06-16T10:00:00Z"
	}`)

	projects, err := ReadProjects(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("want 1, got %d", len(projects))
	}
	if projects[0].ProjectName != "华为泰国数据中心项目" {
		t.Errorf("project_name lost: got %q", projects[0].ProjectName)
	}
	if projects[0].Notes != "客户对价格敏感，需要进一步沟通" {
		t.Errorf("notes lost: got %q", projects[0].Notes)
	}
	if projects[0].AssignedTo != "张三" {
		t.Errorf("assigned_to lost: got %q", projects[0].AssignedTo)
	}
}

// 9. 不以 PRJ 开头的 .json → 跳过（哪怕内容是合法 JSON）
func TestReadProjectsIgnoresNonPRJPrefix(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "PRJ-good", `{"id":"PRJ-good","project_name":"x","customer_id":"CUST-1","status":"跟进中","updated_at":"2026-06-16T10:00:00Z"}`)
	// 几个非 PRJ 前缀的 .json：TEMP.json / TASK-stowaway.json / customer-prj.json
	if err := os.WriteFile(filepath.Join(dir, "TEMP.json"), []byte(`{"id":"TEMP"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "TASK-stowaway.json"), []byte(`{"id":"TASK-stowaway"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prj-lowercase.json"), []byte(`{"id":"prj-lowercase"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	projects, err := ReadProjects(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("want 1 (only PRJ-good), got %d: %v", len(projects), projects)
	}
	if projects[0].ID != "PRJ-good" {
		t.Errorf("want PRJ-good, got %q", projects[0].ID)
	}
}

// 10. 全是非 PRJ 前缀 → 返空
func TestReadProjectsEmptyWhenAllNonPRJ(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "TEMP.json"), []byte(`{"id":"TEMP"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "TASK-1.json"), []byte(`{"id":"TASK-1"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	projects, err := ReadProjects(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if projects == nil {
		t.Errorf("want non-nil empty slice, got nil")
	}
	if len(projects) != 0 {
		t.Errorf("want 0, got %d: %v", len(projects), projects)
	}
}

// helper: 在 dir/{id}.json 写一段 JSON 内容
func writeProjectFile(t *testing.T, dir, id, content string) {
	t.Helper()
	full := filepath.Join(dir, id+".json")
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}
