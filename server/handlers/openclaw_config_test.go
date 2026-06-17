package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// sampleOpenclawConfig 是 mock openclaw.json 的最小可用版本。
// 包含 1 个 openclaw-weixin binding、1 个 wecom binding、1 个 openclaw-weixin account、
// 1 个 wecom account;再加 gateway / agents / meta / tools 等"其他内容"用于验证不被破坏。
const sampleOpenclawConfig = `{
  "gateway": {
    "mode": "local",
    "bind": "${GATEWAY_BIND}",
    "auth": {
      "mode": "token",
      "token": "${OPENCLAW_GATEWAY_TOKEN}"
    }
  },
  "agents": {
    "defaults": {
      "workspace": "/home/node/.openclaw/workspace"
    },
    "list": [
      {
        "id": "main",
        "workspace": "/home/node/.openclaw/workspace/main"
      },
      {
        "id": "wechat",
        "workspace": "/home/node/.openclaw/workspace/wechat"
      }
    ]
  },
  "bindings": [
    {
      "agentId": "wechat",
      "match": {
        "channel": "openclaw-weixin",
        "accountId": "existing-bot-im-bot"
      }
    },
    {
      "agentId": "wechat",
      "match": {
        "channel": "wecom",
        "accountId": "bot1"
      }
    }
  ],
  "channels": {
    "openclaw-weixin": {
      "enabled": true,
      "accounts": {
        "existing-bot-im-bot": {}
      }
    },
    "wecom": {
      "enabled": true,
      "accounts": {
        "bot1": {
          "botId": "secret-bot-id"
        }
      }
    }
  },
  "meta": {
    "lastTouchedVersion": "2026.5.12",
    "lastTouchedAt": "2026-06-04T07:44:43.572Z"
  },
  "tools": {
    "alsoAllow": ["wecom_mcp"]
  }
}`

