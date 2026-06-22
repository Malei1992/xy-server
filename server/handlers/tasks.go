package handlers

// 本文件：代办任务（crm/tasks/*.json）的 GET handler + PATCH status handler。
//
// 端点：
//   - GET  /api/tasks
//   - PATCH /api/tasks/:id/status
//
// GET 数据流：
//  1. 扫 crm/tasks/*.json → ReadTasks
//  2. 按 customer_id 反查 crm/customers/{customer_id}.json → 拿 basic.name
//  3. 合并后返回：任务字段 + customer_name
//
// PATCH 数据流：id 前缀校验 → 状态枚举校验 → 读 → 改 status + updated_at → 原子写回。
//
// 错误约定：
//   - 任务目录不存在 / 空 → 200 + []
//   - customer_id 在客户目录找不到 / 客户文件损坏 / 客户缺字段 → customer_name 空字符串
//   - 损坏的任务文件 → 跳过，不影响其他记录
//   - 任务目录本身 IO 错误（不是 NotExist）→ 500
//   - PATCH:id 不以 TASK 开头 → 400;status 不在枚举内 → 400;id 找不到 / 损坏 → 404

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

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

// validTaskStatuses 是 task status 字段的合法枚举(英文 value)。
// 跟前端 web/src/query/types.ts 的 TaskStatus 保持一致;改前端枚举时这里要同步改。
var validTaskStatuses = map[string]bool{
	"pending":     true,
	"in_progress": true,
	"resolved":    true,
	"dismissed":   true,
}

// PatchTaskStatus 处理 PATCH /api/tasks/:id/status,改 task JSON 的 status 字段。
//   - id 不以 TASK 开头 → 400
//   - body 非法 / status 不在枚举内 → 400
//   - 文件不存在 / 损坏 → 404
//   - 成功 → 200 {ok:true, status:<new>},文件内 status 已变 + updated_at 更新到 now
//
// 写盘由 WriteTask 串行化,handler 不再加锁(避免与 read 死锁)。
func PatchTaskStatus(crmDir, tasksRelDir string) gin.HandlerFunc {
	dir := filepath.Join(crmDir, tasksRelDir)
	return func(c *gin.Context) {
		id := c.Param("id")
		if !strings.HasPrefix(id, taskIDPrefix) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id 必须以 TASK- 开头"})
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
			return
		}
		if !validTaskStatuses[req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status 必须是: pending / in_progress / resolved / dismissed"})
			return
		}

		tasks, err := ReadTasks(dir)
		if err != nil {
			L.Error("patch task: read dir failed", zap.String("dir", dir), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read tasks: " + err.Error()})
			return
		}
		var target *Task
		for i := range tasks {
			if tasks[i].ID == id {
				target = &tasks[i]
				break
			}
		}
		if target == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "task " + id + " not found"})
			return
		}

		target.Status = req.Status
		target.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		if err := WriteTask(dir, *target); err != nil {
			L.Error("patch task: write failed", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write task: " + err.Error()})
			return
		}

		L.Info("patch task status", zap.String("id", id), zap.String("status", req.Status))
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": req.Status})
	}
}
