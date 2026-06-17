package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// L 是全局 logger。main.go 在启动时 InitLogger,业务代码用 handlers.L.Info(...)。
// 初始为 Nop logger 避免未 InitLogger 时 nil panic（测试用 handlerForTest 时常见）。
var L *zap.Logger = zap.NewNop()

func init() {
	// 确保 L 非 nil,即使 InitLogger 未被调用
	if L == nil {
		L = zap.NewNop()
	}
}

// CST 偏移常量 (秒) — 中国标准时间 UTC+8。
const cstOffsetSeconds = 8 * 3600

// cstZone 全局时区对象,用于 EncodeTime。
var cstZone = time.FixedZone("CST", cstOffsetSeconds)

// dayFormat 是日志文件名的日期格式 (yyyy-mm-dd)。
const dayFormat = "2006-01-02"

// InitLogger 在 logsDir 下打开当天的 app.YYYY-MM-DD.log,
// 构造一个带 caller 的 JSON zap logger 赋给全局 L。
// logsDir 必须是绝对路径,目录不存在会自动创建。
// 失败返回 error 由 main.go 决定是否 log.Fatal。
func InitLogger(logsDir string) error {
	if !filepath.IsAbs(logsDir) {
		return fmt.Errorf("logsDir 必须为绝对路径,实际 %q", logsDir)
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", logsDir, err)
	}

	w := newDailyWriter(logsDir)
	_globalWriter = w

	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "ts"
	cfg.MessageKey = "msg"
	cfg.LevelKey = "level"
	cfg.CallerKey = "caller"
	cfg.StacktraceKey = "stack"
	cfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.In(cstZone).Format("2006-01-02T15:04:05.000Z07:00"))
	}
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.EncodeDuration = zapcore.MillisDurationEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(cfg),
		zapcore.AddSync(w),
		zap.InfoLevel,
	)
	L = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(0))
	return nil
}

// dailyWriter 是 io.Writer:写入时按当天日期选文件,跨天自动切换。
// 切日时启动一个 goroutine 异步清理 >30 天的旧日志。
type dailyWriter struct {
	mu      sync.Mutex
	dir     string
	curDay  string // yyyy-mm-dd
	file    *os.File
	nowFunc func() time.Time // 测试可注入
}

func newDailyWriter(dir string) *dailyWriter {
	return &dailyWriter{dir: dir}
}

// Write 是 io.Writer 接口。
func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	if w.nowFunc != nil {
		now = w.nowFunc()
	}
	today := now.In(cstZone).Format(dayFormat)
	if today != w.curDay {
		if w.file != nil {
			_ = w.file.Close()
		}
		path := filepath.Join(w.dir, "app."+today+".log")
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return 0, err
		}
		w.file = f
		w.curDay = today
		// 异步清理 30 天前日志,不阻塞写入
		go cleanOldLogs(w.dir, 30)
	}
	return w.file.Write(p)
}

// Sync 刷盘。
func (w *dailyWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Sync()
}

// cleanOldLogs 删除 dir 下 mtime > maxAgeDays 天的 app.YYYY-MM-DD.log。
// 不存在的目录直接返回,无错误。
func cleanOldLogs(dir string, maxAgeDays int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-time.Duration(maxAgeDays) * 24 * time.Hour)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "app.") || !strings.HasSuffix(name, ".log") {
			continue
		}
		full := filepath.Join(dir, name)
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(full)
		}
	}
}

// ===== 测试钩子 =====

// ResetLoggerForTest 把 L 替换成 Nop logger 并关闭底层文件,避免跨测试污染。
// 由测试 t.Cleanup 调用。
func ResetLoggerForTest() {
	if L != nil {
		_ = L.Sync()
	}
	L = zap.NewNop()
}

// ForceDayForTest 把全局 dailyWriter 的 nowFunc 设为返回 t,模拟"今天是 t 的日期"。
// 下一次 Write 会用 t 的日期作为 today,触发切日逻辑。
func ForceDayForTest(t time.Time) {
	if w, ok := getGlobalWriter(); ok {
		w.mu.Lock()
		w.nowFunc = func() time.Time { return t }
		w.mu.Unlock()
	}
}

// CleanOldLogsForTest 暴露 cleanOldLogs 给测试调用。
func CleanOldLogsForTest(dir string, maxAgeDays int) {
	cleanOldLogs(dir, maxAgeDays)
}

// _globalWriter 持有 InitLogger 创建的 dailyWriter 指针,供测试钩子访问。
var _globalWriter *dailyWriter

func getGlobalWriter() (*dailyWriter, bool) {
	return _globalWriter, _globalWriter != nil
}
