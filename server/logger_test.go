package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"crm-server/handlers"
)

// 这些测试覆盖 handlers/logger.go 的核心行为：
//   - InitLogger 创建 logs/ 目录 + 当天 app.YYYY-MM-DD.log
//   - 写入的日志行含 ts / level / caller / msg
//   - ts 用 CST (UTC+8) 时间格式
//   - 默认 Info 级：Info 落盘、Debug 不落盘
//   - dailyWriter 跨天切换文件
//   - cleanOldLogs 删除 >30 天的旧文件

const testCSTOffset = 8 * 3600 // Asia/Shanghai

// cstTime 返回 t 转换到 UTC+8 后的 yyyy-mm-dd。
func cstTime(t time.Time) string {
	return t.In(time.FixedZone("CST", testCSTOffset)).Format("2006-01-02")
}

// readLastLine 读文件最后一行非空内容。
func readLastLine(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	return ""
}

// findLogContaining 在 logsDir 下找第一个文件内容含 needle 的 .log 文件。
// 找不到时 t.Fatal。
func findLogContaining(t *testing.T, logsDir, needle string) string {
	t.Helper()
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("readdir %s: %v", logsDir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		p := filepath.Join(logsDir, e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if strings.Contains(string(data), needle) {
			return p
		}
	}
	t.Fatalf("logsDir %s 下找不到含 %q 的日志文件", logsDir, needle)
	return ""
}

// 启动 logger 到 t.TempDir()（logsDir 绝对路径），函数返回 logsDir 路径。
// t.Cleanup 会把全局 L 替换为 Nop,避免污染其他测试。
func setupLogger(t *testing.T) string {
	t.Helper()
	logsDir := t.TempDir()
	if err := handlers.InitLogger(logsDir); err != nil {
		t.Fatalf("InitLogger: %v", err)
	}
	t.Cleanup(handlers.ResetLoggerForTest)
	return logsDir
}

func TestInitLogger_CreatesFile(t *testing.T) {
	logsDir := setupLogger(t)
	today := cstTime(time.Now())
	logFile := filepath.Join(logsDir, "app."+today+".log")

	// 触发一次写入
	handlers.L.Info("boot check", zap.String("k", "v"))

	// 关闭 Sync 让数据刷盘
	if err := handlers.L.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	if _, err := os.Stat(logFile); err != nil {
		t.Fatalf("期望日志文件 %s 存在: %v", logFile, err)
	}
}

func TestLoggerCallerHasFileAndLine(t *testing.T) {
	logsDir := setupLogger(t)
	// 标记本测试的位置:logger_test.go:LINE,断言 caller 字段中包含此文件名
	handlers.L.Info("caller probe")
	_ = handlers.L.Sync()

	found := findLogContaining(t, logsDir, "caller probe")
	last := readLastLine(t, found)
	var entry map[string]any
	if err := json.Unmarshal([]byte(last), &entry); err != nil {
		t.Fatalf("日志行不是合法 JSON: %q (%v)", last, err)
	}
	caller, ok := entry["caller"].(string)
	if !ok {
		t.Fatalf("日志缺 caller 字段: %v", entry)
	}
	if !strings.Contains(caller, "logger_test.go") {
		t.Errorf("caller 字段应包含 logger_test.go,实际 %q", caller)
	}
	// 必须含行号 (logger_test.go:NN)
	re := regexp.MustCompile(`logger_test\.go:\d+`)
	if !re.MatchString(caller) {
		t.Errorf("caller 字段应含 logger_test.go:行号,实际 %q", caller)
	}
}

func TestLoggerTimeFormat(t *testing.T) {
	logsDir := setupLogger(t)
	handlers.L.Info("time check")
	_ = handlers.L.Sync()

	found := findLogContaining(t, logsDir, "time check")
	last := readLastLine(t, found)
	var entry map[string]any
	if err := json.Unmarshal([]byte(last), &entry); err != nil {
		t.Fatalf("日志行不是合法 JSON: %q (%v)", last, err)
	}
	ts, _ := entry["ts"].(string)
	// 期望: 2026-06-17T14:23:01.234+08:00
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+08:00$`)
	if !re.MatchString(ts) {
		t.Errorf("ts 字段应匹配 RFC3339 CST 格式,实际 %q", ts)
	}
}

func TestLoggerInfoLevel(t *testing.T) {
	logsDir := setupLogger(t)
	handlers.L.Info("info-msg")
	handlers.L.Debug("debug-msg-should-not-appear")
	_ = handlers.L.Sync()

	found := findLogContaining(t, logsDir, "info-msg")
	data, _ := os.ReadFile(found)
	if strings.Contains(string(data), "debug-msg-should-not-appear") {
		t.Errorf("Debug 级别不应落盘,但文件中有: %s", found)
	}
}

func TestDailyRotation(t *testing.T) {
	logsDir := t.TempDir()
	if err := handlers.InitLogger(logsDir); err != nil {
		t.Fatalf("InitLogger: %v", err)
	}
	t.Cleanup(handlers.ResetLoggerForTest)

	// 第一次写入 (今天)
	handlers.L.Info("day1")
	_ = handlers.L.Sync()

	// 把 nowFunc 设为明天,模拟跨天
	tomorrow := time.Now().Add(24 * time.Hour)
	handlers.ForceDayForTest(tomorrow)

	handlers.L.Info("day2")
	_ = handlers.L.Sync()

	day1 := filepath.Join(logsDir, "app."+cstTime(time.Now())+".log")
	day2 := filepath.Join(logsDir, "app."+cstTime(tomorrow)+".log")

	day1Data, err := os.ReadFile(day1)
	if err != nil {
		t.Fatalf("day1 文件不存在: %v", err)
	}
	day2Data, err := os.ReadFile(day2)
	if err != nil {
		t.Fatalf("day2 文件不存在: %v", err)
	}
	if !strings.Contains(string(day1Data), "day1") {
		t.Errorf("day1 文件应含 day1")
	}
	if strings.Contains(string(day1Data), "day2") {
		t.Errorf("day1 文件不应含 day2 (切日后写入)")
	}
	if !strings.Contains(string(day2Data), "day2") {
		t.Errorf("day2 文件应含 day2")
	}
	if strings.Contains(string(day2Data), "day1") {
		t.Errorf("day2 文件不应含 day1")
	}
}

func TestInitLogger_RejectsRelativePath(t *testing.T) {
	// 相对路径应直接报错,避免静默写到未知目录
	if err := handlers.InitLogger("relative/logs"); err == nil {
		t.Fatal("期望 InitLogger(相对路径) 报错,实际 nil")
	}
}

func TestCleanOldLogs(t *testing.T) {
	logsDir := t.TempDir()

	// 造 35 天前 + 1 天前 + 今天 三个文件
	old := filepath.Join(logsDir, "app."+cstTime(time.Now().Add(-35*24*time.Hour))+".log")
	recent := filepath.Join(logsDir, "app."+cstTime(time.Now().Add(-24*time.Hour))+".log")
	today := filepath.Join(logsDir, "app."+cstTime(time.Now())+".log")
	for _, p := range []string{old, recent, today} {
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	// 修改 mtime 让 35 天前那个文件 mtime 真的是 35 天前,recent 是 1 天前
	past35 := time.Now().Add(-35 * 24 * time.Hour)
	past1 := time.Now().Add(-24 * time.Hour)
	_ = os.Chtimes(old, past35, past35)
	_ = os.Chtimes(recent, past1, past1)

	// 调用 CleanOldLogsForTest
	handlers.CleanOldLogsForTest(logsDir, 30)

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("35 天前的文件应被删除,实际: %v", err)
	}
	if _, err := os.Stat(recent); err != nil {
		t.Errorf("1 天前的文件应保留,实际: %v", err)
	}
	if _, err := os.Stat(today); err != nil {
		t.Errorf("今天的文件应保留,实际: %v", err)
	}
}
