package main

import (
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"crm-server/handlers"
)

func main() {
	// 初始化结构化日志到 /opt/app.YYYY-MM-DD.log
	if err := handlers.InitLogger(AppLogDir); err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer func() { _ = handlers.L.Sync() }()

	// 启动日志：5 行关键信息（监听地址 / 数据目录 / env 路径 / 版本 / uploads 目录）
	handlers.L.Info("crm-server starting")
	handlers.L.Info("boot config",
		zap.String("listen_addr", ListenAddr),
		zap.String("crm_data_dir", CRMDataDir),
		zap.String("env_file", EnvFilePath),
		zap.String("version", Version),
		zap.String("uploads_dir", filepath.Dir(filepath.Join(CRMDataDir, FaqRelPath))),
	)

	// 用 gin.New() 替换 gin.Default() 避免默认 stdout logger
	// 显式注册 Recovery + RequestLogger + CORS
	r := gin.New()
	r.Use(gin.Recovery(), handlers.RequestLogger(), cors.Default())

	// 健康检查
	r.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// API 路由
	api := r.Group("/api")
	{
		api.GET("/index", handlers.GetIndex(CRMDataDir))
		api.GET("/customers", handlers.ListCustomers(CRMDataDir))
		api.GET("/customers/:id", handlers.GetCustomer(CRMDataDir, safeJoin))
			api.PATCH("/customers/:id", handlers.PatchCustomer(CRMDataDir, safeJoin))
		api.GET("/emails/:id", handlers.GetEmail(CRMDataDir, safeJoin))
		api.GET("/config", handlers.GetConfig(EnvFilePath, parseEnv))
		api.PATCH("/config", handlers.PatchConfig(EnvFilePath))
		api.POST("/uploads/faq", handlers.PostFaq(CRMDataDir, FaqRelPath))
		api.POST("/uploads/attachment-moonstar", handlers.PostAttachmentMoonstar(CRMDataDir, AttachmentMoonstarRelPath))
		// 客户价值等级标准：A/B/C 三个 level，没有 S
		valueLevels := []string{"A", "B", "C"}
		// 客户意向等级标准：S/A/B/C 四个 level
		intentLevels := []string{"S", "A", "B", "C"}

		api.GET("/grading-rules", handlers.GetLevels(CRMDataDir, ValueLevelRelPath, valueLevels))
		api.PATCH("/grading-rules", handlers.PatchLevels(CRMDataDir, ValueLevelRelPath, valueLevels))
		api.GET("/interest-level", handlers.GetLevels(CRMDataDir, IntentLevelRelPath, intentLevels))
		api.PATCH("/interest-level", handlers.PatchLevels(CRMDataDir, IntentLevelRelPath, intentLevels))
		// 公开数据源：target_sites.json 读写
		api.GET("/target-sites", handlers.GetSites(CRMDataDir, TargetSitesRelPath))
		api.POST("/target-sites", handlers.PostSite(CRMDataDir, TargetSitesRelPath))
		api.PATCH("/target-sites", handlers.PatchSite(CRMDataDir, TargetSitesRelPath))
		api.DELETE("/target-sites", handlers.DeleteSite(CRMDataDir, TargetSitesRelPath))
		// 商机信息：crm/projects/*.json + crm/customers/{id}.json 关联 join
		api.GET("/projects", handlers.GetProjects(CRMDataDir, ProjectsRelDir, CustomersRelDir))
		// 代办任务：crm/tasks/*.json + crm/customers/{id}.json 关联 join
		api.GET("/tasks", handlers.ListTasks(CRMDataDir, TasksRelDir, CustomersRelDir))
		// 公开信息：crm/opportunities/*.json + crm/customers/{id}.json 关联 join
		api.GET("/opportunities", handlers.ListOpportunities(CRMDataDir, OpportunitiesRelDir, CustomersRelDir))
		// 重启服务:先强删旧容器,再 docker compose up -d 拉起新容器
		restartCmds := [][]string{
			{"docker", "rm", "-f", DockerContainerName},
			{"docker", "compose", "-f", DockerComposeFile, "up", "-d"},
		}
		api.POST("/restart", handlers.PostRestart(restartCmds))
		// 微信绑定:在 gateway 容器内启动 openclaw 微信登录,返回二维码 + 链接
		wechatBindCmds := [][]string{
			{"docker", "exec", DockerContainerName, "bash", "-c", "openclaw channels login --channel openclaw-weixin"},
		}
		api.POST("/wechat/bind", handlers.PostWechatBind(wechatBindCmds, 2*time.Minute))
		// 微信绑定状态轮询:前端拿到 task_id 后反复 GET 此接口拿结果。
		api.GET("/wechat/bind/:task_id", handlers.GetWechatBindStatus())
	}

	addr := ListenAddr
	if err := r.Run(addr); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
