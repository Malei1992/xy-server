package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// buildMultipart 把 fieldName=file 的 multipart body 装进 bytes.Buffer。
// 调用方负责在请求上设 Content-Type 为 w.FormDataContentType()。
func buildMultipart(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return body, w.FormDataContentType()
}

// emptyMultipart 不带 file 字段，仅闭合 multipart 边界
func emptyMultipart(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return body, w.FormDataContentType()
}

// minimalDocx 构造一个只满足"前 4 字节是 PK\x03\x04"的最小 docx。
// 真实 .docx 是完整 ZIP 结构；本服务只校验 magic，不解 zip。
func minimalDocx() []byte {
	magic := []byte{0x50, 0x4B, 0x03, 0x04} // PK\x03\x04
	return append(magic, []byte("rest of fake docx body, not a real zip")...)
}

// minimalDoc 构造一个只满足"前 8 字节是 OLE2 magic"的最小 doc。
// 真实 .doc 是完整 OLE2 复合文档；本服务只校验 magic，不解 OLE2 目录。
func minimalDoc() []byte {
	magic := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1} // OLE2/CFB
	return append(magic, []byte("rest of fake doc body, not a real ole2 doc")...)
}

// newUploadEngine 搭一个最小 router：仅含两个 upload 端点 + healthz。
// crmDir 用 t.TempDir()，绝不污染真实 data/crm/。
func newUploadEngine(t *testing.T, crmDir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.POST("/uploads/faq", uploadFaqHandlerForTest(crmDir))
	api.POST("/uploads/attachment-moonstar", uploadAttachmentHandlerForTest(crmDir))
	return r
}

// ----- /api/uploads/faq -----

// 1. Happy path：合法 magic + .docx 后缀 → 200 + 落盘字节一致
func TestUploadFaqHappy(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	content := minimalDocx()
	body, ct := buildMultipart(t, "faq.docx", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		OK   bool   `json:"ok"`
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if !resp.OK {
		t.Fatalf("want ok=true, got %+v", resp)
	}
	wantPath := filepath.Join(crmDir, "knowledge/FAQ.doc")
	if resp.Path != wantPath {
		t.Fatalf("want path=%q, got %q", wantPath, resp.Path)
	}
	if resp.Size != int64(len(content)) {
		t.Fatalf("want size=%d, got %d", len(content), resp.Size)
	}
	// 落盘验证
	onDisk, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(onDisk, content) {
		t.Fatalf("file content mismatch:\nwant=%q\ngot =%q", string(content), string(onDisk))
	}
}

// 2. 缺字段：空 multipart → 400
func TestUploadFaqMissingField(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	body, ct := emptyMultipart(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["error"] == "" {
		t.Fatalf("want error field, got %v", resp)
	}
}

// 3. 后缀错（.md）→ 400 "only .docx files are accepted"
func TestUploadFaqWrongExtMd(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	content := []byte("# not really markdown but check ext")
	body, ct := buildMultipart(t, "faq.md", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], ".docx") {
		t.Fatalf("want error to mention .docx, got %q", resp["error"])
	}
	if !strings.Contains(resp["error"], "only") {
		t.Fatalf("want error to match 'only .docx files are accepted', got %q", resp["error"])
	}
}

// 4. 后缀错（.txt）→ 400
func TestUploadFaqWrongExtTxt(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	content := []byte("plain text, not docx")
	body, ct := buildMultipart(t, "faq.txt", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	// 不应落盘
	wantPath := filepath.Join(crmDir, "knowledge/FAQ.doc")
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf(".txt upload must not be written, stat err=%v", err)
	}
}

// 5. 后缀错（.pdf）→ 400（即使内容是 PDF magic）
func TestUploadFaqWrongExtPdf(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	// 内容是合法 PDF，但后缀是 .pdf 而不是 .docx
	content := []byte("%PDF-1.4\nfake pdf body\n%%EOF\n")
	body, ct := buildMultipart(t, "faq.pdf", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], ".docx") {
		t.Fatalf("want error to mention .docx, got %q", resp["error"])
	}
}

