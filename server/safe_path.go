package main

import (
	"errors"
	"path/filepath"
	"strings"
)

// safeJoin 在 root 下拼出一个安全路径：
//   - 拒绝绝对路径
//   - 拒绝包含 ".." 的路径段（防止 ../etc/passwd）
//   - 不限制后缀——CRM 数据是 .json，但留扩展以便后续扩展
// 返回相对 root 的最终绝对路径。
func safeJoin(root, rel string) (string, error) {
	if rel == "" {
		return "", errors.New("empty id")
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return "", errors.New("absolute path not allowed")
	}
	// 立即拒绝任何包含 .. 段的形式
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		if seg == ".." {
			return "", errors.New("path traversal not allowed")
		}
	}
	cleaned := filepath.Clean(rel)
	full := filepath.Join(root, cleaned)
	// 确保最终路径仍在 root 之下（用 Clean 再判一次）
	rootClean := filepath.Clean(root) + string(filepath.Separator)
	if !strings.HasPrefix(full+string(filepath.Separator), rootClean) {
		return "", errors.New("path escapes root")
	}
	return full, nil
}