package handlers

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ----- ParseEnvContent 单元测试 -----

func TestParseEnvContent(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "empty",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "comments and blanks",
			input: "# top\n\n# another\n",
			want:  map[string]string{},
		},
		{
			name:  "basic",
			input: "FOO=bar\nBAZ=qux\n",
			want:  map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:  "double quotes",
			input: `FOO="hello world"`,
			want:  map[string]string{"FOO": "hello world"},
		},
		{
			name:  "single quotes",
			input: `FOO='hello world'`,
			want:  map[string]string{"FOO": "hello world"},
		},
		{
			name:  "value with equals",
			input: "URL=a=b=c",
			want:  map[string]string{"URL": "a=b=c"},
		},
		{
			name:  "whitespace trimmed",
			input: "  FOO  =  bar  \n",
			want:  map[string]string{"FOO": "bar"},
		},
		{
			name:  "line without equals skipped",
			input: "JUST_A_KEY\nFOO=bar",
			want:  map[string]string{"FOO": "bar"},
		},
		{
			name:  "mixed",
			input: "# header\n\nDB_URL='postgres://u:p@h/d'\nEMPTY=\nKEY=val=ue\n",
			want: map[string]string{
				"DB_URL": "postgres://u:p@h/d",
				"EMPTY":  "",
				"KEY":    "val=ue",
			},
		},
		{
			name:  "crlf handled",
			input: "FOO=bar\r\nBAZ=qux\r\n",
			want:  map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseEnvContent(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseEnvContent(%q)\n got: %v\nwant: %v", tc.input, got, tc.want)
			}
		})
	}
}

// ----- ReadEnvFile -----

func TestReadEnvFile_Missing(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.env")

	got, err := ReadEnvFile(missing)
	if err != nil {
		t.Fatalf("want nil error for missing file, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty map for missing file, got %v", got)
	}
}

func TestReadEnvFile_Happy(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "FOO=bar\nBAZ=qux\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := ReadEnvFile(envPath)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	want := map[string]string{"FOO": "bar", "BAZ": "qux"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// ----- WriteEnvFile -----

func TestWriteEnvFile_SortedAndQuoted(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	// 注意：WITH_NEWLINE 用 raw string literal（反引号），里面是字面反斜杠+n 两个字符
	// 不是真实的换行符（真实的 LF 会破坏 .env 行结构）。
	env := map[string]string{
		"SMTP_HOST":    "smtp.example.com",
		"IMAP_PORT":    "993",
		"SMTP_PORT":    "587",
		"EMPTY_VALUE":  "",
		"WITH_QUOTE":   `has"quote`,
		"WITH_NEWLINE": `line1\nline2`,
		"WITH_DOLLAR":  "$HOME",
	}
	if err := WriteEnvFile(envPath, env); err != nil {
		t.Fatalf("WriteEnvFile: %v", err)
	}

	raw, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")

	// 按字母序排序
	wantOrder := []string{"EMPTY_VALUE", "IMAP_PORT", "SMTP_HOST", "SMTP_PORT", "WITH_DOLLAR", "WITH_NEWLINE", "WITH_QUOTE"}
	if len(lines) != len(wantOrder) {
		t.Fatalf("want %d lines, got %d:\n%s", len(wantOrder), len(lines), raw)
	}
	for i, k := range wantOrder {
		var expected string
		switch k {
		case "EMPTY_VALUE":
			expected = `EMPTY_VALUE=""`
		case "WITH_QUOTE":
			// 双引号必须被转义
			expected = `WITH_QUOTE="has\"quote"`
		case "WITH_NEWLINE":
			// 字面反斜杠+n 保持原样（不解释为转义序列）
			expected = `WITH_NEWLINE="line1\nline2"`
		case "WITH_DOLLAR":
			expected = `WITH_DOLLAR="$HOME"`
		default:
			expected = k + `="` + env[k] + `"`
		}
		if lines[i] != expected {
			t.Errorf("line %d: got %q, want %q", i, lines[i], expected)
		}
	}
}

func TestWriteEnvFile_Atomic_NoTmpLeftBehind(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	if err := WriteEnvFile(envPath, map[string]string{"FOO": "bar"}); err != nil {
		t.Fatalf("WriteEnvFile: %v", err)
	}

	if _, err := os.Stat(envPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("want .tmp to not exist, stat err = %v", err)
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Fatalf("want .env to exist, stat err = %v", err)
	}
}

func TestWriteEnvFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	// 注意：WITH_NEWLINE 用 raw string literal，反斜杠+n 是字面两字符
	original := map[string]string{
		"SMTP_HOST":     "smtp.example.com",
		"WITH_QUOTE":    `has"quote`,
		"WITH_NEWLINE":  `line1\nline2`,
		"WITH_DOLLAR":   "$HOME",
		"IMAP_USERNAME": "user@example.com",
	}
	if err := WriteEnvFile(envPath, original); err != nil {
		t.Fatalf("WriteEnvFile: %v", err)
	}

	got, err := ReadEnvFile(envPath)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	if !reflect.DeepEqual(got, original) {
		t.Errorf("round-trip mismatch:\n got: %v\nwant: %v", got, original)
	}
}
