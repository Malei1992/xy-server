package handlers

// 本文件：公开数据源（target_sites.json）的 IO helpers。
// 格式：JSON 数组，每条是 TargetSite{name, url, country, industry, type}。
// name 为唯一标识（精确匹配 / 模糊匹配）。
//
// 设计要点：
//   - 缺文件 / 空文件 / "[]" 都视作 0 条记录，返空 slice
//   - WriteSites 原子写（tmp + rename）
//   - JSON 缩进 2 空格，末尾换行（与 level_io 一致）

import (
	"encoding/json"
	"os"
	"strings"
)

// TargetSite 单条公开数据源记录。
// 5 个字段：name（必填，唯一标识）/ url（必填）/ country / industry / type。
// 后三个为可选项；omitempty 让空字符串不会落盘（避免噪声）。
type TargetSite struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Country  string `json:"country,omitempty"`
	Industry string `json:"industry,omitempty"`
	Type     string `json:"type,omitempty"`
}

// ReadSites 读 path 指向的 JSON 文件。
// 文件不存在 / 内容为空 → 返空 slice + nil error。
// 损坏的 JSON 或非数组 → 返 nil + error。
func ReadSites(path string) ([]TargetSite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []TargetSite{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return []TargetSite{}, nil
	}
	var out []TargetSite
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []TargetSite{}
	}
	return out, nil
}

// WriteSites 原子写：先写 path+".tmp"，再 rename 到 path。
// JSON 用 2-空格缩进，文件末尾追加换行符。
// 父目录必须已存在；不会自动创建（调用方负责）。
func WriteSites(path string, sites []TargetSite) error {
	if sites == nil {
		sites = []TargetSite{}
	}
	data, err := json.MarshalIndent(sites, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// FindSiteByName 在 sites 中按 name 精确匹配；找不到返 nil。
// 大小写敏感（与 JSON 字段一致）。
func FindSiteByName(sites []TargetSite, name string) *TargetSite {
	for i := range sites {
		if sites[i].Name == name {
			return &sites[i]
		}
	}
	return nil
}

// FilterSitesByQuery 在 sites 中按 name 做子串匹配（模糊查询）。
// q 为空 → 返全部。找不到匹配 → 返空 slice。
// 大小写敏感（与 FindSiteByName 一致）。
// 始终返非 nil slice（避免 JSON 序列化为 null）。
func FilterSitesByQuery(sites []TargetSite, q string) []TargetSite {
	if q == "" {
		out := make([]TargetSite, len(sites))
		copy(out, sites)
		return out
	}
	out := make([]TargetSite, 0, len(sites))
	for _, s := range sites {
		if strings.Contains(s.Name, q) {
			out = append(out, s)
		}
	}
	return out
}
