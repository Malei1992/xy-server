package handlers

// 本文件：公开信息（crm/opportunities/*.json）的 GET handler + PATCH status handler。
//
// 端点：
//   - GET  /api/opportunities
//   - PATCH /api/opportunities/:id/status
//
// GET 数据流：
//  1. 扫 crm/opportunities/*.json → ReadOpportunities
//  2. 按 customer_id 反查 crm/customers/{customer_id}.json → 拿 basic.name
//  3. 合并后返回：opportunity 字段 + customer_name
//
// PATCH 数据流：id 前缀校验 → 状态枚举校验 → 读 → 改 status + updated_at → 原子写回。
//
// 错误约定：
//   - 商机目录不存在 / 空 → 200 + []
//   - customer_id 在客户目录找不到 / 客户文件损坏 / 客户缺字段 → customer_name 空字符串
//   - 损坏的商机文件 → 跳过，不影响其他记录
//   - 商机目录本身 IO 错误（不是 NotExist）→ 500
//   - PATCH:id 不以 OPP 开头 → 400;status 不在枚举内 → 400;id 找不到 / 损坏 → 404

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// OpportunityWithCustomer 列表页返回结构：在 Opportunity 基础上加客户展示字段。
// customer_name 在 customer_id 找不到 / 缺字段时一律为空字符串。
type OpportunityWithCustomer struct {
	ID              string `json:"id"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	OpportunityName string `json:"opportunity_name"`
	CustomerID      string `json:"customer_id,omitempty"`
	CustomerName    string `json:"customer_name"`
	OpportunityInfo string `json:"opportunity_info,omitempty"`
	SourceURL       string `json:"source_url,omitempty"`
	SourceType      string `json:"source_type"`
	Status          string `json:"status"`
	Notes           string `json:"notes,omitempty"`
}

// ListOpportunities 返回所有公开信息（已 join 客户的 customer_name）。
// opportunitiesRelDir / customersRelDir 都是相对 crmDir 的目录路径。
func ListOpportunities(crmDir, opportunitiesRelDir, customersRelDir string) gin.HandlerFunc {
	oppsDir := filepath.Join(crmDir, opportunitiesRelDir)
	customersDir := filepath.Join(crmDir, customersRelDir)
	return func(c *gin.Context) {
		opps, err := ReadOpportunities(oppsDir)
		if err != nil {
			L.Error("list opportunities: read dir failed", zap.String("dir", oppsDir), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read opportunities: " + err.Error()})
			return
		}

		skippedJoin := 0
		out := make([]OpportunityWithCustomer, 0, len(opps))
		for _, o := range opps {
			row := OpportunityWithCustomer{
				ID:              o.ID,
				CreatedAt:       o.CreatedAt,
				UpdatedAt:       o.UpdatedAt,
				OpportunityName: o.OpportunityName,
				CustomerID:      o.CustomerID,
				OpportunityInfo: o.OpportunityInfo,
				SourceURL:       o.SourceURL,
				SourceType:      o.SourceType,
				Status:          o.Status,
				Notes:           o.Notes,
			}
			if o.CustomerID != "" {
				if cust := lookupCustomer(customersDir, o.CustomerID); cust != nil {
					row.CustomerName = cust.Basic.Name
				} else {
					skippedJoin++
					L.Warn("opportunity join: customer missing",
						zap.String("opportunity_id", o.ID),
						zap.String("customer_id", o.CustomerID))
				}
			}
			out = append(out, row)
		}

		L.Info("list opportunities", zap.Int("count", len(out)), zap.Int("skipped_join", skippedJoin))
		c.JSON(http.StatusOK, out)
	}
}

// validOpportunityStatuses 是 opportunity status 字段的合法枚举(中文)。
// 跟前端 web/src/query/types.ts 的 OpportunityStatus 保持一致;改前端枚举时这里要同步改。
var validOpportunityStatuses = map[string]bool{
	"待评估": true,
	"跟进中": true,
	"已转化": true,
	"已关闭": true,
}

// PatchOpportunityStatus 处理 PATCH /api/opportunities/:id/status,改 opportunity JSON 的 status 字段。
//   - id 不以 OPP 开头 → 400
//   - body 非法 / status 不在枚举内 → 400
//   - 文件不存在 / 损坏 → 404
//   - 成功 → 200 {ok:true, status:<new>},文件内 status 已变 + updated_at 更新到 now
//
// 写盘由 WriteOpportunity 串行化,handler 不再加锁(避免与 read 死锁)。
func PatchOpportunityStatus(crmDir, opportunitiesRelDir string) gin.HandlerFunc {
	dir := filepath.Join(crmDir, opportunitiesRelDir)
	return func(c *gin.Context) {
		id := c.Param("id")
		if !strings.HasPrefix(id, opportunityIDPrefix) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id 必须以 OPP- 开头"})
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
			return
		}
		if !validOpportunityStatuses[req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status 必须是: 待评估 / 跟进中 / 已转化 / 已关闭"})
			return
		}

		opps, err := ReadOpportunities(dir)
		if err != nil {
			L.Error("patch opportunity: read dir failed", zap.String("dir", dir), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read opportunities: " + err.Error()})
			return
		}
		var target *Opportunity
		for i := range opps {
			if opps[i].ID == id {
				target = &opps[i]
				break
			}
		}
		if target == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "opportunity " + id + " not found"})
			return
		}

		target.Status = req.Status
		target.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		if err := WriteOpportunity(dir, *target); err != nil {
			L.Error("patch opportunity: write failed", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write opportunity: " + err.Error()})
			return
		}

		L.Info("patch opportunity status", zap.String("id", id), zap.String("status", req.Status))
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": req.Status})
	}
}