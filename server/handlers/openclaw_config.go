package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// openclawConfigMu 序列化对 openclaw.json 的 read-modify-write。
// 多用户同时点"绑定"会并发跑 sync,没锁的话两次 write 会互相覆盖(后写的丢前写的)。
// 用全局单锁即可——sync 本身只跑在后台 goroutine,且耗时主要在 sleep + 一次 I/O。
var openclawConfigMu sync.Mutex

// openclawConfigSyncDelay 是 bound 标记被 scanner 看到后,等待多久才 sync openclaw 配置。
// 给 openclaw 写 accounts.json 留缓冲(openclaw 标记"已连上微信"和落盘 bot IDs
// 之间可能有 ~1s 延迟,2s 留够 buffer)。
//
// 测试覆盖此值为 0,避免测试跑 2s 才看到 Bound=true(已有 bound-true 检测测试不关心 sync)。
var openclawConfigSyncDelay = 2 * time.Second

// scheduleOpenclawConfigSync 在 scanner 检测到 bound 标记后异步调用:
//  1. 等 openclawConfigSyncDelay(给 openclaw 落 accounts.json 留时间)
//  2. 调 syncOpenclawConfig 把新 bot IDs 写到 openclaw.json
//  3. 无论 sync 成功/失败,都把 Bound=true(因为 openclaw 已经连上微信,
//     sync 只是把 bot IDs 登记到配置,sync 失败也需要人工介入但不影响"绑定成功"事实)
//
// 为什么 Bound 要等 sync 完成才置 true:
//   - 用户原话"这个步骤操作完后，在返回给前端，绑定完成"
//   - 如果 sync 失败但 Bound=true,前端会关掉 modal,但配置实际没同步,
//     下次启动 wechat agent 时找不到 binding 会失败——这种情况对用户更难排查
//   - sync 通常 < 100ms,用户感知的"绑定完成"延迟只多了 2s,可以接受
func scheduleOpenclawConfigSync(taskID string, baseDir string) {
	time.Sleep(openclawConfigSyncDelay)
	added, syncErr := syncOpenclawConfig(baseDir)
	if syncErr != nil {
		L.Error("openclaw config sync failed",
			zap.String("task_id", taskID),
			zap.Error(syncErr),
		)
	}
	GetWechatTaskStore().Update(taskID, func(t *WechatTask) {
		if syncErr != nil {
			t.SyncError = syncErr.Error()
		} else {
			t.SyncError = ""
		}
		t.Bound = true
		L.Info("wechat bind: bound=true after config sync",
			zap.String("task_id", taskID),
			zap.Strings("added", added),
			zap.Bool("sync_ok", syncErr == nil),
		)
	})
}

// syncOpenclawConfig 从 <baseDir>/openclaw-weixin/accounts.json 读 bot IDs,
// 同步到 <baseDir>/openclaw.json 的 bindings 和 channels.openclaw-weixin.accounts。
//
// 行为约定:
//   - accounts.json 不存在 → 视为"还没产生 bot",无操作(nil, nil)
//   - accounts.json 是空数组 / 全是已存在 bot → 无操作(nil, nil),文件不重写
//   - accounts.json 解析失败(不是 JSON 数组) → 返回 error
//   - openclaw.json 不存在 / 解析失败 → 返回 error
//   - bot ID 已在 openclaw-weixin binding 里 → 跳过 binding
//   - bot ID 已在 channels.openclaw-weixin.accounts 里 → 跳过 account
//   - 其他内容(gateway / agents / channels.wecom / meta / tools)全程维持原样
//     (json.MarshalIndent 不会丢字段,但 map key 会按字母序重排,这是 JSON 语义上等价的)
//
// 写盘用 atomic write:先写 <path>.tmp,再 rename 到 <path>;失败时清理 .tmp。
// 没有任何新 bot 时直接 return,不写盘——节省 IO,也保留文件 mtime 不变。
//
// 返回 added = 本次新加的 bot IDs(按 accounts.json 首次出现顺序,去重)。
func syncOpenclawConfig(baseDir string) (added []string, err error) {
	openclawConfigMu.Lock()
	defer openclawConfigMu.Unlock()

	accountsPath := filepath.Join(baseDir, "openclaw-weixin", "accounts.json")
	configPath := filepath.Join(baseDir, "openclaw.json")

	// 1. 读 accounts.json
	botIDs, err := readOpenclawWeixinBotIDs(accountsPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", accountsPath, err)
	}
	if len(botIDs) == 0 {
		return nil, nil
	}

	// 2. 读 openclaw.json
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", configPath, err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse %s: %w", configPath, err)
	}

	// 3. 增量更新 bindings + accounts,获取 added 列表
	newAdded := addBotIDsToOpenclawConfig(config, botIDs)
	if len(newAdded) == 0 {
		return nil, nil
	}

	// 4. 原子写回
	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", configPath, err)
	}
	out = append(out, '\n')

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("rename %s -> %s: %w", tmpPath, configPath, err)
	}

	L.Info("openclaw config synced",
		zap.Strings("added", newAdded),
		zap.String("config", configPath),
	)
	return newAdded, nil
}

