package handlers

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 显式错误，便于上层映射状态码。
var (
	// ErrTooLarge 上传内容超过允许字节上限
	ErrTooLarge = errors.New("file too large")

	// ErrInvalidContent 内容校验失败（后缀或 magic bytes / 文本合法性）
	ErrInvalidContent = errors.New("invalid file content")

	// ErrMissingField multipart 缺 file 字段
	ErrMissingField = errors.New("missing file field")
)

// maxBytes 内的"探针"读取窗口；超过窗口但总长也超 maxBytes 仍然判定为 413。
// 实际最大限制由调用方在 saveUpload 传入。
const (
	// faqMaxBytes FAQ 最大 10 MiB
	faqMaxBytes int64 = 10 * 1024 * 1024

	// pdfMaxBytes 外宣材料最大 10 MiB
	pdfMaxBytes int64 = 10 * 1024 * 1024

	// pdfMagic PDF 文件头魔数
	pdfMagic = "%PDF-"

	// zipMagic ZIP local file header 魔数（.docx 本质是 ZIP 包）
	zipMagic = "PK\x03\x04"

	// ole2Magic OLE2/CFB 复合文档魔数（.doc 旧 Word 格式是 OLE2 容器）
	ole2Magic = "\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1"
)

// saveUpload 通用上传处理：取 file 字段 → 读全内容 → 调 validate(ext, data)
// → 写 fullPath 覆盖保存 → 返回 nil。
// 任何错误由调用方映射为 4xx/5xx 响应。
//
// 注意：fullPath 必须是绝对路径且来自常量配置，不接受用户输入。
// 之所以不调 safeJoin，是因为：这些路径硬编码在 paths.go 里，
// 由部署者修改常量保证安全。
func saveUpload(c *gin.Context, fullPath string, validate func(data []byte, ext string) error, maxBytes int64) error {
	// 1) 缺字段
	fh, err := c.FormFile("file")
	if err != nil {
		return ErrMissingField
	}

	// 2) 提前看 ContentLength：比 maxBytes 大直接拒，省得把全量读进内存
	if c.Request.ContentLength > 0 && c.Request.ContentLength > maxBytes {
		return ErrTooLarge
	}
	if fh.Size > maxBytes {
		return ErrTooLarge
	}

	// 3) 读全部内容（小文件，10MiB 上限内 OK）
	f, err := fh.Open()
	if err != nil {
		return err
	}
	defer f.Close()

	// 用 bytes.Buffer 一把读完，避免大块分配切片
	buf := bytes.Buffer{}
	// 64 KiB chunk
	chunk := make([]byte, 64*1024)
	read := int64(0)
	for {
		n, rerr := f.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
			read += int64(n)
			if read > maxBytes {
				// 边读边拦截，避免一次性大文件打爆内存
				return ErrTooLarge
			}
		}
		if rerr != nil {
			break
		}
	}
	data := buf.Bytes()

	// 4) 校验（后缀 + 内容）
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if err := validate(data, ext); err != nil {
		return err
	}

	// 5) 落盘：父目录自动建，覆盖写
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return err
	}
	return nil
}

// validateFaq FAQ 文件校验：后缀 .doc 或 .docx + 对应 magic 字节。
// - .docx：前 4 字节是 ZIP magic（PK\x03\x04），.docx 实质是 ZIP 包
// - .doc：前 8 字节是 OLE2/CFB magic（D0 CF 11 E0 A1 B1 1A E1），旧 Word 格式是 OLE2 容器
// magic 校验不做深度解析（不解 zip、不解 OLE2 目录），只验文件头 magic。
// 落盘时统一存为 .doc（见 FaqRelPath 常量），不保留用户上传的扩展名。
func validateFaq(data []byte, ext string) error {
	switch {
	case strings.EqualFold(ext, ".docx"):
		if len(data) < len(zipMagic) {
			return errors.New("not a valid Word document (ZIP magic missing)")
		}
		if !bytes.Equal(data[:len(zipMagic)], []byte(zipMagic)) {
			return errors.New("not a valid Word document (ZIP magic missing)")
		}
		return nil
	case strings.EqualFold(ext, ".doc"):
		if len(data) < len(ole2Magic) {
			return errors.New("not a valid Word document (OLE2 magic missing)")
		}
		if !bytes.Equal(data[:len(ole2Magic)], []byte(ole2Magic)) {
			return errors.New("not a valid Word document (OLE2 magic missing)")
		}
		return nil
	default:
		return errors.New("only .doc or .docx files are accepted")
	}
}

