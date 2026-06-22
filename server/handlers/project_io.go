package handlers

// 本文件：商机信息（crm/projects/*.json）的 IO helpers。
// 存储格式：每个 project 是独立 .json 文件，文件名 = "{id}.json"。
// id 由前端生成，格式 PRJ-{timestamp_ms}-{random_hex}。
//
// 设计要点：
//   - 缺目录 / 空目录 → 返空 slice + nil error
//   - JSON 损坏文件 / 空文件 / 非 .json / 子目录 / 不以 PRJ 开头 → 跳过，不影响其他文件
//   - 返回非 nil slice（避免 JSON 序列化为 null）
//   - 写文件：projectsMu 串行化 + 原子写（.tmp + rename），失败清理 .tmp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// projectsMu 序列化对 crm/projects/*.json 的并发写。
// 沿用 openclaw_config.go 的全局单锁模式：read-modify-write 不丢更新；rename 失败时 .tmp 清理。
// 三个实体（project / task / opportunity）各自一个 mutex，互不阻塞。
var projectsMu sync.Mutex

// 商机文件名 / id 的约定前缀（与 schema.md 中 project id 格式一致）。
// 商机目录里非 PRJ 开头的 .json 不视为商机记录：list 跳过。
const projectIDPrefix = "PRJ"

// Project 单条商机记录。
// 字段对应前端写入的 JSON；与 customer_id 关联到 crm/customers/{customer_id}.json。
//
// 状态（status）枚举：跟进中 / 谈判中 / 签约中 / 已落地 / 已关闭。
// 意向等级（intent_level）枚举：S / A / B / C；写入端不在此处校验，由 PATCH/POST handler 负责。
// 本类型只负责反序列化。
type Project struct {
	ID              string    `json:"id"`
	CreatedAt       string    `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
	ProjectName     string    `json:"project_name"`
	CustomerID      string    `json:"customer_id"`
	Status          string    `json:"status"`
	IntentLevel     string    `json:"intent_level,omitempty"`
	AssignedTo      string    `json:"assigned_to,omitempty"`
	Notes           string    `json:"notes,omitempty"`
	RelatedEmailIDs []string  `json:"related_email_ids,omitempty"`
}

// ReadProjects 扫描 dirPath 下所有 *.json，解析为 Project 列表。
// 行为约定（与 test 配套）：
//   - 目录不存在 / 目录为空 → 返空 slice + nil error
//   - 损坏 JSON → 跳过，不影响其他文件
//   - 空文件 → 跳过
//   - 非 .json 后缀 / 子目录 / 不以 PRJ 开头 → 跳过
//   - 结果按 id 升序排序，输出稳定
func ReadProjects(dirPath string) ([]Project, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Project{}, nil
		}
		return nil, err
	}

	out := make([]Project, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		if !strings.HasPrefix(name, projectIDPrefix) {
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
		var p Project
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		out = append(out, p)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// WriteProject 原子写单条 project 到 dirPath/{id}.json。
//   - dirPath 不存在 → MkdirAll 自动创建
//   - 先写 <path>.tmp，再 rename 到 <path>；rename 失败时清理 .tmp
//   - 由 projectsMu 串行化，避免并发 read-modify-write 互相覆盖
//
// 仅供 PatchProjectStatus / 后续 project 写操作使用；现有 read 路径（ReadProjects）不受影响。
func WriteProject(dirPath string, p Project) error {
	projectsMu.Lock()
	defer projectsMu.Unlock()
	return writeJSONFile(dirPath, p.ID+".json", p)
}

// writeJSONFile 原子写 v 到 dir/filename：marshalIndent → .tmp → rename；rename 失败清理 .tmp。
// dir 不存在 → MkdirAll 自动创建。
//
// 这是 project / task / opportunity 三个 Write* 共享的底层 helper。
// 行为对齐 openclaw_config.go 的 syncOpenclawConfig 写盘流程：先 .tmp 再 rename，失败清理。
//
// 注意：调用方负责加各自的 entity-level mutex（本函数不加锁）。
func writeJSONFile(dir, filename string, v any) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	full := filepath.Join(dir, filename)
	tmp := full + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, full); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
