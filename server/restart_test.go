package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newRestartEngine(t *testing.T, commands [][]string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.POST("/restart", postRestartHandlerForTest(commands))
	return r
}

func doRestartPost(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, "/api/restart", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 两条命令都成功 → 200 + 合并输出含两条命令的标记
func TestRestartSuccess(t *testing.T) {
	cmds := [][]string{
		{"echo", "step1-ok"},
		{"echo", "step2-ok"},
	}
	r := newRestartEngine(t, cmds)
	w := doRestartPost(t, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("want ok=true, got %v", body["ok"])
	}
	out, _ := body["output"].(string)
	if !strings.Contains(out, "step1-ok") {
		t.Errorf("output 应含 step1-ok,实际 %q", out)
	}
	if !strings.Contains(out, "step2-ok") {
		t.Errorf("output 应含 step2-ok,实际 %q", out)
	}
	// 应含命令回显,便于运维审计
	if !strings.Contains(out, "echo step1-ok") {
		t.Errorf("output 应回显命令 echo step1-ok,实际 %q", out)
	}
	if !strings.Contains(out, "echo step2-ok") {
		t.Errorf("output 应回显命令 echo step2-ok,实际 %q", out)
	}
}

// 第一条命令失败 → 500,output 仍含已执行的输出
func TestRestartStep1Fails(t *testing.T) {
	cmds := [][]string{
		{"sh", "-c", "echo step1-broke; exit 1"},
		{"echo", "step2-not-run"},
	}
	r := newRestartEngine(t, cmds)
	w := doRestartPost(t, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Errorf("want error field, got %v", body)
	}
	out, _ := body["output"].(string)
	if !strings.Contains(out, "step1-broke") {
		t.Errorf("output 应含 step1 输出,实际 %q", out)
	}
}

// 第一条成功、第二条失败 → 500,且不丢失第一条输出
func TestRestartStep2Fails(t *testing.T) {
	cmds := [][]string{
		{"echo", "step1-ok"},
		{"sh", "-c", "echo step2-broke; exit 2"},
	}
	r := newRestartEngine(t, cmds)
	w := doRestartPost(t, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, _ := body["output"].(string)
	if !strings.Contains(out, "step1-ok") {
		t.Errorf("step1 输出不应丢失,实际 %q", out)
	}
	if !strings.Contains(out, "step2-broke") {
		t.Errorf("应含 step2 失败输出,实际 %q", out)
	}
}

// 空命令列表 → 200(无操作,等价于"没有要重启的容器")
func TestRestartEmptyCommands(t *testing.T) {
	r := newRestartEngine(t, nil)
	w := doRestartPost(t, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("want ok=true, got %v", body["ok"])
	}
}
