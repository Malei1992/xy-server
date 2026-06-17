package handlers

import (
	"encoding/json"
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

// TestParseWechatOutput_NotJustSingleLine 防止"单行也算 QR"的退化:
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

// ===== 端到端 HTTP 测试 =====
// 走 gin engine + 真实 handler,验证响应码、字段、超时。

const fakeLink = "https://liteapp.weixin.qq.com/q/abc123"

// happyCmd 模拟"成功 + 一次 stdout"
func happyCmd() [][]string {
	return [][]string{
		{"sh", "-c", "echo 'startup...'; echo '" + strings.ReplaceAll(fakeQRBlock(), "\n", "\\n") + "'; echo 'https://liteapp.weixin.qq.com/q/abc123'; echo 'waiting'"},
	}
}

// linkOnlyCmd 模拟"只有 link 没有 qr"
func linkOnlyCmd() [][]string {
	return [][]string{
		{"sh", "-c", "echo 'https://liteapp.weixin.qq.com/q/onlylink'"},
	}
}

// qrOnlyCmd 模拟"只有 qr 没有 link"
func qrOnlyCmd() [][]string {
	return [][]string{
		{"sh", "-c", "echo '" + strings.ReplaceAll(fakeQRBlock(), "\n", "\\n") + "'"},
	}
}

// failCmd 模拟"docker 启动失败(非超时)":echo 到 stderr 后 exit 1
func failCmd() [][]string {
	return [][]string{
		{"sh", "-c", "echo 'no such container' 1>&2; exit 1"},
	}
}

// slowCmd 模拟"30s 还没结束":sleep 60
func slowCmd() [][]string {
	return [][]string{
		{"sh", "-c", "echo 'starting...'; sleep 60"},
	}
}

// TestPostWechatBind_Happy 正常输出 → 200,字段全
func TestPostWechatBind_Happy(t *testing.T) {
	r := newWechatEngine(t, happyCmd(), 5*time.Second)
	w := doWechatPost(t, r)
	if w.Code != 200 {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := decodeWechatBody(t, w)
	if body["ok"] != true {
		t.Errorf("ok = %v, want true", body["ok"])
	}
	if body["link"] != fakeLink {
		t.Errorf("link = %v, want %s", body["link"], fakeLink)
	}
	if body["qr"] == "" {
		t.Errorf("qr 不应为空")
	}
	if body["expired"] != false {
		t.Errorf("expired = %v, want false", body["expired"])
	}
	raw, _ := body["raw"].(string)
	if !strings.Contains(raw, fakeLink) {
		t.Errorf("raw 应含 link,实际 %q", raw)
	}
}

// TestPostWechatBind_LinkOnly → 200, qr=""
func TestPostWechatBind_LinkOnly(t *testing.T) {
	r := newWechatEngine(t, linkOnlyCmd(), 5*time.Second)
	w := doWechatPost(t, r)
	if w.Code != 200 {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := decodeWechatBody(t, w)
	if body["link"] != "https://liteapp.weixin.qq.com/q/onlylink" {
		t.Errorf("link = %v", body["link"])
	}
	if body["qr"] != "" {
		t.Errorf("qr 应为空,实际 %v", body["qr"])
	}
}

// TestPostWechatBind_QROnly → 200, link=""
func TestPostWechatBind_QROnly(t *testing.T) {
	r := newWechatEngine(t, qrOnlyCmd(), 5*time.Second)
	w := doWechatPost(t, r)
	if w.Code != 200 {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := decodeWechatBody(t, w)
	if body["link"] != "" {
		t.Errorf("link 应为空,实际 %v", body["link"])
	}
	if body["qr"] == "" {
		t.Errorf("qr 不应为空")
	}
}

// TestPostWechatBind_DockerFail → 500, expired=false
func TestPostWechatBind_DockerFail(t *testing.T) {
	r := newWechatEngine(t, failCmd(), 5*time.Second)
	w := doWechatPost(t, r)
	if w.Code != 500 {
		t.Fatalf("want 500, got %d body=%s", w.Code, w.Body.String())
	}
	body := decodeWechatBody(t, w)
	if _, ok := body["error"]; !ok {
		t.Errorf("应含 error 字段,实际 %v", body)
	}
	if body["expired"] != false {
		t.Errorf("expired = %v, want false (启动失败,非超时)", body["expired"])
	}
}

// TestPostWechatBind_Timeout → 500, expired=true
// 用 1s 超时打 sleep 60,验证超时分支
func TestPostWechatBind_Timeout(t *testing.T) {
	r := newWechatEngine(t, slowCmd(), 1*time.Second)
	w := doWechatPost(t, r)
	if w.Code != 500 {
		t.Fatalf("want 500, got %d body=%s", w.Code, w.Body.String())
	}
	body := decodeWechatBody(t, w)
	if _, ok := body["error"]; !ok {
		t.Errorf("应含 error 字段,实际 %v", body)
	}
	if body["expired"] != true {
		t.Errorf("expired = %v, want true (超时)", body["expired"])
	}
}

// ===== 端到端测试辅助函数 =====

func newWechatEngine(t *testing.T, commands [][]string, timeout time.Duration) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.POST("/wechat/bind", PostWechatBind(commands, timeout))
	return r
}

func doWechatPost(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, "/api/wechat/bind", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeWechatBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	return body
}
