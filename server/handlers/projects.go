package handlers

// 本文件：商机信息（crm/projects/*.json）的 GET handler。
//
// 端点：GET /api/projects
//
// 数据流：
//  1. 扫 crm/projects/*.json → ReadProjects
//  2. 按 customer_id 反查 crm/customers/{customer_id}.json → 拿 basic.name / basic.contacts
//  3. 合并后返回：项目字段 + customer_name / customer_email
//
// 意向等级（intent_level）来自项目自身的 intent_level 字段（S / A / B / C 枚举），不再读
// 客户档案的 engagement.intent_level。
//
// 错误约定：
//   - 项目目录不存在 / 空 → 200 + []
//   - customer_id 在客户目录找不到 / 客户文件损坏 / 客户缺字段 → 该条 customer_* 字段空字符串
//   - 损坏的项目文件 → 跳过，不影响其他记录
//   - 项目目录本身 IO 错误（不是 NotExist）→ 500

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ProjectWithCustomer 列表页返回结构：在 Project 基础上加客户侧的展示字段。
// 2 个客户字段在缺数据时一律为空字符串（前端可直接展示，无需做 null 判断）。
// 意向等级 intent_level 在项目自身无 intent_level 字段时为空字符串。
type ProjectWithCustomer struct {
	ID              string   `json:"id"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	ProjectName     string   `json:"project_name"`
	CustomerID      string   `json:"customer_id"`
	CustomerName    string   `json:"customer_name"`
	IntentLevel     string   `json:"intent_level"`
	CustomerEmail   string   `json:"customer_email"`
	Status          string   `json:"status"`
	AssignedTo      string   `json:"assigned_to,omitempty"`
	Notes           string   `json:"notes,omitempty"`
	RelatedEmailIDs []string `json:"related_email_ids,omitempty"`
}

// customerLookup 是反查客户时实际解析的最小结构。
// 不引入完整 Customer 类型，避免耦合字段；缺字段一律为空字符串。
// 注意：不读 engagement.intent_level——意向等级来自项目自身。
// Contacts 用 json.RawMessage，因为实际数据中 contacts 可能是 string 或 []string，
// 用具体类型会导致 unmarshal 失败，进而丢弃整个 customer（含 Name）。
type customerLookup struct {
	Basic struct {
		Name     string          `json:"name"`
		Contacts json.RawMessage `json:"contacts"`
	} `json:"basic"`
}

// extractFirstContact 从 basic.contacts 的原始 JSON 中提取第一个联系邮箱。
// 兼容 string（"a@b.com"）和 []string（["a@b.com","c@d.com"]）两种实际数据形态。
// 无法解析时返回空字符串。
func extractFirstContact(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// 尝试作为字符串解析
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// 尝试作为数组解析，取第一个元素
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr[0]
	}
	return ""
}

// GetProjects 返回所有商机（已 join 客户的展示字段）。
// projectsRelDir / customersRelDir 都是相对 crmDir 的目录路径。
func GetProjects(crmDir, projectsRelDir, customersRelDir string) gin.HandlerFunc {
	projectsDir := filepath.Join(crmDir, projectsRelDir)
	customersDir := filepath.Join(crmDir, customersRelDir)
	return func(c *gin.Context) {
		projects, err := ReadProjects(projectsDir)
		if err != nil {
			L.Error("get projects: read dir failed", zap.String("dir", projectsDir), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read projects: " + err.Error()})
			return
		}

		skippedJoin := 0
		out := make([]ProjectWithCustomer, 0, len(projects))
		for _, p := range projects {
			row := ProjectWithCustomer{
				ID:              p.ID,
				CreatedAt:       p.CreatedAt,
				UpdatedAt:       p.UpdatedAt,
				ProjectName:     p.ProjectName,
				CustomerID:      p.CustomerID,
				IntentLevel:     p.IntentLevel,
				Status:          p.Status,
				AssignedTo:      p.AssignedTo,
				Notes:           p.Notes,
				RelatedEmailIDs: p.RelatedEmailIDs,
			}
			if p.CustomerID != "" {
				if cust := lookupCustomer(customersDir, p.CustomerID); cust != nil {
					row.CustomerName = cust.Basic.Name
					row.CustomerEmail = extractFirstContact(cust.Basic.Contacts)
				} else {
					skippedJoin++
					L.Warn("project join: customer missing",
						zap.String("project_id", p.ID),
						zap.String("customer_id", p.CustomerID))
				}
			}
			out = append(out, row)
		}

		L.Info("get projects", zap.Int("count", len(out)), zap.Int("skipped_join", skippedJoin))
		c.JSON(http.StatusOK, out)
	}
}

// lookupCustomer 读 customersDir/{customerID}.json 并解析为最小 customerLookup。
// 找不到文件 / 损坏 / 缺字段 → 返 nil（调用方把 customer 字段留空）。
func lookupCustomer(customersDir, customerID string) *customerLookup {
	full := filepath.Join(customersDir, customerID+".json")
	data, err := os.ReadFile(full)
	if err != nil {
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	var cust customerLookup
	if err := json.Unmarshal(data, &cust); err != nil {
		return nil
	}
	return &cust
}

// validProjectStatuses 是 project status 字段的合法枚举。
// 跟前端 web/src/query/types.ts 的 ProjectStatus 保持一致;改前端枚举时这里要同步改。
var validProjectStatuses = map[string]bool{
	"跟进中": true,
	"谈判中": true,
	"签约中": true,
	"已落地": true,
	"已关闭": true,
}

// PatchProjectStatus 处理 PATCH /api/projects/:id/status,改 project JSON 的 status 字段。
//   - id 不以 PRJ 开头 → 400(防 path traversal / 写错目录)
//   - body 非法 / status 不在枚举内 → 400
//   - 文件不存在 / 损坏 → 404(对齐 spec:读失败或 unmarshal 失败 → 404)
//   - 成功 → 200 {ok:true, status:<new>},文件内 status 已变 + updated_at 更新到 now
//
// 写盘流程对齐 openclaw_config.go:.tmp + rename,失败清理 .tmp。
// 串行化由 WriteProject 内部 projectsMu 保证,handler 不再加锁(避免与 read 死锁)。
func PatchProjectStatus(crmDir, projectsRelDir string) gin.HandlerFunc {
	dir := filepath.Join(crmDir, projectsRelDir)
	return func(c *gin.Context) {
		id := c.Param("id")
		if !strings.HasPrefix(id, projectIDPrefix) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id 必须以 PRJ- 开头"})
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
			return
		}
		if !validProjectStatuses[req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status 必须是: 跟进中 / 谈判中 / 签约中 / 已落地 / 已关闭"})
			return
		}

		// 读 → 改 → 写。ReadProjects 会跳过损坏 / 不以 PRJ 开头的文件,id 不存在 → 返空 slice
		projects, err := ReadProjects(dir)
		if err != nil {
			L.Error("patch project: read dir failed", zap.String("dir", dir), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read projects: " + err.Error()})
			return
		}
		var target *Project
		for i := range projects {
			if projects[i].ID == id {
				target = &projects[i]
				break
			}
		}
		if target == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "project " + id + " not found"})
			return
		}

		target.Status = req.Status
		target.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		if err := WriteProject(dir, *target); err != nil {
			L.Error("patch project: write failed", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write project: " + err.Error()})
			return
		}

		L.Info("patch project status", zap.String("id", id), zap.String("status", req.Status))
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": req.Status})
	}
}
