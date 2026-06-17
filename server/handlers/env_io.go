package handlers

import (
	"os"
	"sort"
	"strings"
)

// ParseEnvContent 把 .env 文本解析成 map[string]string。
// 行为与 main 包的 parseEnv 基本一致（兼容前端 parseEnv.ts），但额外支持
// WriteEnvFile 写出的转义：双引号包裹的值内 \" → "，单引号包裹的值内 \' → '。
//   - 空行 / "#" 开头 → 跳过
//   - "KEY=value"，value 自动 trim
//   - 包裹在 "..." 或 '...' 里的引号会去掉
//   - 等号只在第一个出现的位置切分（value 里可以有 =）
//   - CRLF 行尾 → 自动 trim \r
func ParseEnvContent(content string) map[string]string {
	out := make(map[string]string)
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 {
			first, last := val[0], val[len(val)-1]
			if first == '"' && last == '"' {
				val = val[1 : len(val)-1]
				val = strings.ReplaceAll(val, `\"`, `"`)
			} else if first == '\'' && last == '\'' {
				val = val[1 : len(val)-1]
				val = strings.ReplaceAll(val, `\'`, `'`)
			}
		}
		if key == "" {
			continue
		}
		out[key] = val
	}
	return out
}

// ReadEnvFile 读 path 指向的 .env 文件并解析成 map。
// 文件不存在 → 返回空 map + nil error。
func ReadEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	return ParseEnvContent(string(data)), nil
}

// WriteEnvFile 把 env map 写回 path，按 key 字母序排序，每行 KEY="VALUE"（空值写 KEY=""）。
// 原子写：先写 path+".tmp"，再 rename 到 path；rename 失败时清理 .tmp。
//
// 写回时只对双引号做反转义（\"）；其它字符（包括反斜杠、换行、$）按字面量处理。
// ParseEnvContent 读取时只去掉外层引号，不解释转义序列——保持与 .env 简单格式一致。
func WriteEnvFile(path string, env map[string]string) error {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		v := env[k]
		// 双引号转义为 \"（必须，否则破坏外层包裹）
		escaped := strings.ReplaceAll(v, `"`, `\"`)
		sb.WriteString(k)
		sb.WriteString(`="`)
		sb.WriteString(escaped)
		sb.WriteString("\"\n")
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(sb.String()), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