// readOpenclawWeixinBotIDs 读 accounts.json 并返回非空 bot ID 列表。
// 文件不存在 → (nil, nil),视为"还没产生 bot"。
// 必须是 JSON 字符串数组(每个元素一个 bot ID);其他格式或解析失败 → error。
func readOpenclawWeixinBotIDs(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("expected JSON array of bot ID strings: %w", err)
	}
	// 过滤空字符串(防御性:openclaw 偶尔会写脏数据)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			out = append(out, id)
		}
	}
	return out, nil
}

// addBotIDsToOpenclawConfig 把 botIDs 里"还没在 config 里"的加到:
//   - bindings:追加 channel=openclaw-weixin 的新条目
//   - channels.openclaw-weixin.accounts:加 "<id>": {} 键值
//
// 已存在于 binding 或 accounts 的 ID 各自独立判断(可能只在一处存在)。
// 返回 added = 本次新加的 bot IDs(去重,保持 botIDs 首次出现顺序)。
//
// 其他 bindings(wecom 等)和 channels(wecom 等)不被改动。
// 顶层非 bindings/channels 的字段(gateway / agents / models / meta / tools 等)也保留。
func addBotIDsToOpenclawConfig(config map[string]any, botIDs []string) []string {
	existingInBindings := collectOpenclawWeixinBindings(config)
	existingInAccounts := collectOpenclawWeixinAccounts(config)

	var added []string
	seen := make(map[string]bool, len(botIDs))

	for _, id := range botIDs {
		if seen[id] {
			continue
		}
		seen[id] = true

		needBinding := !existingInBindings[id]
		needAccount := !existingInAccounts[id]
		if !needBinding && !needAccount {
			continue
		}
		if needBinding {
			appendOpenclawWeixinBinding(config, id)
			existingInBindings[id] = true // 防止 botIDs 内重复触发再加
		}
		if needAccount {
			ensureOpenclawWeixinAccountsMap(config)[id] = map[string]any{}
			existingInAccounts[id] = true
		}
		added = append(added, id)
	}
	return added
}

// collectOpenclawWeixinBindings 返回 config.bindings 里所有 channel=openclaw-weixin 的 accountId 集合。
// bindings 不是数组 / 元素不是对象 / match 不是对象 / accountId 不是字符串 → 忽略该条目。
// channel 是其他值(wecom 等) → 不计入(那些是别的通道,不影响 openclaw-weixin 的去重判断)。
func collectOpenclawWeixinBindings(config map[string]any) map[string]bool {
	out := map[string]bool{}
	bindings, ok := config["bindings"].([]any)
	if !ok {
		return out
	}
	for _, b := range bindings {
		bMap, ok := b.(map[string]any)
		if !ok {
			continue
		}
		match, ok := bMap["match"].(map[string]any)
		if !ok {
			continue
		}
		if match["channel"] != "openclaw-weixin" {
			continue
		}
		id, ok := match["accountId"].(string)
		if !ok || id == "" {
			continue
		}
		out[id] = true
	}
	return out
}

// collectOpenclawWeixinAccounts 返回 channels.openclaw-weixin.accounts 的所有 key。
// 路径上任何一层缺失或类型不对 → 视为空集合(首次 sync 时正常路径)。
func collectOpenclawWeixinAccounts(config map[string]any) map[string]bool {
	out := map[string]bool{}
	channels, ok := config["channels"].(map[string]any)
	if !ok {
		return out
	}
	openclawWeixin, ok := channels["openclaw-weixin"].(map[string]any)
	if !ok {
		return out
	}
	accounts, ok := openclawWeixin["accounts"].(map[string]any)
	if !ok {
		return out
	}
	for id := range accounts {
		out[id] = true
	}
	return out
}

// appendOpenclawWeixinBinding 在 config.bindings 末尾追加一条 openclaw-weixin binding。
// bindings 不是数组 → 替换为新数组(并保留新条目)。
// 格式严格按用户要求:{"agentId":"wechat","match":{"channel":"openclaw-weixin","accountId":<id>}}
func appendOpenclawWeixinBinding(config map[string]any, accountID string) {
	newBinding := map[string]any{
		"agentId": "wechat",
		"match": map[string]any{
			"channel":   "openclaw-weixin",
			"accountId": accountID,
		},
	}
	bindings, ok := config["bindings"].([]any)
	if !ok {
		bindings = []any{}
	}
	bindings = append(bindings, newBinding)
	config["bindings"] = bindings
}

// ensureOpenclawWeixinAccountsMap 确保 config.channels.openclaw-weixin.accounts 存在,
// 返回 accounts map 引用。惰性创建中间缺失的层(理论上不会,生产 openclaw.json 必有 channels,
// 但防御性写法避免 sync 把整个文件结构搞坏)。
func ensureOpenclawWeixinAccountsMap(config map[string]any) map[string]any {
	channels, ok := config["channels"].(map[string]any)
	if !ok {
		channels = map[string]any{}
		config["channels"] = channels
	}
	openclawWeixin, ok := channels["openclaw-weixin"].(map[string]any)
	if !ok {
		openclawWeixin = map[string]any{}
		channels["openclaw-weixin"] = openclawWeixin
	}
	accounts, ok := openclawWeixin["accounts"].(map[string]any)
	if !ok {
		accounts = map[string]any{}
		openclawWeixin["accounts"] = accounts
	}
	return accounts
}