// 6. ZIP magic 失败：.docx 后缀 + 内容是 "hello world" → 400 "not a valid Word document (ZIP magic missing)"
func TestUploadFaqBadMagic(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	content := []byte("hello world")
	body, ct := buildMultipart(t, "faq.docx", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "ZIP magic missing") {
		t.Fatalf("want error to mention 'ZIP magic missing', got %q", resp["error"])
	}
	// 不应落盘
	wantPath := filepath.Join(crmDir, "knowledge/FAQ.doc")
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf("invalid docx must not be written, stat err=%v", err)
	}
}

// 6b. ZIP magic 失败：内容太短（<4 字节）→ 400
func TestUploadFaqTooShortForMagic(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	content := []byte("PK") // 只有 2 字节
	body, ct := buildMultipart(t, "faq.docx", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "ZIP magic missing") {
		t.Fatalf("want error to mention 'ZIP magic missing', got %q", resp["error"])
	}
}

// 7. 超大：>100 MiB → 413，且前缀是 ZIP magic 也照样拒
func TestUploadFaqTooLarge(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	// 101 MiB：前 4 字节是合法 PK\x03\x04，但超过 100 MiB 上限
	content := make([]byte, 101*1024*1024)
	copy(content, []byte{0x50, 0x4B, 0x03, 0x04})
	for i := 4; i < len(content); i++ {
		content[i] = 'a'
	}
	body, ct := buildMultipart(t, "faq.docx", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d body=%s", w.Code, w.Body.String())
	}
	// 不应落盘
	wantPath := filepath.Join(crmDir, "knowledge/FAQ.doc")
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf("oversized file must not be written, stat err=%v", err)
	}
}

// 8. 大小写不敏感：.DOCX 后缀 + 合法 magic → 200
func TestUploadFaqCaseInsensitiveExt(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	content := minimalDocx()
	body, ct := buildMultipart(t, "FAQ.DOCX", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 (EqualFold on ext), got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		OK   bool   `json:"ok"`
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if !resp.OK {
		t.Fatalf("want ok=true, got %+v", resp)
	}
	wantPath := filepath.Join(crmDir, "knowledge/FAQ.doc")
	if resp.Path != wantPath {
		t.Fatalf("want path=%q, got %q", wantPath, resp.Path)
	}
}

// 9. Happy path：.doc 后缀 + OLE2 magic → 200，文件落到 knowledge/FAQ.doc
func TestUploadFaqDocHappy(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	content := minimalDoc()
	body, ct := buildMultipart(t, "faq.doc", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		OK   bool   `json:"ok"`
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if !resp.OK {
		t.Fatalf("want ok=true, got %+v", resp)
	}
	wantPath := filepath.Join(crmDir, "knowledge/FAQ.doc")
	if resp.Path != wantPath {
		t.Fatalf("want path=%q, got %q", wantPath, resp.Path)
	}
	if resp.Size != int64(len(content)) {
		t.Fatalf("want size=%d, got %d", len(content), resp.Size)
	}
	onDisk, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(onDisk, content) {
		t.Fatalf("file content mismatch:\nwant=%q\ngot =%q", string(content), string(onDisk))
	}
}

// 10. .doc + 非 OLE2 magic → 400 "OLE2 magic missing"
func TestUploadFaqDocBadMagic(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	content := []byte("hello world, not an OLE2 file")
	body, ct := buildMultipart(t, "faq.doc", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "OLE2 magic missing") {
		t.Fatalf("want error to mention 'OLE2 magic missing', got %q", resp["error"])
	}
	wantPath := filepath.Join(crmDir, "knowledge/FAQ.doc")
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf("invalid .doc must not be written, stat err=%v", err)
	}
}

// 11. .doc + 内容 < 8 字节 → 400 "OLE2 magic missing"
func TestUploadFaqDocTooShortForMagic(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	content := []byte("\xD0\xCF") // 只有 2 字节，magic 不完整
	body, ct := buildMultipart(t, "faq.doc", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "OLE2 magic missing") {
		t.Fatalf("want error to mention 'OLE2 magic missing', got %q", resp["error"])
	}
}

// 12. .doc + 上传的是 ZIP magic（错配）→ 400（.doc 必须 OLE2 magic，不能用 ZIP）
func TestUploadFaqDocWrongMagicZip(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	content := minimalDocx() // ZIP magic，不符合 .doc
	body, ct := buildMultipart(t, "faq.doc", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "OLE2 magic missing") {
		t.Fatalf("want error to mention 'OLE2 magic missing', got %q", resp["error"])
	}
}

// 13. 大小写不敏感：.DOC 后缀 + 合法 OLE2 magic → 200
func TestUploadFaqDocCaseInsensitiveExt(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	content := minimalDoc()
	body, ct := buildMultipart(t, "FAQ.DOC", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/faq", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 (EqualFold on ext), got %d body=%s", w.Code, w.Body.String())
	}
}

