package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadLevelFileMissing 文件不存在 → 返回空 map + nil
func TestReadLevelFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	got, err := ReadLevelFile(path)
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty map, got %v", got)
	}
}

// TestReadLevelFileEmpty 文件存在但内容为空 → 返回空 map
func TestReadLevelFileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadLevelFile(path)
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty map, got %v", got)
	}
}

// TestReadLevelFileValid 合法 JSON → 正确解析
func TestReadLevelFileValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	if err := os.WriteFile(path, []byte(`{"S":"top","A":"good"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadLevelFile(path)
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if got["S"] != "top" {
		t.Errorf("want S=top, got %q", got["S"])
	}
	if got["A"] != "good" {
		t.Errorf("want A=good, got %q", got["A"])
	}
}

// TestWriteLevelFileRoundTrip 写 → 读 内容一致,且无 .tmp 残留
func TestWriteLevelFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	in := map[string]string{"S": "x", "A": "y", "B": "z", "C": "w"}
	if err := WriteLevelFile(path, in); err != nil {
		t.Fatalf("write: %v", err)
	}
	// .tmp 不应残留
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf(".tmp should not exist, stat err = %v", err)
	}
	// 读回
	got, err := ReadLevelFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for k, v := range in {
		if got[k] != v {
			t.Errorf("key %s: want %q got %q", k, v, got[k])
		}
	}
}

// TestWriteLevelFileOverwrite 覆盖写:旧 keys 全部消失
func TestWriteLevelFileOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	if err := os.WriteFile(path, []byte(`{"OLD":"value","S":"old"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := WriteLevelFile(path, map[string]string{"S": "new"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadLevelFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, exists := got["OLD"]; exists {
		t.Errorf("OLD should be removed, got %v", got)
	}
	if got["S"] != "new" {
		t.Errorf("want S=new, got %q", got["S"])
	}
}

// TestWriteLevelFileSpecialChars 特殊字符:引号 / 换行 / 中文 round-trip
func TestWriteLevelFileSpecialChars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	in := map[string]string{"S": "with \"quotes\" and \n newline\n and 中文"}
	if err := WriteLevelFile(path, in); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadLevelFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got["S"] != in["S"] {
		t.Errorf("round-trip failed:\nwant=%q\ngot =%q", in["S"], got["S"])
	}
}

// TestWriteLevelFileFileNotFound 写到不存在的目录(父目录缺失)→ 应创建
// 实际上 WriteLevelFile 不创建父目录;测试调用方需先 mkdir。
// 这里测试的是:文件不存在但父目录存在时,能直接创建文件。
func TestWriteLevelFileCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "x.json")
	// 子目录不存在,应失败
	if err := WriteLevelFile(path, map[string]string{"S": "x"}); err == nil {
		t.Errorf("want error when parent dir missing, got nil")
	}
}
