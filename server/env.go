package main

import (
	"strings"
)

// parseEnv 把 .env 文本解析成 map[string]string。
// 逻辑与前端 src/ui/parseEnv.ts 保持一致：
//   - 空行 / "#" 开头 → 跳过
//   - "KEY=value"，value 自动 trim
//   - 包裹在 "..." 或 '...' 里的引号会去掉
//   - 等号只在第一个出现的位置切分（value 里可以有 =）
func parseEnv(text string) map[string]string {
	out := make(map[string]string)
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 {
			first, last := val[0], val[len(val)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if key == "" {
			continue
		}
		out[key] = val
	}
	return out
}