package handlers

// 本文件：公开信息（crm/opportunities/*.json）的 IO helpers。
// 存储格式：每个 opportunity 是独立 .json 文件，文件名 = "{id}.json"。
// id 由 OpportunityAgent 生成，格式 OPP-{timestamp_ms}-{random_hex}。
//
// 设计要点：
//   - 缺目录 / 空目录 → 返空 slice + nil error
//   - JSON 损坏文件 / 空文件 / 非 .json / 子目录 / 不以 OPP 开头 → 跳过，不影响其他文件
//   - 返回非 nil slice（避免 JSON 序列化为 null）
//   - 排序按 id 升序

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 公开信息文件名 / id 的约定前缀（与 schema.md 中 opportunity id 格式一致）。
// 商机目录里非 OPP 开头的 .json 不视为商机记录：list 跳过。
const opportunityIDPrefix = "OPP"

// Opportunity 单条公开信息记录。
// 字段对应 schema.md 中 opportunity 的 JSON 字段。customer_id 关联到 crm/customers/{customer_id}.json。
//
// source_type 枚举：新闻搜索 / 行业报告 / 招标公告 / 企业公告 / 其他
// status 枚举：待评估 / 跟进中 / 已转化 / 已关闭
//
// 写入端不在此处校验，由 PATCH/POST handler 负责；本类型只负责反序列化。
type Opportunity struct {
	ID              string `json:"id"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	OpportunityName string `json:"opportunity_name"`
	CustomerID      string `json:"customer_id,omitempty"`
	OpportunityInfo string `json:"opportunity_info,omitempty"`
	SourceURL       string `json:"source_url,omitempty"`
	SourceType      string `json:"source_type"`
	Status          string `json:"status"`
	Notes           string `json:"notes,omitempty"`
}

// ReadOpportunities 扫描 dirPath 下所有 *.json，解析为 Opportunity 列表。
// 行为约定（与 test 配套）：
//   - 目录不存在 / 目录为空 → 返空 slice + nil error
//   - 损坏 JSON → 跳过，不影响其他文件
//   - 空文件 → 跳过
//   - 非 .json 后缀 / 子目录 / 不以 OPP 开头 → 跳过
//   - 结果按 id 升序排序，输出稳定
func ReadOpportunities(dirPath string) ([]Opportunity, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Opportunity{}, nil
		}
		return nil, err
	}

	out := make([]Opportunity, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		if !strings.HasPrefix(name, opportunityIDPrefix) {
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
		var o Opportunity
		if err := json.Unmarshal(data, &o); err != nil {
			continue
		}
		out = append(out, o)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}