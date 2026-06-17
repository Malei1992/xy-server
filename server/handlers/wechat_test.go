package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// fakeQRBlock 构造一个标准 20 行 QR 块,仅含 ▄/█/空白,便于跨测试复用。
// 内容是"任意合法字符",只关心块结构是否被识别。
func fakeQRBlock() string {
	// 20 行,每行 32 个块字符
	lines := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		row := ""
		for j := 0; j < 32; j++ {
			if (i+j)%2 == 0 {
				row += "█"
			} else {
				row += "▄"
			}
		}
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

// ===== parseWechatOutput 单元测试(纯函数,跟异步无关,保留) =====

// TestParseWechatOutput_Normal 单个 QR + 单个 link → 都能解析到
func TestParseWechatOutput_Normal(t *testing.T) {
	out := "starting openclaw login\n" +
		"Please scan the QR code below:\n" +
		fakeQRBlock() + "\n" +
		"https://liteapp.weixin.qq.com/q/abc123\n" +
		"Waiting for scan...\n"
	link, qr := parseWechatOutput(out)
	if link != "https://liteapp.weixin.qq.com/q/abc123" {
		t.Errorf("link = %q, want abc123 link", link)
	}
	if !strings.HasPrefix(qr, "█") || !strings.HasSuffix(qr, "█") {
		t.Errorf("qr 应以 █ 开头和结尾,实际 %q", qr)
	}
	if strings.Count(qr, "\n")+1 != 20 {
		t.Errorf("qr 应是 20 行,实际 %d 行", strings.Count(qr, "\n")+1)
	}
}

// TestParseWechatOutput_MultipleRefreshes 多轮刷新(3 套) → 取最后一对
func TestParseWechatOutput_MultipleRefreshes(t *testing.T) {
	out := "round 1\n" + fakeQRBlock() + "\n" +
		"https://liteapp.weixin.qq.com/q/round1\n" +
		"round 2\n" + fakeQRBlock() + "\n" +
		"https://liteapp.weixin.qq.com/q/round2\n" +
		"round 3\n" + fakeQRBlock() + "\n" +
		"https://liteapp.weixin.qq.com/q/round3\n" +
		"final\n"
	link, qr := parseWechatOutput(out)
	if link != "https://liteapp.weixin.qq.com/q/round3" {
		t.Errorf("link 应是 round3,实际 %q", link)
	}
	if strings.Count(qr, "\n")+1 != 20 {
		t.Errorf("qr 应是最后一轮 20 行,实际 %d 行", strings.Count(qr, "\n")+1)
	}
}

// TestParseWechatOutput_LinkOnly 只有 link 没有 qr → 200 语义,parseWechatOutput 应返 link, qr=""
func TestParseWechatOutput_LinkOnly(t *testing.T) {
	out := "openclaw login...\n" +
		"https://liteapp.weixin.qq.com/q/onlylink\n" +
		"waiting\n"
	link, qr := parseWechatOutput(out)
	if link != "https://liteapp.weixin.qq.com/q/onlylink" {
		t.Errorf("link = %q, want onlylink", link)
	}
	if qr != "" {
		t.Errorf("qr 应为空,实际 %q", qr)
	}
}

// TestParseWechatOutput_QROnly 只有 qr 没有 link → 仍能解析 qr, link=""
func TestParseWechatOutput_QROnly(t *testing.T) {
	out := "openclaw login...\n" + fakeQRBlock() + "\n" + "no link here\n"
	link, qr := parseWechatOutput(out)
	if link != "" {
		t.Errorf("link 应为空,实际 %q", link)
	}
	if strings.Count(qr, "\n")+1 != 20 {
		t.Errorf("qr 应是 20 行,实际 %d 行", strings.Count(qr, "\n")+1)
	}
}

// TestParseWechatOutput_Empty 空输出 → 都空
func TestParseWechatOutput_Empty(t *testing.T) {
	link, qr := parseWechatOutput("")
	if link != "" || qr != "" {
		t.Errorf("空输入应都空,实际 link=%q qr=%q", link, qr)
	}
}

// TestParseWechatOutput_QRWithBlankLines QR 块前后有空行也应识别
func TestParseWechatOutput_QRWithBlankLines(t *testing.T) {
	out := "header\n" + "\n" + fakeQRBlock() + "\n\n" + "footer\n"
	link, qr := parseWechatOutput(out)
	if qr == "" {
		t.Errorf("qr 不应为空")
	}
	if strings.Count(qr, "\n")+1 != 20 {
		t.Errorf("qr 应是 20 行,实际 %d 行", strings.Count(qr, "\n")+1)
	}
	_ = link
}

// TestParseWechatOutput_SingleLineNotQR 防止"单行也算 QR"的退化:
// 1 行包含 ▄/█ 不能算 QR 块(必须连续 >= 5 行,典型 20 行)
func TestParseWechatOutput_SingleLineNotQR(t *testing.T) {
	out := "random ▄▄▄█ text █▄▄▄ on one line\n" +
		"https://liteapp.weixin.qq.com/q/single\n"
	link, qr := parseWechatOutput(out)
	if link != "https://liteapp.weixin.qq.com/q/single" {
		t.Errorf("link = %q", link)
	}
	if qr != "" {
		t.Errorf("单行 ▄/█ 不应被识别为 qr,实际 %q", qr)
	}
}

// fakeQRBlockFullRange 构造一个标准 20 行 QR 块,使用完整的 Unicode 块字符
// 集(▀▁▂▃▄▅▆▇█▉▊▋▌▍▎▏ + ░▒▓),而不是只 ▄/█。
// 验证 qrLineRegex 覆盖完整块元素范围 —— 之前只放 ▄/█ 时,这种块会被切成
// 1 行小段然后 < minLines,导致解析空。
func fakeQRBlockFullRange() string {
	chars := []rune{'▀', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█', '▉', '▊', '▋', '▌', '▍', '▎', '▏', '░', '▒', '▓'}
	lines := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		row := make([]rune, 32)
		for j := 0; j < 32; j++ {
			row[j] = chars[(i+j)%len(chars)]
		}
		lines = append(lines, string(row))
	}
	return strings.Join(lines, "\n")
}

// TestParseWechatOutput_FullBlockCharRange QR 块用完整块字符(▀▁▂▃...▓)也应被识别。
// 之前的实现 qrLineRegex 只放 ▄/█,openclaw 实际会混合使用整组,导致块被切碎。
func TestParseWechatOutput_FullBlockCharRange(t *testing.T) {
	out := "starting openclaw login\n" +
		"Please scan the QR code below:\n" +
		fakeQRBlockFullRange() + "\n" +
		"https://liteapp.weixin.qq.com/q/fullrange\n"
	link, qr := parseWechatOutput(out)
	if link != "https://liteapp.weixin.qq.com/q/fullrange" {
		t.Errorf("link = %q", link)
	}
	if strings.Count(qr, "\n")+1 != 20 {
		t.Errorf("qr 应是 20 行,实际 %d 行 (%q)", strings.Count(qr, "\n")+1, qr)
	}
	// qr 必须包含 ▀(上块)和 ▓(阴影块)——这些是 ▄/█ 旧正则匹配不到的字符
	if !strings.Contains(qr, "▀") {
		t.Errorf("qr 应包含 ▀(上块),实际未含")
	}
	if !strings.Contains(qr, "▓") {
		t.Errorf("qr 应包含 ▓(阴影块),实际未含")
	}
}

// TestParseWechatOutput_RealisticOpenclaw 模拟 openclaw 真实输出:
// QR 块之前有几行中文(被 bufio.Scanner 正常 split),中间是 QR 块,后面有链接。
// 验证 "最后一对"语义在多语言夹杂场景下仍工作。
func TestParseWechatOutput_RealisticOpenclaw(t *testing.T) {
	out := "正在启动...\n\n" +
		"用手机微信扫描以下二维码,以继续连接：\n\n" +
		fakeQRBlockFullRange() + "\n" +
		"https://liteapp.weixin.qq.com/q/7GiQu1?qrcode=xxx&bot_type=3\n" +
		"\n正在等待操作...\n\n"
	link, qr := parseWechatOutput(out)
	if link != "https://liteapp.weixin.qq.com/q/7GiQu1?qrcode=xxx&bot_type=3" {
		t.Errorf("link = %q", link)
	}
	if strings.Count(qr, "\n")+1 != 20 {
		t.Errorf("qr 应是 20 行,实际 %d 行", strings.Count(qr, "\n")+1)
	}
}

// ===== 异步端到端 HTTP 测试 =====
// 走 gin engine + 真实 handler,验证:POST 立即 202 + task_id;轮询 GET 拿到 done/failed/expired。
// 同步版本的 e2e 测试已删(契约变了:不再同步等 docker)。

const fakeLink = "https://liteapp.weixin.qq.com/q/abc123"

// happyCmd 模拟"成功 + 一次 stdout"
func happyCmd() [][]string {
	return [][]string{
		{"sh", "-c", "echo 'startup...'; echo '" + strings.ReplaceAll(fakeQRBlock(), "\n", "\\n") + "'; echo 'https://liteapp.weixin.qq.com/q/abc123'; echo 'waiting'"},
	}
}

// failCmd 模拟"docker 启动失败(非超时)":echo 到 stderr 后 exit 1
func failCmd() [][]string {
	return [][]string{
		{"sh", "-c", "echo 'no such container' 1>&2; exit 1"},
	}
}

// slowCmd 模拟"30s 还没结束":先打印完整 QR + link(模拟 openclaw 已生成),
// 然后 sleep 60(后端 1s 超时,所以会被 SIGKILL 砍掉)。
//
// 重点:超时前 stdout 已经有 QR + link —— 旧实现 bug 是只在 done 路径调
// parseWechatOutput,导致 expired 时 link/qr 都是空。修复后,expired 状态也
// 应该有 link/qr(用于前端 modal 让用户试扫"可能还有效"的二维码)。
func slowCmd() [][]string {
	return [][]string{
		{"sh", "-c", "echo 'starting...'; echo '" + strings.ReplaceAll(fakeQRBlock(), "\n", "\\n") + "'; echo 'https://liteapp.weixin.qq.com/q/timeout-link'; sleep 60"},
	}
}

// slowCmdNoOutput 模拟"30s 还没结束 且 stdout 没 QR":pure sleep。
// 用来验证 link/qr 都是空时,确实走 failed/error 路径,不会强行填。
func slowCmdNoOutput() [][]string {
	return [][]string{
		{"sh", "-c", "echo 'starting...'; sleep 60"},
	}
}

// boundCmd 模拟"openclaw 扫码成功后输出成功标记"：打印 QR + link + 标记 + exit 0。
// 用于测试 Bound=true 在 scanner 循环里被早期发布,以及 final state 里也保留。
func boundCmd() [][]string {
	return [][]string{
		{"sh", "-c", "echo 'starting...'; echo '" + strings.ReplaceAll(fakeQRBlock(), "\n", "\\n") + "'; echo 'https://liteapp.weixin.qq.com/q/bound-link'; echo '已将此 OpenClaw 连接到微信。'; exit 0"},
	}
}

// slowBoundCmd 模拟"openclaw 打印了 QR/link,等用户扫码期间":不打印成功标记,
// sleep 60(让 test 通过 timeout 强制 SIGKILL 走 expired 路径)。但保留 QR/link。
// 用来验证"early publish 数据但终态不带 Bound"的场景。
func slowBoundCmd() [][]string {
	return slowCmd()
}

// slowBoundWithMarkerCmd 模拟"openclaw 打印了 QR/link 后等用户扫码,然后输出
// 成功标记后自然 exit 0":用 sleep 0.2 模拟扫码间隔,让 test 能轮询到 running+bound
// 的中间状态(用户可以提前看到成功标记而不必等 exec 自然返回)。
func slowBoundWithMarkerCmd() [][]string {
	return [][]string{
		{"sh", "-c", "echo 'starting...'; echo '" + strings.ReplaceAll(fakeQRBlock(), "\n", "\\n") + "'; echo 'https://liteapp.weixin.qq.com/q/marker-link'; sleep 0.3; echo '已将此 OpenClaw 连接到微信。'; exit 0"},
	}
}

// TestPostWechatBind_Happy 正常输出 → POST 返 202 + task_id;轮询 GET 最终 status=done, link/qr 正确
func TestPostWechatBind_Happy(t *testing.T) {
	resetWechatStore(t)
	r := newWechatEngine(t, happyCmd(), 5*time.Second)

	postResp := postWechatBind(t, r)
	if postResp.Code != http.StatusAccepted {
		t.Fatalf("POST want 202, got %d body=%s", postResp.Code, postResp.Body.String())
	}
	postBody := decodeBody(t, postResp)
	taskID, _ := postBody["task_id"].(string)
	if !strings.HasPrefix(taskID, "wt-") {
		t.Fatalf("task_id 应以 wt- 开头,实际 %q", taskID)
	}
	if postBody["status"] != string(StatusPending) {
		t.Errorf("POST status 应是 pending,实际 %v", postBody["status"])
	}

	// 轮询 GET 拿结果
	final := pollWechatTask(t, r, taskID, 50, 50*time.Millisecond)
	if final["status"] != string(StatusDone) {
		t.Fatalf("最终 status 应是 done,实际 %v, body=%v", final["status"], final)
	}
	if final["link"] != fakeLink {
		t.Errorf("link = %v, want %s", final["link"], fakeLink)
	}
	if final["qr"] == "" {
		t.Errorf("qr 不应为空")
	}
	if final["expired"] != false {
		t.Errorf("expired = %v, want false", final["expired"])
	}
	raw, _ := final["raw"].(string)
	if !strings.Contains(raw, fakeLink) {
		t.Errorf("raw 应含 link,实际 %q", raw)
	}
	if _, ok := final["error"]; ok && final["error"] != "" {
		t.Errorf("done 状态 error 应为空,实际 %v", final["error"])
	}
}

// TestPostWechatBind_DockerFail docker 启动失败 → POST 返 202;轮询最终 status=failed, error 字段含 docker 错误
func TestPostWechatBind_DockerFail(t *testing.T) {
	resetWechatStore(t)
	r := newWechatEngine(t, failCmd(), 5*time.Second)

	postResp := postWechatBind(t, r)
	if postResp.Code != http.StatusAccepted {
		t.Fatalf("POST want 202, got %d", postResp.Code)
	}
	taskID, _ := decodeBody(t, postResp)["task_id"].(string)

	final := pollWechatTask(t, r, taskID, 50, 50*time.Millisecond)
	if final["status"] != string(StatusFailed) {
		t.Fatalf("最终 status 应是 failed,实际 %v, body=%v", final["status"], final)
	}
	if final["expired"] != false {
		t.Errorf("expired = %v, want false (启动失败,非超时)", final["expired"])
	}
	errStr, _ := final["error"].(string)
	if !strings.Contains(errStr, "exit status 1") {
		t.Errorf("error 应含 exit status 1,实际 %q", errStr)
	}
}

// TestPostWechatBind_Timeout sleep 60 + 1s 超时 → status=expired, expired=true。
// 同时验证 stdout 中的 QR + link 被保留(因为进程被中断但 QR 可能还有效,
// 前端要展示 modal 让用户试扫)。
func TestPostWechatBind_Timeout(t *testing.T) {
	resetWechatStore(t)
	r := newWechatEngine(t, slowCmd(), 1*time.Second)

	postResp := postWechatBind(t, r)
	if postResp.Code != http.StatusAccepted {
		t.Fatalf("POST want 202, got %d", postResp.Code)
	}
	taskID, _ := decodeBody(t, postResp)["task_id"].(string)

	final := pollWechatTask(t, r, taskID, 50, 50*time.Millisecond)
	if final["status"] != string(StatusExpired) {
		t.Fatalf("最终 status 应是 expired,实际 %v, body=%v", final["status"], final)
	}
	if final["expired"] != true {
		t.Errorf("expired = %v, want true (超时)", final["expired"])
	}
	errStr, _ := final["error"].(string)
	if errStr == "" {
		t.Errorf("expired 状态 error 不应为空")
	}
	// 关键:虽然 expired,但 link/qr 应该被填上(让前端 modal 让用户试扫)。
	// 旧实现这两字段都是空字符串。
	linkStr, _ := final["link"].(string)
	if linkStr != "https://liteapp.weixin.qq.com/q/timeout-link" {
		t.Errorf("expired 状态 link 应保留,实际 %q", linkStr)
	}
	qrStr, _ := final["qr"].(string)
	if qrStr == "" {
		t.Errorf("expired 状态 qr 不应为空(stdout 中有 QR 块)")
	}
	rawStr, _ := final["raw"].(string)
	if !strings.Contains(rawStr, "timeout-link") {
		t.Errorf("raw 应含原始 link,实际 %q", rawStr)
	}
}

// TestPostWechatBind_TimeoutNoOutput 超时且 stdout 无 QR/link → link/qr 都空,
// 不强行填。这是 expired 路径的"失败"分支,前端展示行内错误而不是 modal。
func TestPostWechatBind_TimeoutNoOutput(t *testing.T) {
	resetWechatStore(t)
	r := newWechatEngine(t, slowCmdNoOutput(), 1*time.Second)

	postResp := postWechatBind(t, r)
	if postResp.Code != http.StatusAccepted {
		t.Fatalf("POST want 202, got %d", postResp.Code)
	}
	taskID, _ := decodeBody(t, postResp)["task_id"].(string)

	final := pollWechatTask(t, r, taskID, 50, 50*time.Millisecond)
	if final["status"] != string(StatusExpired) {
		t.Fatalf("最终 status 应是 expired,实际 %v, body=%v", final["status"], final)
	}
	// stdout 没 QR/link → link/qr 都空,符合"真正失败"语义
	if final["link"] != "" {
		t.Errorf("stdout 无 link 时,expired 状态 link 应为空,实际 %v", final["link"])
	}
	if final["qr"] != "" {
		t.Errorf("stdout 无 qr 时,expired 状态 qr 应为空,实际 %v", final["qr"])
	}
}

// TestPostWechatBind_EarlyPublish 早期发布验证:openclaw 在 timeout 之前
// 就把 link+qr 写进 stdout 了(docker exec 不会自然返回,等用户扫码),
// 旧实现必须等 backend 2 分钟 timeout SIGKILL 才会把 link/qr 写进 store,
// 这时 GET 才能拿到 —— 表现就是前端 polling 2 分钟才出 modal,体感坏。
//
// 修复后:scanner 循环里看到 link 行就立刻把 link/qr 写进 store。
// 本测试用 slowCmd(打印 QR + link 后 sleep) + 3s timeout,验证:
//   - 早期发布:timeout 之前 GET 已经能看到 link/qr(status 仍 running)
//   - 终态保留:timeout 之后 status=expired,link/qr 仍在(final state 覆盖不丢失)
func TestPostWechatBind_EarlyPublish(t *testing.T) {
	resetWechatStore(t)
	r := newWechatEngine(t, slowCmd(), 3*time.Second)

	postResp := postWechatBind(t, r)
	if postResp.Code != http.StatusAccepted {
		t.Fatalf("POST want 202, got %d", postResp.Code)
	}
	taskID, _ := decodeBody(t, postResp)["task_id"].(string)

	// 早期发布:轮询 30 次(总 1.5s),期望 ~200ms 内就能看到 link/qr
	// 注意:break 时只记 earlyBody,不能"顺便"等到终态 —— 那会等到 timeout 后
	// status 变成 expired,失去 "early publish 时 status 仍 running" 的语义
	var earlyBody map[string]any
	var sawEarly bool
	for i := 0; i < 30; i++ {
		req, _ := http.NewRequest(http.MethodGet, "/api/wechat/bind/"+taskID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		body := decodeBody(t, w)
		link, _ := body["link"].(string)
		qr, _ := body["qr"].(string)
		if link != "" && qr != "" {
			earlyBody = body
			sawEarly = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !sawEarly {
		t.Fatalf("1.5s 内 GET 没看到 link/qr,early publish 没生效(应早于 3s timeout)")
	}
	// 此时 timeout(3s)还没到,status 应是 running,但 link/qr 已被早期发布
	if earlyBody["status"] != string(StatusRunning) {
		t.Errorf("early publish 时 status 应是 running,实际 %v", earlyBody["status"])
	}
	if earlyBody["link"] != "https://liteapp.weixin.qq.com/q/timeout-link" {
		t.Errorf("link = %v, want timeout-link", earlyBody["link"])
	}
	if earlyBody["qr"] == "" {
		t.Errorf("qr 不应为空")
	}

	// 终态:用 pollWechatTask 轮询到 done/failed/expired。
	// 这时 status 应该是 expired(3s timeout 触发),link/qr 仍在(early publish 已写,
	// final state 写时不清空 link/qr,只覆盖 status/error/expired)。
	final := pollWechatTask(t, r, taskID, 50, 100*time.Millisecond)
	if final["status"] != string(StatusExpired) {
		t.Errorf("终态 status 应是 expired,实际 %v", final["status"])
	}
	if final["link"] != "https://liteapp.weixin.qq.com/q/timeout-link" {
		t.Errorf("终态 link 应保留(early publish 已写,final state 不清空),实际 %v", final["link"])
	}
	if final["qr"] == "" {
		t.Errorf("终态 qr 不应为空")
	}
}

// TestGetWechatBindStatus_NotFound GET 不存在 id → 404
func TestGetWechatBindStatus_NotFound(t *testing.T) {
	resetWechatStore(t)
	r := newWechatEngine(t, happyCmd(), 5*time.Second)

	req, _ := http.NewRequest(http.MethodGet, "/api/wechat/bind/wt-doesnotexist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["error"] != "task not found" {
		t.Errorf("error = %v, want 'task not found'", body["error"])
	}
}

// TestPostWechatBind_DetectsBoundMarker stdout 含「已将此 OpenClaw 连接到微信。」
// → GET 响应里 bound=true。这是早期发布,即使 exec 还没自然退出,前端也能拿到。
func TestPostWechatBind_DetectsBoundMarker(t *testing.T) {
	resetWechatStore(t)
	// boundCmd 是 exec 0 的命令,但 scanner 循环里看到成功标记就置 Bound=true;
	// cmd.Wait() 返回 0 → final state 走 done,Bound 保留。
	r := newWechatEngine(t, boundCmd(), 5*time.Second)

	postResp := postWechatBind(t, r)
	if postResp.Code != http.StatusAccepted {
		t.Fatalf("POST want 202, got %d", postResp.Code)
	}
	taskID, _ := decodeBody(t, postResp)["task_id"].(string)

	final := pollWechatTask(t, r, taskID, 50, 50*time.Millisecond)
	if final["status"] != string(StatusDone) {
		t.Fatalf("最终 status 应是 done,实际 %v, body=%v", final["status"], final)
	}
	// 关键:bound=true 被记录在 GET 响应里
	if final["bound"] != true {
		t.Errorf("bound = %v, want true (stdout 含成功标记)", final["bound"])
	}
	if final["link"] != "https://liteapp.weixin.qq.com/q/bound-link" {
		t.Errorf("link = %v, want bound-link", final["link"])
	}
}

// TestPostWechatBind_BoundEarlyPublish bound 也能早期发布:exec 在 sleep 期间
// 输出成功标记,timeout 之前 GET 就能看到 bound=true 且 status 仍 running。
// 这模拟"用户扫码快,docker exec 还没自然退"场景。
func TestPostWechatBind_BoundEarlyPublish(t *testing.T) {
	resetWechatStore(t)
	r := newWechatEngine(t, slowBoundWithMarkerCmd(), 3*time.Second)

	postResp := postWechatBind(t, r)
	if postResp.Code != http.StatusAccepted {
		t.Fatalf("POST want 202, got %d", postResp.Code)
	}
	taskID, _ := decodeBody(t, postResp)["task_id"].(string)

	// 轮询 30 次(总 1.5s),期望在 0.5s 内能看到 bound=true
	var earlyBody map[string]any
	var sawBound bool
	for i := 0; i < 30; i++ {
		req, _ := http.NewRequest(http.MethodGet, "/api/wechat/bind/"+taskID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		body := decodeBody(t, w)
		if b, _ := body["bound"].(bool); b {
			earlyBody = body
			sawBound = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !sawBound {
		t.Fatalf("1.5s 内 GET 没看到 bound=true,early publish 没生效")
	}
	// 此时可能在 running,也可能已进入 done(exit 0 后),看时序
	// 都接受:关键是 bound=true 已写入
	if earlyBody["bound"] != true {
		t.Errorf("bound = %v, want true", earlyBody["bound"])
	}
}

// TestPostWechatBindCancel_BasicFlow running 期间 cancel → 进程被杀,status=expired,
// error 含 cancel 提示。这模拟"用户点关闭后等不到 2 分钟 timeout"的场景。
func TestPostWechatBindCancel_BasicFlow(t *testing.T) {
	resetWechatStore(t)
	// slowCmd:打印完 QR+link 后 sleep 60,1s timeout 会 SIGKILL。
	// cancel 在 timeout 之前调,能抢先结束 exec。
	r := newWechatEngine(t, slowCmd(), 5*time.Second)

	postResp := postWechatBind(t, r)
	if postResp.Code != http.StatusAccepted {
		t.Fatalf("POST want 202, got %d", postResp.Code)
	}
	taskID, _ := decodeBody(t, postResp)["task_id"].(string)

	// 等后台 goroutine 把 cancel 字段设上(从 store.Add 到第一个 store.Update(cancel) 之间
	// 有 200ms 左右的窗口,POST cancel 必须不早于这个时刻)
	time.Sleep(100 * time.Millisecond)

	// 调 cancel 端点
	cancelReq, _ := http.NewRequest(http.MethodPost, "/api/wechat/bind/"+taskID+"/cancel", nil)
	cancelW := httptest.NewRecorder()
	r.ServeHTTP(cancelW, cancelReq)
	if cancelW.Code != http.StatusOK {
		t.Fatalf("cancel want 200, got %d body=%s", cancelW.Code, cancelW.Body.String())
	}
	cancelBody := decodeBody(t, cancelW)
	if cancelBody["cancelled"] != true {
		t.Errorf("cancelled = %v, want true", cancelBody["cancelled"])
	}

	// 轮询到终态:status 应是 expired(被 cancel 触发,跟 timeout 等价走 expired 路径)
	final := pollWechatTask(t, r, taskID, 50, 50*time.Millisecond)
	if final["status"] != string(StatusExpired) {
		t.Errorf("cancel 后 status 应是 expired,实际 %v", final["status"])
	}
	if final["expired"] != true {
		t.Errorf("cancel 后 expired 应是 true,实际 %v", final["expired"])
	}
	// link/qr 仍保留(早期发布时已写入)
	if final["link"] != "https://liteapp.weixin.qq.com/q/timeout-link" {
		t.Errorf("cancel 后 link 应保留,实际 %v", final["link"])
	}
}

// TestPostWechatBindCancel_NotFound cancel 不存在 task → 404
func TestPostWechatBindCancel_NotFound(t *testing.T) {
	resetWechatStore(t)
	r := newWechatEngine(t, happyCmd(), 5*time.Second)

	req, _ := http.NewRequest(http.MethodPost, "/api/wechat/bind/wt-doesnotexist/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["error"] != "task not found" {
		t.Errorf("error = %v, want 'task not found'", body["error"])
	}
}

// TestPostWechatBindCancel_AfterDone 已 done 的 task 再 cancel → 200 + cancelled=false,
// 不重复 kill。这是幂等保护(避免前端 modal 关闭按钮被用户连点时出乱子)。
func TestPostWechatBindCancel_AfterDone(t *testing.T) {
	resetWechatStore(t)
	r := newWechatEngine(t, happyCmd(), 5*time.Second)

	postResp := postWechatBind(t, r)
	if postResp.Code != http.StatusAccepted {
		t.Fatalf("POST want 202, got %d", postResp.Code)
	}
	taskID, _ := decodeBody(t, postResp)["task_id"].(string)

	// 轮询到 done
	final := pollWechatTask(t, r, taskID, 50, 50*time.Millisecond)
	if final["status"] != string(StatusDone) {
		t.Fatalf("setup: 任务应先到 done,实际 %v", final["status"])
	}

	// cancel 已 done 的 task
	cancelReq, _ := http.NewRequest(http.MethodPost, "/api/wechat/bind/"+taskID+"/cancel", nil)
	cancelW := httptest.NewRecorder()
	r.ServeHTTP(cancelW, cancelReq)
	if cancelW.Code != http.StatusOK {
		t.Fatalf("cancel 已 done task want 200, got %d body=%s", cancelW.Code, cancelW.Body.String())
	}
	cancelBody := decodeBody(t, cancelW)
	if cancelBody["cancelled"] != false {
		t.Errorf("done task cancel 后 cancelled = %v, want false", cancelBody["cancelled"])
	}
	if cancelBody["reason"] != "task already finished" {
		t.Errorf("reason = %v, want 'task already finished'", cancelBody["reason"])
	}
}

// TestPostWechatBind_ConcurrentTasks 同时建 3 个 task,各自独立完成,互不干扰
func TestPostWechatBind_ConcurrentTasks(t *testing.T) {
	resetWechatStore(t)
	// 3 个不同的 happy 命令(每条 echo 不同的 link),验证不会被串台
	cmds := [][]string{
		{"sh", "-c", "echo '" + strings.ReplaceAll(fakeQRBlock(), "\n", "\\n") + "'; echo 'https://liteapp.weixin.qq.com/q/task1'"},
		{"sh", "-c", "echo '" + strings.ReplaceAll(fakeQRBlock(), "\n", "\\n") + "'; echo 'https://liteapp.weixin.qq.com/q/task2'"},
		{"sh", "-c", "echo '" + strings.ReplaceAll(fakeQRBlock(), "\n", "\\n") + "'; echo 'https://liteapp.weixin.qq.com/q/task3'"},
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	// 同一个 handler 工厂接受不同 commands 是不行的(闭包已固定),所以建 3 个 handler
	// 共享同一个 task store(由 handler 内部 GetWechatTaskStore 拿到),互不干扰。
	api.POST("/wechat/bind", PostWechatBind([][]string{cmds[0]}, 5*time.Second))
	api.GET("/wechat/bind/:task_id", GetWechatBindStatus())
	api.POST("/wechat/bind2", PostWechatBind([][]string{cmds[1]}, 5*time.Second))
	api.GET("/wechat/bind2/:task_id", GetWechatBindStatus())
	api.POST("/wechat/bind3", PostWechatBind([][]string{cmds[2]}, 5*time.Second))
	api.GET("/wechat/bind3/:task_id", GetWechatBindStatus())

	// 三个 POST 都立刻拿 task_id
	taskIDs := make([]string, 3)
	paths := []string{"/api/wechat/bind", "/api/wechat/bind2", "/api/wechat/bind3"}
	for i, p := range paths {
		req, _ := http.NewRequest(http.MethodPost, p, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("POST %s want 202, got %d", p, w.Code)
		}
		id, _ := decodeBody(t, w)["task_id"].(string)
		if id == "" {
			t.Fatalf("POST %s task_id 为空", p)
		}
		taskIDs[i] = id
	}
	// 3 个 id 必须唯一
	seen := map[string]bool{}
	for _, id := range taskIDs {
		if seen[id] {
			t.Errorf("task_id 重复: %s", id)
		}
		seen[id] = true
	}

	// 三个分别轮询,期待各自 link 不同
	expectedLinks := []string{
		"https://liteapp.weixin.qq.com/q/task1",
		"https://liteapp.weixin.qq.com/q/task2",
		"https://liteapp.weixin.qq.com/q/task3",
	}
	for i, id := range taskIDs {
		path := fmt.Sprintf("/api/wechat/bind%d/%s", i+1, id)
		if i == 0 {
			path = "/api/wechat/bind/" + id
		}
		final := pollWechatTaskAtPath(t, r, path, 50, 50*time.Millisecond)
		if final["status"] != string(StatusDone) {
			t.Fatalf("task %d status = %v, want done, body=%v", i, final["status"], final)
		}
		if final["link"] != expectedLinks[i] {
			t.Errorf("task %d link = %v, want %s", i, final["link"], expectedLinks[i])
		}
	}
}

// TestWechatTaskStore_AddGetUpdate 单测 store 基本行为,跟 docker 无关。
func TestWechatTaskStore_AddGetUpdate(t *testing.T) {
	resetWechatStore(t)
	store := GetWechatTaskStore()

	id, task := store.Add()
	if task.Status != StatusPending {
		t.Errorf("Add 后 status 应是 pending,实际 %q", task.Status)
	}
	if !strings.HasPrefix(id, "wt-") {
		t.Errorf("id 应以 wt- 开头,实际 %q", id)
	}
	if len(id) != len("wt-")+12 {
		t.Errorf("id 应是 wt- + 12 hex 字符,实际 %q (len=%d)", id, len(id))
	}

	got, ok := store.Get(id)
	if !ok || got != task {
		t.Fatalf("Get 拿不到刚 Add 的 task: ok=%v", ok)
	}

	store.Update(id, func(t *WechatTask) {
		t.Status = StatusRunning
		t.Link = fakeLink
	})
	got2, _ := store.Get(id)
	if got2.Status != StatusRunning || got2.Link != fakeLink {
		t.Errorf("Update 后状态不对: %+v", got2)
	}

	// 拿不存在的 id
	if _, ok := store.Get("wt-deadbeef"); ok {
		t.Errorf("不存在的 id 不应拿到")
	}
}

// TestWechatTaskStore_GetSnapshot 验证 snapshot 是脱钩的值拷贝,改原 task 不影响 snapshot。
func TestWechatTaskStore_GetSnapshot(t *testing.T) {
	resetWechatStore(t)
	store := GetWechatTaskStore()
	id, _ := store.Add()

	// 初始 snapshot:status 已是 pending
	snap, ok := store.GetSnapshot(id)
	if !ok {
		t.Fatalf("GetSnapshot 拿不到")
	}
	if snap.Status != StatusPending {
		t.Errorf("初始 status 应是 pending,实际 %q", snap.Status)
	}

	// 通过 Update 改原 task(走锁)
	store.Update(id, func(t *WechatTask) {
		t.Status = StatusDone
		t.Link = fakeLink
	})

	// 之前拿的 snapshot 不应被影响(已经是脱钩的值)
	if snap.Status != StatusPending {
		t.Errorf("snapshot 状态不应被原 task 影响,实际 %q", snap.Status)
	}
	if snap.Link != "" {
		t.Errorf("snapshot link 不应被原 task 影响,实际 %q", snap.Link)
	}

	// 重新拿 snapshot 应该是新值
	snap2, _ := store.GetSnapshot(id)
	if snap2.Status != StatusDone || snap2.Link != fakeLink {
		t.Errorf("新 snapshot 应反映最新状态,实际 %+v", snap2)
	}
}

// TestWechatTaskStore_TTLCleanup 验证 TTL 清理逻辑:完成的 task 超过 ttl 被删,未完成的保留。
func TestWechatTaskStore_TTLCleanup(t *testing.T) {
	resetWechatStore(t)
	// 用一个 50ms ttl 的 store 直接测清理逻辑(不走单例的 5min ttl)
	s := newWechatTaskStore(50 * time.Millisecond)

	idA, _ := s.Add()
	s.Update(idA, func(t *WechatTask) {
		t.Status = StatusDone
		t.CompletedAt = time.Now().Add(-100 * time.Millisecond)
	})

	idB, _ := s.Add()
	// B 默认 StatusPending,CompletedAt 零值,模拟"还在跑"
	s.Update(idB, func(t *WechatTask) {
		t.Status = StatusRunning
	})

	idC, _ := s.Add()
	s.Update(idC, func(t *WechatTask) {
		t.Status = StatusFailed
		t.CompletedAt = time.Now().Add(-200 * time.Millisecond)
	})

	s.cleanupOnce()

	if _, ok := s.Get(idA); ok {
		t.Errorf("A(100ms 前完成, ttl 50ms)应被清理")
	}
	if _, ok := s.Get(idB); !ok {
		t.Errorf("B(还在 running)不应被清理")
	}
	if _, ok := s.Get(idC); ok {
		t.Errorf("C(200ms 前完成, ttl 50ms)应被清理")
	}
}

// TestWechatTaskStore_IDsUnique 多次 Add 的 id 互不重复
func TestWechatTaskStore_IDsUnique(t *testing.T) {
	resetWechatStore(t)
	store := GetWechatTaskStore()
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id, _ := store.Add()
		if seen[id] {
			t.Fatalf("id 撞车: %s (第 %d 次)", id, i)
		}
		seen[id] = true
	}
}

// ===== 测试辅助函数 =====

// resetWechatStore 每个异步测试前重置 store 单例,避免污染。
// 注意:旧 store 的后台清理 goroutine 还在跑,但它引用的 sync.Map 已被 GC(因为没人引用了),
// ticker 会空转直到进程结束;测试用 t.Cleanup 不杀它(无影响,且新 store 会启动新的 ticker)。
func resetWechatStore(t *testing.T) {
	t.Helper()
	ResetWechatTaskStoreForTest()
}

func newWechatEngine(t *testing.T, commands [][]string, timeout time.Duration) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.POST("/wechat/bind", PostWechatBind(commands, timeout))
	api.GET("/wechat/bind/:task_id", GetWechatBindStatus())
	api.POST("/wechat/bind/:task_id/cancel", PostWechatBindCancel())
	return r
}

func postWechatBind(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, "/api/wechat/bind", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	return body
}

// pollWechatTask 轮询 GET 任务状态,直到终态或重试用完。
//   - maxAttempts: 重试次数
//   - interval: 每次间隔
// 返回最后一次 GET 的 body(可能是 pending/running,如果是 timeout 用完了)。
func pollWechatTask(t *testing.T, r *gin.Engine, taskID string, maxAttempts int, interval time.Duration) map[string]any {
	t.Helper()
	return pollWechatTaskAtPath(t, r, "/api/wechat/bind/"+taskID, maxAttempts, interval)
}

func pollWechatTaskAtPath(t *testing.T, r *gin.Engine, path string, maxAttempts int, interval time.Duration) map[string]any {
	t.Helper()
	var lastBody map[string]any
	for i := 0; i < maxAttempts; i++ {
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s 拿非 200: %d body=%s", path, w.Code, w.Body.String())
		}
		lastBody = decodeBody(t, w)
		if s, _ := lastBody["status"].(string); s == string(StatusDone) || s == string(StatusFailed) || s == string(StatusExpired) {
			return lastBody
		}
		time.Sleep(interval)
	}
	t.Fatalf("轮询 %d 次仍未到终态,最后 body=%v", maxAttempts, lastBody)
	return lastBody
}