// mockOpenclawConfig 在临时目录创建 accounts.json 和 openclaw.json。
// 返回 baseDir(用于 syncOpenclawConfig)。
func mockOpenclawConfig(t *testing.T, accountsContent, openclawContent string) string {
	t.Helper()
	baseDir := t.TempDir()

	accountsDir := filepath.Join(baseDir, "openclaw-weixin")
	if err := os.MkdirAll(accountsDir, 0o755); err != nil {
		t.Fatalf("mkdir accounts dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(accountsDir, "accounts.json"), []byte(accountsContent), 0o644); err != nil {
		t.Fatalf("write accounts.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "openclaw.json"), []byte(openclawContent), 0o644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}
	return baseDir
}

// readOpenclawJSON 读 openclaw.json 解析成 map,供测试断言。
func readOpenclawJSON(t *testing.T, baseDir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(baseDir, "openclaw.json"))
	if err != nil {
		t.Fatalf("read openclaw.json: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("unmarshal openclaw.json: %v", err)
	}
	return config
}

// countOpenclawWeixinBindings 返回 bindings 里 channel=openclaw-weixin 的条目数。
func countOpenclawWeixinBindings(bindings []any) int {
	n := 0
	for _, b := range bindings {
		bMap, _ := b.(map[string]any)
		match, _ := bMap["match"].(map[string]any)
		if match["channel"] == "openclaw-weixin" {
			n++
		}
	}
	return n
}

// hasOpenclawWeixinBinding 检查 bindings 里是否有 channel=openclaw-weixin + accountId=id 的条目。
func hasOpenclawWeixinBinding(bindings []any, accountID string) bool {
	for _, b := range bindings {
		bMap, _ := b.(map[string]any)
		match, _ := bMap["match"].(map[string]any)
		if match["channel"] == "openclaw-weixin" && match["accountId"] == accountID {
			return true
		}
	}
	return false
}

// ===== 测试 =====

// TestSyncOpenclawConfig_NoNewBots accounts.json 里只有已有 bot → 不加任何东西,文件保持不变
func TestSyncOpenclawConfig_NoNewBots(t *testing.T) {
	baseDir := mockOpenclawConfig(t, `["existing-bot-im-bot"]`, sampleOpenclawConfig)

	added, err := syncOpenclawConfig(baseDir)
	if err != nil {
		t.Fatalf("syncOpenclawConfig: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("added = %v, want empty", added)
	}

	// 文件应保持原样(2 个 bindings 不变)
	cfg := readOpenclawJSON(t, baseDir)
	bindings, _ := cfg["bindings"].([]any)
	if len(bindings) != 2 {
		t.Errorf("bindings count = %d, want 2 (unchanged)", len(bindings))
	}
	channels, _ := cfg["channels"].(map[string]any)
	ow, _ := channels["openclaw-weixin"].(map[string]any)
	accounts, _ := ow["accounts"].(map[string]any)
	if len(accounts) != 1 {
		t.Errorf("accounts count = %d, want 1 (unchanged)", len(accounts))
	}
}

// TestSyncOpenclawConfig_AddsNewBots accounts.json 有新 bot → bindings 和 accounts 都加
func TestSyncOpenclawConfig_AddsNewBots(t *testing.T) {
	baseDir := mockOpenclawConfig(t,
		`["existing-bot-im-bot", "new-bot1-im-bot", "new-bot2-im-bot"]`,
		sampleOpenclawConfig,
	)

	added, err := syncOpenclawConfig(baseDir)
	if err != nil {
		t.Fatalf("syncOpenclawConfig: %v", err)
	}
	if len(added) != 2 {
		t.Errorf("added count = %d, want 2; added=%v", len(added), added)
	}
	// 顺序应与 accounts.json 一致
	wantAdded := []string{"new-bot1-im-bot", "new-bot2-im-bot"}
	for i, id := range wantAdded {
		if i >= len(added) || added[i] != id {
			t.Errorf("added[%d] = %v, want %s", i, added, id)
		}
	}

	cfg := readOpenclawJSON(t, baseDir)
	bindings, _ := cfg["bindings"].([]any)
	// 2 原有 + 2 新加 = 4
	if len(bindings) != 4 {
		t.Errorf("bindings count = %d, want 4", len(bindings))
	}
	// openclaw-weixin bindings 应有 3 条
	if n := countOpenclawWeixinBindings(bindings); n != 3 {
		t.Errorf("openclaw-weixin bindings count = %d, want 3", n)
	}
	// 新 bot 都加进了 bindings
	for _, id := range wantAdded {
		if !hasOpenclawWeixinBinding(bindings, id) {
			t.Errorf("missing binding for %s", id)
		}
	}

	channels, _ := cfg["channels"].(map[string]any)
	ow, _ := channels["openclaw-weixin"].(map[string]any)
	accounts, _ := ow["accounts"].(map[string]any)
	if len(accounts) != 3 {
		t.Errorf("accounts count = %d, want 3", len(accounts))
	}
	for _, id := range []string{"existing-bot-im-bot", "new-bot1-im-bot", "new-bot2-im-bot"} {
		if _, ok := accounts[id]; !ok {
			t.Errorf("missing account %s", id)
		}
	}
}

// TestSyncOpenclawConfig_PreservesOtherContent 其他字段(gateway/agents/meta/wecom)不变
func TestSyncOpenclawConfig_PreservesOtherContent(t *testing.T) {
	baseDir := mockOpenclawConfig(t, `["new-bot-im-bot"]`, sampleOpenclawConfig)

	_, err := syncOpenclawConfig(baseDir)
	if err != nil {
		t.Fatalf("syncOpenclawConfig: %v", err)
	}

	cfg := readOpenclawJSON(t, baseDir)

	// gateway 内容(嵌套)应保留
	gateway, _ := cfg["gateway"].(map[string]any)
	if gateway["mode"] != "local" {
		t.Errorf("gateway.mode = %v, want 'local'", gateway["mode"])
	}
	if gateway["bind"] != "${GATEWAY_BIND}" {
		t.Errorf("gateway.bind = %v, want '${GATEWAY_BIND}'", gateway["bind"])
	}
	auth, _ := gateway["auth"].(map[string]any)
	if auth["token"] != "${OPENCLAW_GATEWAY_TOKEN}" {
		t.Errorf("gateway.auth.token lost: %v", auth["token"])
	}

	// agents 列表保留(2 条)
	agents, _ := cfg["agents"].(map[string]any)
	list, _ := agents["list"].([]any)
	if len(list) != 2 {
		t.Errorf("agents.list count = %d, want 2", len(list))
	}

	// meta 字段保留
	meta, _ := cfg["meta"].(map[string]any)
	if meta["lastTouchedVersion"] != "2026.5.12" {
		t.Errorf("meta.lastTouchedVersion lost: %v", meta["lastTouchedVersion"])
	}

	// tools 保留
	tools, _ := cfg["tools"].(map[string]any)
	alsoAllow, _ := tools["alsoAllow"].([]any)
	if len(alsoAllow) != 1 || alsoAllow[0] != "wecom_mcp" {
		t.Errorf("tools.alsoAllow lost: %v", alsoAllow)
	}

	// wecom channel 完整保留(含 bot1 的 botId)
	channels, _ := cfg["channels"].(map[string]any)
	wecom, _ := channels["wecom"].(map[string]any)
	if wecom == nil {
		t.Fatal("wecom channel lost")
	}
	wecomAccounts, _ := wecom["accounts"].(map[string]any)
	bot1, _ := wecomAccounts["bot1"].(map[string]any)
	if bot1 == nil || bot1["botId"] != "secret-bot-id" {
		t.Errorf("wecom.accounts.bot1.botId lost: %v", bot1)
	}

	// wecom binding 应保留
	bindings, _ := cfg["bindings"].([]any)
	if !hasWecomBinding(bindings, "bot1") {
		t.Errorf("wecom binding for bot1 lost")
	}
}

// hasWecomBinding 检查 bindings 里是否有 channel=wecom + accountId=id 的条目
func hasWecomBinding(bindings []any, accountID string) bool {
	for _, b := range bindings {
		bMap, _ := b.(map[string]any)
		match, _ := bMap["match"].(map[string]any)
		if match["channel"] == "wecom" && match["accountId"] == accountID {
			return true
		}
	}
	return false
}

// TestSyncOpenclawConfig_PartialOverlap 部分 bot 已存在,只加新的
// (binding 在但 account 不在,或反之)
func TestSyncOpenclawConfig_PartialOverlap(t *testing.T) {
	// accounts.json 有一个 bot 只在 bindings 里(没在 accounts map)
	// 模拟"binding 残留但 accounts 缺失"
	openclawContent := `{
  "bindings": [
    {
      "agentId": "wechat",
      "match": {"channel": "openclaw-weixin", "accountId": "old-bind-only"}
    }
  ],
  "channels": {
    "openclaw-weixin": {
      "accounts": {}
    }
  }
}`
	baseDir := mockOpenclawConfig(t, `["old-bind-only", "new-bot-im-bot"]`, openclawContent)

	added, err := syncOpenclawConfig(baseDir)
	if err != nil {
		t.Fatalf("syncOpenclawConfig: %v", err)
	}
	// old-bind-only: 在 bindings 里 → 跳过 binding;不在 accounts → 加 account
	// new-bot-im-bot: 都不在 → 两边都加
	wantAdded := []string{"old-bind-only", "new-bot-im-bot"}
	if len(added) != len(wantAdded) {
		t.Errorf("added count = %d, want %d; added=%v", len(added), len(wantAdded), added)
	}
	for i, id := range wantAdded {
		if i >= len(added) || added[i] != id {
			t.Errorf("added[%d] = %v, want %s", i, added, id)
		}
	}

	cfg := readOpenclawJSON(t, baseDir)
	bindings, _ := cfg["bindings"].([]any)
	// old-bind-only 已在 bindings → 不重复加;new-bot-im-bot 应加
	if len(bindings) != 2 {
		t.Errorf("bindings count = %d, want 2", len(bindings))
	}
	if !hasOpenclawWeixinBinding(bindings, "new-bot-im-bot") {
		t.Errorf("new-bot-im-bot not added to bindings")
	}

	channels, _ := cfg["channels"].(map[string]any)
	ow, _ := channels["openclaw-weixin"].(map[string]any)
	accounts, _ := ow["accounts"].(map[string]any)
	// 两个 bot 都应有 account
	if len(accounts) != 2 {
		t.Errorf("accounts count = %d, want 2", len(accounts))
	}
}

// TestSyncOpenclawConfig_DedupInAccounts accounts.json 内部重复 → 只加一次
func TestSyncOpenclawConfig_DedupInAccounts(t *testing.T) {
	baseDir := mockOpenclawConfig(t, `["dup-bot-im-bot", "dup-bot-im-bot", "new-bot-im-bot"]`, sampleOpenclawConfig)

	added, err := syncOpenclawConfig(baseDir)
	if err != nil {
		t.Fatalf("syncOpenclawConfig: %v", err)
	}
	// dedup → added 应只有 2 条,顺序按 accounts.json 首次出现顺序
	wantAdded := []string{"dup-bot-im-bot", "new-bot-im-bot"}
	if len(added) != len(wantAdded) {
		t.Errorf("added count = %d, want %d; added=%v", len(added), len(wantAdded), added)
	}
	for i, id := range wantAdded {
		if i >= len(added) || added[i] != id {
			t.Errorf("added[%d] = %v, want %s", i, added, id)
		}
	}
}

// TestSyncOpenclawConfig_EmptyAccounts accounts.json 是 [] → 无操作
func TestSyncOpenclawConfig_EmptyAccounts(t *testing.T) {
	baseDir := mockOpenclawConfig(t, `[]`, sampleOpenclawConfig)

	added, err := syncOpenclawConfig(baseDir)
	if err != nil {
		t.Fatalf("syncOpenclawConfig: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("added = %v, want empty", added)
	}
}

// TestSyncOpenclawConfig_MissingAccountsFile accounts.json 不存在 → 不报错(视为"还没产生 bot")
func TestSyncOpenclawConfig_MissingAccountsFile(t *testing.T) {
	// 只建 openclaw.json,openclaw-weixin/ 不建
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "openclaw.json"), []byte(sampleOpenclawConfig), 0o644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	added, err := syncOpenclawConfig(baseDir)
	if err != nil {
		t.Fatalf("syncOpenclawConfig: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("added = %v, want empty", added)
	}
}

// TestSyncOpenclawConfig_MissingOpenclawFile openclaw.json 不存在 → 报错
func TestSyncOpenclawConfig_MissingOpenclawFile(t *testing.T) {
	baseDir := t.TempDir()
	accountsDir := filepath.Join(baseDir, "openclaw-weixin")
	if err := os.MkdirAll(accountsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(accountsDir, "accounts.json"), []byte(`["x"]`), 0o644); err != nil {
		t.Fatalf("write accounts: %v", err)
	}

	_, err := syncOpenclawConfig(baseDir)
	if err == nil {
		t.Fatal("want error for missing openclaw.json, got nil")
	}
}

// TestSyncOpenclawConfig_MalformedAccounts accounts.json 不是合法 JSON → 报错
func TestSyncOpenclawConfig_MalformedAccounts(t *testing.T) {
	baseDir := mockOpenclawConfig(t, `not json`, sampleOpenclawConfig)

	_, err := syncOpenclawConfig(baseDir)
	if err == nil {
		t.Fatal("want error for malformed accounts.json, got nil")
	}
}

// TestSyncOpenclawConfig_AtomicWrite 写文件是原子的(.tmp 不残留,openclaw.json 完整)
func TestSyncOpenclawConfig_AtomicWrite(t *testing.T) {
	baseDir := mockOpenclawConfig(t, `["new-bot-im-bot"]`, sampleOpenclawConfig)
	configPath := filepath.Join(baseDir, "openclaw.json")

	_, err := syncOpenclawConfig(baseDir)
	if err != nil {
		t.Fatalf("syncOpenclawConfig: %v", err)
	}

	// .tmp 不应残留
	if _, err := os.Stat(configPath + ".tmp"); !os.IsNotExist(err) {
		t.Errorf(".tmp should not exist after sync, stat err = %v", err)
	}
	// openclaw.json 应是合法 JSON
	cfg := readOpenclawJSON(t, baseDir)
	if cfg == nil {
		t.Fatal("openclaw.json empty or invalid after sync")
	}
}

// TestSyncOpenclawConfig_BindingFormat 新加的 binding 字段格式必须匹配用户指定:
//   {"agentId": "wechat", "match": {"channel": "openclaw-weixin", "accountId": "<id>"}}
func TestSyncOpenclawConfig_BindingFormat(t *testing.T) {
	baseDir := mockOpenclawConfig(t, `["new-bot-im-bot"]`, sampleOpenclawConfig)

	_, err := syncOpenclawConfig(baseDir)
	if err != nil {
		t.Fatalf("syncOpenclawConfig: %v", err)
	}

	cfg := readOpenclawJSON(t, baseDir)
	bindings, _ := cfg["bindings"].([]any)
	// 找到新加的那条
	var found map[string]any
	for _, b := range bindings {
		bMap, _ := b.(map[string]any)
		match, _ := bMap["match"].(map[string]any)
		if match["accountId"] == "new-bot-im-bot" {
			found = bMap
			break
		}
	}
	if found == nil {
		t.Fatal("new-bot-im-bot binding not found")
	}
	if found["agentId"] != "wechat" {
		t.Errorf("agentId = %v, want 'wechat'", found["agentId"])
	}
	match, _ := found["match"].(map[string]any)
	if match["channel"] != "openclaw-weixin" {
		t.Errorf("match.channel = %v, want 'openclaw-weixin'", match["channel"])
	}
	if match["accountId"] != "new-bot-im-bot" {
		t.Errorf("match.accountId = %v, want 'new-bot-im-bot'", match["accountId"])
	}

	// account 格式应是 "<id>": {}(空对象,不是其他值)
	channels, _ := cfg["channels"].(map[string]any)
	ow, _ := channels["openclaw-weixin"].(map[string]any)
	accounts, _ := ow["accounts"].(map[string]any)
	entry, _ := accounts["new-bot-im-bot"].(map[string]any)
	if entry == nil {
		t.Errorf("new-bot-im-bot account not an object: %T", accounts["new-bot-im-bot"])
	}
	if len(entry) != 0 {
		t.Errorf("new account should be {}, got %v", entry)
	}
}