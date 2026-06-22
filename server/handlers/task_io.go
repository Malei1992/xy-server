package handlers

// 本文件：代办任务（crm/tasks/*.json）的 IO helpers。
// 存储格式：每个 task 是独立 .json 文件，文件名 = "{id}.json"。
// id 由写入者生成，格式 TASK-{timestamp_ms}-{random_hex}。
//
// 设计要点：
//   - 缺目录 / 空目录 → 返空 slice + nil error
//   - JSON 损坏文件 / 空文件 / 非 .json / 子目录 / 不以 TASK 开头 → 跳过，不影响其他文件
//   - 返回非 nil slice（避免 JSON 序列化为 null）
//   - 排序按 id 升序（与 projects 一致）
//   - 写文件：tasksMu 串行化 + 原子写（.tmp + rename），失败清理 .tmp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// tasksMu 序列化对 crm/tasks/*.json 的并发写。
// 沿用 openclaw_config.go 的全局单锁模式：read-modify-write 不丢更新；rename 失败时 .tmp 清理。
// 三个实体（project / task / opportunity）各自一个 mutex，互不阻塞。
var tasksMu sync.Mutex

// 任务文件名 / id 的约定前缀（与 schema.md 中 task id 格式一致）。
// 任务目录里非 TASK 开头的 .json 不视为任务记录：list 跳过。
const taskIDPrefix = "TASK"

// Task 单条代办任务记录。
// 字段对应 schema.md 中 task 的 JSON 字段。customer_id 关联到 crm/customers/{customer_id}.json。
//
// type 枚举：data_insufficient / compliance_blocked / llm_failure /
//            human_notify_failed / review_timeout / complex_inquiry /
//            anomaly_alert / low_confidence / send_failed
// priority 枚举：P0 / P1 / P2 / P3
// status 枚举：pending / in_progress / resolved / dismissed
//
// 写入端不在此处校验，由 PATCH/POST handler 负责；本类型只负责反序列化。
type Task struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	Source      string `json:"source,omitempty"`
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Status      string `json:"status"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	CustomerID  string `json:"customer_id,omitempty"`
	EmailID     string `json:"email_id,omitempty"`
	AssignedTo  string `json:"assigned_to,omitempty"`
	ResolvedAt  string `json:"resolved_at,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
}

// ReadTasks 扫描 dirPath 下所有 *.json，解析为 Task 列表。
// 行为约定（与 test 配套）：
//   - 目录不存在 / 目录为空 → 返空 slice + nil error
//   - 损坏 JSON → 跳过，不影响其他文件
//   - 空文件 → 跳过
//   - 非 .json 后缀 / 子目录 / 不以 TASK 开头 → 跳过
//   - 结果按 id 升序排序，输出稳定
func ReadTasks(dirPath string) ([]Task, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Task{}, nil
		}
		return nil, err
	}

	out := make([]Task, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		if !strings.HasPrefix(name, taskIDPrefix) {
			continue
		}
		full := filepath.Join(dirPath, name)
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		if len(data) == 0 {
			continue
		}
		var t Task
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}
		out = append(out, t)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// WriteTask 原子写单条 task 到 dirPath/{id}.json。
//   - dirPath 不存在 → MkdirAll 自动创建
//   - 先写 <path>.tmp，再 rename 到 <path>；rename 失败时清理 .tmp
//   - 由 tasksMu 串行化，避免并发 read-modify-write 互相覆盖
//
// 仅供 PatchTaskStatus / 后续 task 写操作使用；现有 read 路径（ReadTasks）不受影响。
func WriteTask(dirPath string, t Task) error {
	tasksMu.Lock()
	defer tasksMu.Unlock()
	return writeJSONFile(dirPath, t.ID+".json", t)
}
