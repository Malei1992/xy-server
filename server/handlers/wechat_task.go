package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// TaskStatus 微信绑定任务生命周期状态。
//  1. pending  : POST 已建 task,docker 还没起
//  2. running  : docker exec 正在跑
//  3. done     : exit 0,且 link/qr 至少有一个
//  4. failed   : docker 启动失败 / 退出非 0
//  5. expired  : 超时未结束
type TaskStatus string

const (
	StatusPending TaskStatus = "pending"
	StatusRunning TaskStatus = "running"
	StatusDone    TaskStatus = "done"
	StatusFailed  TaskStatus = "failed"
	StatusExpired TaskStatus = "expired"
)

// WechatTask 一次微信绑定任务的全量状态。POST 立刻建一个(pending),
// 后台 goroutine 推进 status 并填充 link/qr/raw/error/expired/bound。
//
// Bound:openclaw 成功连接微信后(stdout 出现「已将此 OpenClaw 连接到微信。」)
// 后台 goroutine 在 scanner 循环里置 true,前端用来切到「绑定成功」状态;
// 早期发布,不等 cmd 退出就能让前端知道。
//
// cancel:context cancel 函数,POST cancel 端点用它来 SIGKILL 整个 exec 进程组。
// 不暴露给 JSON,因为它仅用于进程内同步。
//
// CompletedAt 在状态进入终态(done/failed/expired)时打点,TTL 清理用它作为基准;
// Active 期间为零值。
//
// 字段并发安全:
//   - store 持有 *WechatTask,生命周期内不会替换
//   - 后台 goroutine 通过 Update 写,GET handler 通过 GetSnapshot 读
//   - 锁是单独的 *taskLocker 指针,不在 struct 体内,这样 snapshot 可以安全做
//     `cp := *t`(只拷贝数据字段,锁是指针共享)而不会触发 race detector
type WechatTask struct {
	locker      *taskLocker
	TaskID      string     `json:"task_id"`
	Status      TaskStatus `json:"status"`
	Link        string     `json:"link"`
	QR          string     `json:"qr"`
	Raw         string     `json:"raw"`
	Expired     bool       `json:"expired"`
	Bound       bool       `json:"bound,omitempty"`
	Error       string     `json:"error,omitempty"`
	CompletedAt time.Time  `json:"-"`
	cancel      context.CancelFunc
}

// taskLocker 把 mutex 单独抽出来放堆上,避免嵌入 WechatTask 后值拷贝触发
// race detector("copies of locks")。每个 task 一个。
type taskLocker struct {
	mu sync.Mutex
}

// newWechatTask 构造一个新任务并自带独立锁。
func newWechatTask() *WechatTask {
	return &WechatTask{locker: &taskLocker{}}
}

// WechatTaskStore 进程内单例,包 sync.Map 保证并发安全。
// ttl 是终态任务的存活时间;后台 goroutine 每 30s 扫一遍删除到期项。
//
// 用 sync.Map 而非 map+Mutex 是因为读多写少,且 task 数有限,
// 性能不是瓶颈,简单可靠优先。
type WechatTaskStore struct {
	m   sync.Map
	ttl time.Duration
}

var (
	wechatStoreOnce sync.Once
	wechatStore     *WechatTaskStore
)

// GetWechatTaskStore 进程内单例,懒初始化。
// ttl 固定 5 分钟,后台清理 goroutine 仅启动一次。
// 测试可调用 ResetWechatTaskStoreForTest 重置。
func GetWechatTaskStore() *WechatTaskStore {
	wechatStoreOnce.Do(func() {
		wechatStore = newWechatTaskStore(5 * time.Minute)
	})
	return wechatStore
}

func newWechatTaskStore(ttl time.Duration) *WechatTaskStore {
	s := &WechatTaskStore{ttl: ttl}
	go s.runCleanupLoop()
	return s
}

// newTaskID 生成 wt- 前缀 + 12 hex 字符的 id。
// crypto/rand 6 字节熵 48 bit,2^48 空间,撞 id 概率可忽略。
func newTaskID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand 在现代系统上不会失败;退而用时间戳作兜底 id 也只是 5 分钟 TTL,影响小。
		// 不过 Go 1.24 文档保证 rand.Read 不会返回非 nil err(无熵源就 panic),这里几乎走不到。
		return "wt-" + time.Now().UTC().Format("150405.000000")
	}
	return "wt-" + hex.EncodeToString(b[:])
}