// validateAttachmentMoonstar 外宣 PDF 校验：后缀 .pdf + magic bytes "%PDF-"。
func validateAttachmentMoonstar(data []byte, ext string) error {
	if !strings.EqualFold(ext, ".pdf") {
		return errors.New("only .pdf files are accepted")
	}
	if len(data) < len(pdfMagic) {
		return errors.New("not a valid PDF file")
	}
	if !bytes.Equal(data[:len(pdfMagic)], []byte(pdfMagic)) {
		return errors.New("not a valid PDF file")
	}
	return nil
}

// PostFaq FAQ 上传端点
// 入参：multipart/form-data，字段名 file；必须是 ≤10MiB 的 .docx（ZIP magic 校验）
// 保存到 <crmDir>/<relPath>（relPath 来自 paths.go 常量，绝对路径前 crmDir 是根）。
func PostFaq(crmDir, relPath string) gin.HandlerFunc {
	fullPath := filepath.Join(crmDir, relPath)
	return func(c *gin.Context) {
		err := saveUpload(c, fullPath, validateFaq, faqMaxBytes)
		if err != nil {
			logUploadFailure("faq", err)
			writeUploadError(c, err)
			return
		}
		info, statErr := os.Stat(fullPath)
		size := int64(0)
		if statErr == nil {
			size = info.Size()
		}
		L.Info("upload faq", zap.String("path", fullPath), zap.Int64("size", size))
		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"path": fullPath,
			"size": size,
		})
	}
}

// PostAttachmentMoonstar 外宣 PDF 上传端点
// 入参：multipart/form-data，字段名 file；必须是 ≤10MiB 的 .pdf
// 保存到 <crmDir>/<relPath>
func PostAttachmentMoonstar(crmDir, relPath string) gin.HandlerFunc {
	fullPath := filepath.Join(crmDir, relPath)
	return func(c *gin.Context) {
		err := saveUpload(c, fullPath, validateAttachmentMoonstar, pdfMaxBytes)
		if err != nil {
			logUploadFailure("attachment-moonstar", err)
			writeUploadError(c, err)
			return
		}
		info, statErr := os.Stat(fullPath)
		size := int64(0)
		if statErr == nil {
			size = info.Size()
		}
		L.Info("upload attachment-moonstar", zap.String("path", fullPath), zap.Int64("size", size))
		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"path": fullPath,
			"size": size,
		})
	}
}

// logUploadFailure 分类记上传失败原因。
func logUploadFailure(kind string, err error) {
	switch {
	case errors.Is(err, ErrTooLarge):
		L.Warn("upload rejected: too large", zap.String("kind", kind), zap.Error(err))
	case errors.Is(err, ErrMissingField):
		L.Warn("upload rejected: missing file field", zap.String("kind", kind))
	case errors.Is(err, ErrInvalidContent):
		L.Warn("upload rejected: invalid content", zap.String("kind", kind), zap.Error(err))
	default:
		// 来自 validate 的具体消息或底层 IO 错误
		msg := err.Error()
		if strings.HasPrefix(msg, "only .") || strings.HasPrefix(msg, "not a valid") {
			L.Warn("upload rejected: content validate", zap.String("kind", kind), zap.String("msg", msg))
		} else {
			L.Error("upload failed: io", zap.String("kind", kind), zap.Error(err))
		}
	}
}

// writeUploadError 把 saveUpload 的错误映射成 4xx 响应。
func writeUploadError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrMissingField):
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing 'file' field"})
	case errors.Is(err, ErrTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
	case errors.Is(err, ErrInvalidContent):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		// 来自 validate 的具体消息（"only .doc or .docx ..."/"not a valid ..."）也是 400
		msg := err.Error()
		// 区分"明显是用户内容问题" vs "服务器读盘失败"
		if strings.HasPrefix(msg, "only .") || strings.HasPrefix(msg, "not a valid") {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
	}
}
