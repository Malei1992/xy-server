package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 客户档案文件名 / URL id 的约定前缀（与 schema.md 中 customer id 格式一致）。
// 客户目录里非 CUST 开头的 .json 不视为客户档案：list 跳过，single GET 返 404。
const customerIDPrefix = "CUST"

// GetCustomer 返回 <crmDir>/crm/customers/{id}.json。
// 路径非法（".."、绝对路径）→ 400；id 不以 CUST 开头 → 404；文件不存在 → 404；其他 IO 错 → 500。
func GetCustomer(crmDir string, safeJoin func(root, rel string) (string, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if !strings.HasPrefix(id, customerIDPrefix) {
			L.Warn("get customer: non-CUST id rejected", zap.String("id", id))
			c.JSON(http.StatusNotFound, gin.H{"error": "customer " + id + " not found"})
			return
		}
		full, err := safeJoin(crmDir+"/crm/customers", id+".json")
		if err != nil {
			L.Warn("get customer: safeJoin failed", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		data, err := readJSONFile(full)
		if err != nil {
			status := http.StatusInternalServerError
			if err.Error() == "file not found" {
				status = http.StatusNotFound
				L.Warn("get customer: not found", zap.String("id", id))
			} else {
				L.Error("get customer: read failed", zap.String("id", id), zap.String("path", full), zap.Error(err))
			}
			c.JSON(status, gin.H{"error": "customer " + id + " not found"})
			return
		}
		L.Info("get customer", zap.String("id", id))
		c.Data(http.StatusOK, "application/json; charset=utf-8", data)
	}
}

// ListCustomers 返回 <crmDir>/crm/customers/*.json 的全量列表，元素是 Customer 原文。
// 行为约定：
//   - 目录不存在 / 空 → 200 + []
//   - 单个文件损坏 / 空文件 / 非 .json / 子目录 / 不以 CUST 开头 → 跳过，不影响其他文件
//   - 按文件名（id）升序排序，输出稳定
//   - 始终返非 nil 数组（避免 JSON 序列化为 null）
//
// 用 []json.RawMessage 而非结构体：与 GetCustomer 返回的"原文"语义一致，
// 避免引入 Customer Go 类型与 schema 双向耦合；字段变更不需要改后端。
func ListCustomers(crmDir string) gin.HandlerFunc {
	dir := filepath.Join(crmDir, "crm/customers")
	return func(c *gin.Context) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				L.Info("list customers: dir not found, return empty", zap.String("dir", dir))
				c.JSON(http.StatusOK, []json.RawMessage{})
				return
			}
			L.Error("list customers: read dir failed", zap.String("dir", dir), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read customers: " + err.Error()})
			return
		}

		type entry struct {
			name string
			data json.RawMessage
		}
		pairs := make([]entry, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if filepath.Ext(name) != ".json" {
				continue
			}
			if !strings.HasPrefix(name, customerIDPrefix) {
				continue
			}
			full := filepath.Join(dir, name)
			data, err := os.ReadFile(full)
			if err != nil {
				continue
			}
			if len(data) == 0 {
				continue
			}
			// 用 json.Valid 快速剔除损坏 JSON；有效则原文塞进数组
			if !json.Valid(data) {
				continue
			}
			pairs = append(pairs, entry{name: name, data: data})
		}

		// 按文件名（即 id）升序，输出稳定
		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i].name < pairs[j].name
		})

		out := make([]json.RawMessage, len(pairs))
		for i, p := range pairs {
			out[i] = p.data
		}
		c.JSON(http.StatusOK, out)
	}
}

// PatchCustomer 部分更新客户档案的 basic.contacts 和 basic.phones。
// 请求体接收 string 或 []string 两种形态；至少传一个字段，返回更新后的完整客户 JSON。
func PatchCustomer(crmDir string, safeJoin func(root, rel string) (string, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if !strings.HasPrefix(id, customerIDPrefix) {
			L.Warn("patch customer: non-CUST id rejected", zap.String("id", id))
			c.JSON(http.StatusNotFound, gin.H{"error": "customer " + id + " not found"})
			return
		}
		rel := id + ".json"
		full, err := safeJoin(crmDir+"/crm/customers", rel)
		if err != nil {
			L.Warn("patch customer: safeJoin failed", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 绑定请求体
		var req struct {
			Contacts json.RawMessage `json:"contacts"`
			Phones   json.RawMessage `json:"phones"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			L.Warn("patch customer: invalid body", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
			return
		}
		if len(req.Contacts) == 0 && len(req.Phones) == 0 {
			L.Warn("patch customer: empty body", zap.String("id", id))
			c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of contacts or phones is required"})
			return
		}

		// 验证字段类型：必须是 string 或 []string
		if len(req.Contacts) > 0 {
			if err := validateStringOrArray(req.Contacts); err != nil {
				L.Warn("patch customer: contacts invalid", zap.String("id", id), zap.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{"error": "contacts: " + err.Error()})
				return
			}
		}
		if len(req.Phones) > 0 {
			if err := validateStringOrArray(req.Phones); err != nil {
				L.Warn("patch customer: phones invalid", zap.String("id", id), zap.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{"error": "phones: " + err.Error()})
				return
			}
		}

		// 读取现有客户 JSON
		data, err := readJSONFile(full)
		if err != nil {
			status := http.StatusInternalServerError
			if err.Error() == "file not found" {
				status = http.StatusNotFound
				L.Warn("patch customer: not found", zap.String("id", id))
			} else {
				L.Error("patch customer: read failed", zap.String("id", id), zap.Error(err))
			}
			c.JSON(status, gin.H{"error": "customer " + id + " not found"})
			return
		}

		// 解析为 map 以便更新 basic 子对象
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			L.Error("patch customer: parse failed", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "parse customer: " + err.Error()})
			return
		}

		basic, ok := doc["basic"].(map[string]any)
		if !ok {
			basic = map[string]any{}
			doc["basic"] = basic
		}

		// 记录实际更新的字段
		updatedFields := []string{}
		// 更新 contacts 和/或 phones
		if len(req.Contacts) > 0 {
			var val any
			if err := json.Unmarshal(req.Contacts, &val); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal: parse contacts"})
				return
			}
			basic["contacts"] = val
			updatedFields = append(updatedFields, "contacts")
		}
		if len(req.Phones) > 0 {
			var val any
			if err := json.Unmarshal(req.Phones, &val); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal: parse phones"})
				return
			}
			basic["phones"] = val
			updatedFields = append(updatedFields, "phones")
		}

		// 序列化并原子写回
		updated, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			L.Error("patch customer: marshal failed", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal: " + err.Error()})
			return
		}
		updated = append(updated, '\n')

		tmp := full + ".tmp"
		if err := os.WriteFile(tmp, updated, 0o644); err != nil {
			L.Error("patch customer: write tmp failed", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write customer: " + err.Error()})
			return
		}
		if err := os.Rename(tmp, full); err != nil {
			_ = os.Remove(tmp)
			L.Error("patch customer: rename failed", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "rename: " + err.Error()})
			return
		}

		L.Info("patch customer", zap.String("id", id), zap.Strings("fields", updatedFields))
		c.Data(http.StatusOK, "application/json; charset=utf-8", updated)
	}
}

// validateStringOrArray 校验 json.RawMessage 是 string 或 []string。
func validateStringOrArray(raw json.RawMessage) error {
	if !json.Valid(raw) {
		return errors.New("invalid json")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return nil
	}
	return errors.New("must be a string or array of strings")
}
