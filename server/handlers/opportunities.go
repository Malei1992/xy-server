package handlers

// 本文件：公开信息（crm/opportunities/*.json）的 GET handler。
//
// 端点：GET /api/opportunities
//
// 数据流：
//  1. 扫 crm/opportunities/*.json → ReadOpportunities
//  2. 按 customer_id 反查 crm/customers/{customer_id}.json → 拿 basic.name
//  3. 合并后返回：opportunity 字段 + customer_name
//
// 错误约定：
//   - 商机目录不存在 / 空 → 200 + []
//   - customer_id 在客户目录找不到 / 客户文件损坏 / 客户缺字段 → customer_name 空字符串
//   - 损坏的商机文件 → 跳过，不影响其他记录
//   - 商机目录本身 IO 错误（不是 NotExist）→ 500

import (
	"net/http"
	"path/filepath"

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