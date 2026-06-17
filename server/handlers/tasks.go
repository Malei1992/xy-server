package handlers

// 本文件：代办任务（crm/tasks/*.json）的 GET handler。
//
// 端点：GET /api/tasks
//
// 数据流：
//  1. 扫 crm/tasks/*.json → ReadTasks
//  2. 按 customer_id 反查 crm/customers/{customer_id}.json → 拿 basic.name
//  3. 合并后返回：任务字段 + customer_name
//
// 错误约定：
//   - 任务目录不存在 / 空 → 200 + []
//   - customer_id 在客户目录找不到 / 客户文件损坏 / 客户缺字段 → customer_name 空字符串
//   - 损坏的任务文件 → 跳过，不影响其他记录
//   - 任务目录本身 IO 错误（不是 NotExist）→ 500

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TaskWithCustomer 列表页返回结构：在 Task 基础上加客户展示字段。
// customer_name 在 customer_id 找不到 / 缺字段时一律为空字符串。
type TaskWithCustomer struct {
	ID           string `json:"id"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	Source       string `json:"source,omitempty"`
	Type         string `json:"type"`
	Priority     string `json:"priority"`
	Status       string `json:"status"`
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	CustomerID   string `json:"customer_id,omitempty"`
	CustomerName string `json:"customer_name"`
	EmailID      string `json:"email_id,omitempty"`
	AssignedTo   string `json:"assigned_to,omitempty"`
	ResolvedAt   string `json:"resolved_at,omitempty"`
	Resolution   string `json:"resolution,omitempty"`
}

// ListTasks 返回所有任务（已 join 客户的 customer_name）。
// tasksRelDir / customersRelDir 都是相对 crmDir 的目录路径。
func ListTasks(crmDir, tasksRelDir, customersRelDir string) gin.HandlerFunc {
	tasksDir := filepath.Join(crmDir, tasksRelDir)
	customersDir := filepath.Join(crmDir, customersRelDir)
	return func(c *gin.Context) {
		tasks, err := ReadTasks(tasksDir)
		if err != nil {
			L.Error("list tasks: read dir failed", zap.String("dir", tasksDir), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read tasks: " + err.Error()})
			return
		}

		skippedJoin := 0
		out := make([]TaskWithCustomer, 0, len(tasks))
		for _, t := range tasks {
			row := TaskWithCustomer{
				ID:          t.ID,
				CreatedAt:   t.CreatedAt,
				UpdatedAt:   t.UpdatedAt,
				Source:      t.Source,
				Type:        t.Type,
				Priority:    t.Priority,
				Status:      t.Status,
				Title:       t.Title,
				Description: t.Description,
				CustomerID:  t.CustomerID,
				EmailID:     t.EmailID,
				AssignedTo:  t.AssignedTo,
				ResolvedAt:  t.ResolvedAt,
				Resolution:  t.Resolution,
			}
			if t.CustomerID != "" {
				if cust := lookupCustomer(customersDir, t.CustomerID); cust != nil {
					row.CustomerName = cust.Basic.Name
				} else {
					skippedJoin++
					L.Warn("task join: customer missing",
						zap.String("task_id", t.ID),
						zap.String("customer_id", t.CustomerID))
				}
			}
			out = append(out, row)
		}

		L.Info("list tasks", zap.Int("count", len(out)), zap.Int("skipped_join", skippedJoin))
		c.JSON(http.StatusOK, out)
	}
}
