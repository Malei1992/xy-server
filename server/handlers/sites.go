package handlers

// 本文件：公开数据源（target_sites.json）的 4 个 CRUD handler。
//
// 4 个端点：
//   GET    /api/target-sites[?q=xxx]   列出全部或按 name 子串模糊查询
//   POST   /api/target-sites            新增；body {name, url, country?, industry?, type?}
//                                       name + url 必填；name 不能与现有重复
//   PATCH  /api/target-sites?name=xxx   按 name 精确匹配并部分更新
//                                       body 不能含 name（name 是 identifier，不允许改）
//   DELETE /api/target-sites?name=xxx   按 name 精确删除
//
// 错误码：
//   400  body 缺 name/url、name 重复、query 缺 name、待改/待删的 name 不存在、body 含 name
//   500  读 / 写文件失败
//
// 缺文件 / 空文件 / "[]" → GET 返 200 + []，其他操作视作 0 条。

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetSites 列出全部 sites；可选 ?q=xxx 按 name 子串模糊过滤。
// 缺文件 / 空文件 → 200 + []。
// 损坏的 JSON → 500。
func GetSites(crmDir, relPath string) gin.HandlerFunc {
	fullPath := filepath.Join(crmDir, relPath)
	return func(c *gin.Context) {
		sites, err := ReadSites(fullPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read: " + err.Error()})
			return
		}
		q := c.Query("q")
		filtered := FilterSitesByQuery(sites, q)
		c.JSON(http.StatusOK, filtered)
	}
}

// PostSite 新增一条 site。
// 校验：name + url 必填（任一空 → 400），name 不能与现有重复（→ 400）。
// 成功 → 200 + 写入后的全部 sites。
func PostSite(crmDir, relPath string) gin.HandlerFunc {
	fullPath := filepath.Join(crmDir, relPath)
	return func(c *gin.Context) {
		var body TargetSite
		if err := c.ShouldBindJSON(&body); err != nil {
			L.Warn("post site: invalid body", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body: " + err.Error()})
			return
		}
		if body.Name == "" {
			L.Warn("post site: name empty")
			c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
			return
		}
		if body.URL == "" {
			L.Warn("post site: url empty", zap.String("name", body.Name))
			c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
			return
		}

		sites, err := ReadSites(fullPath)
		if err != nil {
			L.Error("post site: read failed", zap.String("name", body.Name), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read: " + err.Error()})
			return
		}
		if FindSiteByName(sites, body.Name) != nil {
			L.Warn("post site: duplicate name", zap.String("name", body.Name))
			c.JSON(http.StatusBadRequest, gin.H{"error": "site already exists: " + body.Name})
			return
		}

		sites = append(sites, body)
		if err := writeSitesAtomic(fullPath, sites); err != nil {
			L.Error("post site: write failed", zap.String("name", body.Name), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write: " + err.Error()})
			return
		}
		L.Info("post site", zap.String("name", body.Name))
		c.JSON(http.StatusOK, body)
	}
}

// PatchSite 按 ?name=xxx 精确匹配并部分更新。
// 校验：query 必须有 name（→ 400），body 不含 name（→ 400，因为 name 是 identifier），
// 待改的 name 必须存在（→ 400），body 不能为空（→ 400）。
func PatchSite(crmDir, relPath string) gin.HandlerFunc {
	fullPath := filepath.Join(crmDir, relPath)
	return func(c *gin.Context) {
		name := c.Query("name")
		if name == "" {
			L.Warn("patch site: missing name query")
			c.JSON(http.StatusBadRequest, gin.H{"error": "name query is required"})
			return
		}

		var body map[string]any
		if err := c.ShouldBindJSON(&body); err != nil {
			L.Warn("patch site: invalid body", zap.String("name", name), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body: " + err.Error()})
			return
		}
		if len(body) == 0 {
			L.Warn("patch site: empty body", zap.String("name", name))
			c.JSON(http.StatusBadRequest, gin.H{"error": "empty body"})
			return
		}
		if _, hasName := body["name"]; hasName {
			L.Warn("patch site: attempt to modify name", zap.String("name", name))
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot modify name via PATCH (name is identifier)"})
			return
		}

		sites, err := ReadSites(fullPath)
		if err != nil {
			L.Error("patch site: read failed", zap.String("name", name), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read: " + err.Error()})
			return
		}
		idx := -1
		for i := range sites {
			if sites[i].Name == name {
				idx = i
				break
			}
		}
		if idx < 0 {
			L.Warn("patch site: name not found", zap.String("name", name))
			c.JSON(http.StatusBadRequest, gin.H{"error": "site not found: " + name})
			return
		}

		// 只更新 body 里有的字段（不允许通过 PATCH 改 name，上面已 reject）
		if v, ok := body["url"].(string); ok {
			sites[idx].URL = v
		}
		if v, ok := body["country"].(string); ok {
			sites[idx].Country = v
		}
		if v, ok := body["industry"].(string); ok {
			sites[idx].Industry = v
		}
		if v, ok := body["type"].(string); ok {
			sites[idx].Type = v
		}

		if err := writeSitesAtomic(fullPath, sites); err != nil {
			L.Error("patch site: write failed", zap.String("name", name), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write: " + err.Error()})
			return
		}
		L.Info("patch site", zap.String("name", name))
		c.JSON(http.StatusOK, sites[idx])
	}
}

// DeleteSite 按 ?name=xxx 精确删除。
// 校验：query 必须有 name（→ 400），name 必须存在（→ 400）。
func DeleteSite(crmDir, relPath string) gin.HandlerFunc {
	fullPath := filepath.Join(crmDir, relPath)
	return func(c *gin.Context) {
		name := c.Query("name")
		if name == "" {
			L.Warn("delete site: missing name query")
			c.JSON(http.StatusBadRequest, gin.H{"error": "name query is required"})
			return
		}

		sites, err := ReadSites(fullPath)
		if err != nil {
			L.Error("delete site: read failed", zap.String("name", name), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read: " + err.Error()})
			return
		}
		idx := -1
		for i := range sites {
			if sites[i].Name == name {
				idx = i
				break
			}
		}
		if idx < 0 {
			L.Warn("delete site: name not found", zap.String("name", name))
			c.JSON(http.StatusBadRequest, gin.H{"error": "site not found: " + name})
			return
		}

		// 保持顺序删除
		sites = append(sites[:idx], sites[idx+1:]...)
		if err := writeSitesAtomic(fullPath, sites); err != nil {
			L.Error("delete site: write failed", zap.String("name", name), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write: " + err.Error()})
			return
		}
		L.Info("delete site", zap.String("name", name))
		c.JSON(http.StatusOK, gin.H{"deleted": name})
	}
}

// writeSitesAtomic 原子写 site list；父目录自动建。
func writeSitesAtomic(fullPath string, sites []TargetSite) error {
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return WriteSites(fullPath, sites)
}
