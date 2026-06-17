package handlers

// 本文件：task_io.go 的单元测试。
// 覆盖 ReadTasks：缺目录 / 空目录 / 多文件 / 损坏文件 / 非 .json 文件 / 子目录 / 空文件 / 中文 round-trip。

import (
	"os"
	"path/filepath"
	"testing"
)

// 1. 目录不存在 → 返空 slice + nil error
func TestReadTasksDirMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")

	tasks, err := ReadTasks(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if tasks == nil {
		t.Errorf("want non-nil empty slice, got nil")
	}
	if len(tasks) != 0 {
		t.Errorf("want 0, got %d", len(tasks))
	}
}

// 2. 目录存在但为空 → 返空 slice + nil error
func TestReadTasksDirEmpty(t *testing.T) {
	dir := t.TempDir()
	tasks, err := ReadTasks(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("want 0, got %d", len(tasks))
	}
}

// 3. 多文件 → 全部解析
func TestReadTasksMultiple(t *testing.T) {
	dir := t.TempDir()
	writeTaskFile(t, dir, "TASK-1", `{
		"id":"TASK-1","title":"任务1","type":"compliance_blocked","priority":"P1","status":"pending",
		"customer_id":"CUST-1","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"
	}`)
	writeTaskFile(t, dir, "TASK-2", `{
		"id":"TASK-2","title":"任务2","type":"anomaly_alert","priority":"P0","status":"in_progress",
		"customer_id":"CUST-2","created_at":"2026-06-15T11:00:00Z","updated_at":"2026-06-16T11:00:00Z"
	}`)

	tasks, err := ReadTasks(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("want 2, got %d", len(tasks))
	}
	ids := map[string]bool{}
	for _, t1 := range tasks {
		ids[t1.ID] = true
	}
	if !ids["TASK-1"] || !ids["TASK-2"] {
		t.Errorf("want TASK-1 and TASK-2, got %v", ids)
	}
}

// 4. 损坏的 JSON 文件 → 跳过
func TestReadTasksSkipsCorrupt(t *testing.T) {
	dir := t.TempDir()
	writeTaskFile(t, dir, "TASK-good", `{"id":"TASK-good","title":"好","type":"compliance_blocked","priority":"P1","status":"pending","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`)
	writeTaskFile(t, dir, "TASK-bad", `{this is not valid json`)

	tasks, err := ReadTasks(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("want 1 (corrupt skipped), got %d", len(tasks))
	}
	if tasks[0].ID != "TASK-good" {
		t.Errorf("want TASK-good, got %q", tasks[0].ID)
	}
}

// 5. 非 .json 后缀 → 跳过
func TestReadTasksIgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	writeTaskFile(t, dir, "TASK-good", `{"id":"TASK-good","title":"x","type":"compliance_blocked","priority":"P1","status":"pending","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tasks, err := ReadTasks(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("want 1, got %d", len(tasks))
	}
}

// 6. 子目录 → 跳过
func TestReadTasksIgnoresSubdirs(t *testing.T) {
	dir := t.TempDir()
	writeTaskFile(t, dir, "TASK-good", `{"id":"TASK-good","title":"x","type":"compliance_blocked","priority":"P1","status":"pending","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`)
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	tasks, err := ReadTasks(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("want 1, got %d", len(tasks))
	}
}

// 7. 空文件 → 跳过
func TestReadTasksSkipsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeTaskFile(t, dir, "TASK-good", `{"id":"TASK-good","title":"x","type":"compliance_blocked","priority":"P1","status":"pending","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`)
	if err := os.WriteFile(filepath.Join(dir, "TASK-empty.json"), []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tasks, err := ReadTasks(dir)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("want 1 (empty skipped), got %d", len(tasks))
	}
}

// 8. 中文 title + description round-trip
func TestReadTasksChineseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeTaskFile(t, dir, "TASK-1", `{
		"id":"TASK-1",
		"title":"合规文件缺失：泰国 BOI",
		"description":"客户缺少投资促进委员会证明，请尽快补充",
		"type":"compliance_blocked",
		"priority":"P1",
		"status":"pending",
		"customer_id":"CUST-1",
		"assigned_to":"张三",
		"created_at":"2026-06-15T10:00:00Z",
		"updated_at":"2026-06-16T10:00:00Z"
	}`)

	tasks, err := ReadTasks(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("want 1, got %d", len(tasks))
	}
	if tasks[0].Title != "合规文件缺失：泰国 BOI" {
		t.Errorf("title lost: %q", tasks[0].Title)
	}
	if tasks[0].Description != "客户缺少投资促进委员会证明，请尽快补充" {
		t.Errorf("description lost: %q", tasks[0].Description)
	}
	if tasks[0].AssignedTo != "张三" {
		t.Errorf("assigned_to lost: %q", tasks[0].AssignedTo)
	}
}

// 9. 不以 TASK 开头的 .json → 跳过（哪怕内容是合法 JSON）
func TestReadTasksIgnoresNonTASKPrefix(t *testing.T) {
	dir := t.TempDir()
	writeTaskFile(t, dir, "TASK-good", `{"id":"TASK-good","title":"x","type":"compliance_blocked","priority":"P1","status":"pending","created_at":"2026-06-15T10:00:00Z","updated_at":"2026-06-16T10:00:00Z"}`)
	// 几个非 TASK 前缀的 .json：TEMP.json / PRJ-stowaway.json / task-lowercase.json
	if err := os.WriteFile(filepath.Join(dir, "TEMP.json"), []byte(`{"id":"TEMP"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "PRJ-stowaway.json"), []byte(`{"id":"PRJ-stowaway"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "task-lowercase.json"), []byte(`{"id":"task-lowercase"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tasks, err := ReadTasks(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("want 1 (only TASK-good), got %d: %v", len(tasks), tasks)
	}
	if tasks[0].ID != "TASK-good" {
		t.Errorf("want TASK-good, got %q", tasks[0].ID)
	}
}

// 10. 全是非 TASK 前缀 → 返空
func TestReadTasksEmptyWhenAllNonTASK(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "TEMP.json"), []byte(`{"id":"TEMP"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "PRJ-1.json"), []byte(`{"id":"PRJ-1"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tasks, err := ReadTasks(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if tasks == nil {
		t.Errorf("want non-nil empty slice, got nil")
	}
	if len(tasks) != 0 {
		t.Errorf("want 0, got %d: %v", len(tasks), tasks)
	}
}

// helper: 在 dir/{id}.json 写一段 JSON 内容
func writeTaskFile(t *testing.T, dir, id, content string) {
	t.Helper()
	full := filepath.Join(dir, id+".json")
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}