// Add 新建一个 pending 任务并存入 store,返回 task_id。
// 调用方不需要预分配 *WechatTask,store 内部 new 出来并设 Status=pending。
// 返回的 *WechatTask 是 store 内同一份指针,改它必须走 Update,否则会触发 race。
func (s *WechatTaskStore) Add() (string, *WechatTask) {
	t := newWechatTask()
	t.Status = StatusPending
	id := newTaskID()
	t.TaskID = id
	s.m.Store(id, t)
	return id, t
}

// Get 原子读取。返回 (task, true) 表示存在,否则为 (nil, false)。
// 返回的是 store 里的 *WechatTask 引用,只能用于需要指针语义的场景;
// 外部读字段请用 GetSnapshot,避免 data race。
func (s *WechatTaskStore) Get(id string) (*WechatTask, bool) {
	v, ok := s.m.Load(id)
	if !ok {
		return nil, false
	}
	t, ok := v.(*WechatTask)
	return t, ok
}

// GetSnapshot 原子读取并立即值拷贝,返回的 *WechatTask 跟 store 完全脱钩,
// 后续后台 goroutine 改原 task 不会影响这个副本。GET handler 走这个,
// 避免 gin JSON 序列化时跟 Update 写并发读写字段触发 race detector。
//
// 锁不在 WechatTask 体内(是 *taskLocker 指针),所以 `cp := *t` 安全:只复制了
// 数据字段,锁对象是堆上同一份,Update 和 GetSnapshot 走同一把锁。
func (s *WechatTaskStore) GetSnapshot(id string) (*WechatTask, bool) {
	v, ok := s.m.Load(id)
	if !ok {
		return nil, false
	}
	t, ok := v.(*WechatTask)
	if !ok {
		return nil, false
	}
	t.locker.mu.Lock()
	cp := *t
	// cp.locker 也指向同一把锁,外部使用 cp 不会再加锁,清空避免误用
	cp.locker = nil
	t.locker.mu.Unlock()
	return &cp, true
}

// Update 原子修改(Load → 加锁 → 改字段 → Store)。
// 加锁是必要的:GetSnapshot 也读同一组字段,不加锁 race detector 必报。
// 一个 task 同时只会有一个 Update 调用方(其对应 goroutine),所以锁竞争只来自
// 并发的 GetSnapshot 读,不会写写竞争。
func (s *WechatTaskStore) Update(id string, mutator func(*WechatTask)) {
	for i := 0; i < 3; i++ {
		v, ok := s.m.Load(id)
		if !ok {
			return
		}
		t, ok := v.(*WechatTask)
		if !ok {
			return
		}
		t.locker.mu.Lock()
		mutator(t)
		t.locker.mu.Unlock()
		s.m.Store(id, t)
		return
	}
}

// runCleanupLoop 每 30s 扫一次,清理 ttl 之前完成的 task。
// 启动即跑一次,生产环境冷启动不积压。
func (s *WechatTaskStore) runCleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	s.cleanupOnce()
	for range ticker.C {
		s.cleanupOnce()
	}
}

func (s *WechatTaskStore) cleanupOnce() {
	now := time.Now()
	s.m.Range(func(k, v any) bool {
		t, ok := v.(*WechatTask)
		if !ok {
			return true
		}
		t.locker.mu.Lock()
		completed := t.CompletedAt
		t.locker.mu.Unlock()
		if completed.IsZero() {
			// 还在跑,不删
			return true
		}
		if now.Sub(completed) > s.ttl {
			s.m.Delete(k)
		}
		return true
	})
}

// ResetWechatTaskStoreForTest 清空 store 并重置 once,让测试拿到干净的单例。
// 不停止后台 goroutine(下次 newWechatTaskStore 会启动新的,旧的 ticker 退出由进程结束处理)。
func ResetWechatTaskStoreForTest() {
	wechatStoreOnce = sync.Once{}
	wechatStore = nil
}