// ----- /api/uploads/attachment-moonstar -----

func TestUploadAttachmentHappy(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	// 合法 PDF：必须以 %PDF- 开头
	content := []byte("%PDF-1.4\nfake pdf body for test\n%%EOF\n")
	body, ct := buildMultipart(t, "moonstar.pdf", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/attachment-moonstar", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		OK   bool   `json:"ok"`
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	wantPath := filepath.Join(crmDir, "attachments/MOONSTAR_Investment.pdf")
	if resp.Path != wantPath {
		t.Fatalf("want path=%q, got %q", wantPath, resp.Path)
	}
	if resp.Size != int64(len(content)) {
		t.Fatalf("want size=%d, got %d", len(content), resp.Size)
	}
	onDisk, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(onDisk, content) {
		t.Fatalf("file content mismatch")
	}
}

func TestUploadAttachmentMissingField(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	body, ct := emptyMultipart(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/attachment-moonstar", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUploadAttachmentWrongExt(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	content := []byte("%PDF-1.4\nbody\n")
	body, ct := buildMultipart(t, "moonstar.doc", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/attachment-moonstar", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUploadAttachmentTooLarge(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	// 101 MiB，PDF magic 开头 + 后缀正确，但超过 100 MiB 上限
	content := make([]byte, 101*1024*1024)
	copy(content, []byte("%PDF-"))
	for i := 5; i < len(content); i++ {
		content[i] = 'a'
	}
	body, ct := buildMultipart(t, "moonstar.pdf", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/attachment-moonstar", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUploadAttachmentBadMagic(t *testing.T) {
	crmDir := t.TempDir()
	r := newUploadEngine(t, crmDir)

	// .pdf 后缀 + 非 PDF magic
	content := []byte("hello world, definitely not a pdf")
	body, ct := buildMultipart(t, "moonstar.pdf", content)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/uploads/attachment-moonstar", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "PDF") {
		t.Fatalf("want error to mention PDF, got %q", resp["error"])
	}
	// 不应落盘
	wantPath := filepath.Join(crmDir, "attachments/MOONSTAR_Investment.pdf")
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf("invalid PDF must not be written, stat err=%v", err)
	}
}

// 额外保险：确保两个 happy-path 落盘路径一致且不重叠
func TestUploadPathsAreDistinct(t *testing.T) {
	crmDir := t.TempDir()
	faqPath := filepath.Join(crmDir, "knowledge/FAQ.doc")
	attPath := filepath.Join(crmDir, "attachments/MOONSTAR_Investment.pdf")
	if faqPath == attPath {
		t.Fatalf("upload paths must differ: %q", faqPath)
	}
	if !strings.HasPrefix(faqPath, crmDir+"/knowledge/") {
		t.Fatalf("faqPath not under knowledge/: %q", faqPath)
	}
	if !strings.HasPrefix(attPath, crmDir+"/attachments/") {
		t.Fatalf("attPath not under attachments/: %q", attPath)
	}
}
