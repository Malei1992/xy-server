package handlers

import (
	"encoding/json"
	"os"
)

// ReadLevelFile 读 path 指向的 JSON 文件到 map。文件不存在或为空 → 返回空 map + nil。
// 损坏的 JSON → 返回 nil + error（由上层决定 500）。
func ReadLevelFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]string{}, nil
	}
	var out map[string]string
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]string{}
	}
	return out, nil
}

// WriteLevelFile 原子写：先写 path+".tmp"，再 rename 到 path。
// JSON 用 2-空格缩进，文件末尾追加换行符。
// 父目录必须已存在；不会自动创建（调用方负责）。
func WriteLevelFile(path string, levels map[string]string) error {
	data, err := json.MarshalIndent(levels, "", "  ")
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
