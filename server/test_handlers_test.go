package main

// 本文件：暴露给 *_test.go 用的 handler 包装器。
// 真实路由在 main.go 走常量；测试路由需要可注入路径，所以这里提供构造函数，
// 直接复用 internal/handlers 包。

import (
	"github.com/gin-gonic/gin"

	"crm-server/handlers"
)

func indexHandlerForTest(crmDir string) gin.HandlerFunc {
	return handlers.GetIndex(crmDir)
}

func customerHandlerForTest(crmDir string) gin.HandlerFunc {
	return handlers.GetCustomer(crmDir, safeJoin)
}

func patchCustomerHandlerForTest(crmDir string) gin.HandlerFunc {
	return handlers.PatchCustomer(crmDir, safeJoin)
}

func listCustomersHandlerForTest(crmDir string) gin.HandlerFunc {
	return handlers.ListCustomers(crmDir)
}

func emailHandlerForTest(crmDir string) gin.HandlerFunc {
	return handlers.GetEmail(crmDir, safeJoin)
}

func configHandlerForTest(envPath string) gin.HandlerFunc {
	return handlers.GetConfig(envPath, parseEnv)
}

func patchConfigHandlerForTest(envPath string) gin.HandlerFunc {
	return handlers.PatchConfig(envPath)
}

func uploadFaqHandlerForTest(crmDir string) gin.HandlerFunc {
	return handlers.PostFaq(crmDir, FaqRelPath)
}

func uploadAttachmentHandlerForTest(crmDir string) gin.HandlerFunc {
	return handlers.PostAttachmentMoonstar(crmDir, "attachments/MOONSTAR_Investment.pdf")
}

func getLevelsHandlerForTest(crmDir, rel string, defaultLevels []string) gin.HandlerFunc {
	return handlers.GetLevels(crmDir, rel, defaultLevels)
}

func patchLevelsHandlerForTest(crmDir, rel string, defaultLevels []string) gin.HandlerFunc {
	return handlers.PatchLevels(crmDir, rel, defaultLevels)
}

func getSitesHandlerForTest(crmDir, rel string) gin.HandlerFunc {
	return handlers.GetSites(crmDir, rel)
}

func postSiteHandlerForTest(crmDir, rel string) gin.HandlerFunc {
	return handlers.PostSite(crmDir, rel)
}

func patchSiteHandlerForTest(crmDir, rel string) gin.HandlerFunc {
	return handlers.PatchSite(crmDir, rel)
}

func deleteSiteHandlerForTest(crmDir, rel string) gin.HandlerFunc {
	return handlers.DeleteSite(crmDir, rel)
}

func getProjectsHandlerForTest(crmDir, projectsRelDir, customersRelDir string) gin.HandlerFunc {
	return handlers.GetProjects(crmDir, projectsRelDir, customersRelDir)
}

func listTasksHandlerForTest(crmDir, tasksRelDir, customersRelDir string) gin.HandlerFunc {
	return handlers.ListTasks(crmDir, tasksRelDir, customersRelDir)
}

func listOpportunitiesHandlerForTest(crmDir, opportunitiesRelDir, customersRelDir string) gin.HandlerFunc {
	return handlers.ListOpportunities(crmDir, opportunitiesRelDir, customersRelDir)
}

func postRestartHandlerForTest(commands [][]string) gin.HandlerFunc {
	return handlers.PostRestart(commands)
}