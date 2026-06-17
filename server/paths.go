package main

// 路径常量——部署到生产时只改 BASE_DIR。
// 用户明确要求用 Go 常量而不是环境变量。

const (
	// 项目根目录。CRMDataDir 和 EnvFilePath 均基于此拼接。
	BASE_DIR = "/home/park_acquisition"

	// 数据根目录。CRM 数据位于 <CRMDataDir>/crm/ 下（含 index.json、customers/、emails/）。
	CRMDataDir = BASE_DIR + "/workspace/data"


	// .env 文件绝对路径
	EnvFilePath = BASE_DIR + "/.env"

	// 监听地址（host:port 形式，:15373 表示监听所有网卡 15373 端口）
	ListenAddr = ":15373"

	// 启动时打印的版本号
	Version = "0.1.0"

	// FaqRelPath FAQ 文件路径（相对 CRMDataDir）
	// 落盘固定为 .doc；上传允许 .doc（OLE2 magic）和 .docx（ZIP magic），存为 .doc
	FaqRelPath = "knowledge/FAQ.doc"

	// AttachmentMoonstarRelPath 外宣材料 PDF 路径（相对 CRMDataDir）
	AttachmentMoonstarRelPath = "attachments/MOONSTAR_Investment.pdf"

	// ValueLevelRelPath 客户价值等级标准（JSON 格式：{"A": "规则", "B": "规则", "C": "规则"}）。
	// 注意：value level 只有 A/B/C 三个 level，没有 S。
	ValueLevelRelPath = "crm/grading_rules/enterprise_grade_rules.json"

	// IntentLevelRelPath 客户意向等级标准（JSON 格式：{"S": "规则", "A": "规则", "B": "规则", "C": "规则"}）。
	// intent level 共 S/A/B/C 四个 level。
	IntentLevelRelPath = "crm/grading_rules/intent_grade_rules.json"

	// TargetSitesRelPath 公开数据源列表（JSON 数组：[{"name","url","country","industry","type"}, ...]）。
	// 缺文件返空数组；name 为唯一标识，用于精确和模糊匹配。
	TargetSitesRelPath = "target_sites.json"

	// ProjectsRelDir 商机信息目录（相对 CRMDataDir）。
	// 目录下每个 .json 是一个商机记录（文件名 = "{id}.json"，id 形如 PRJ-{ts}-{hex}）。
	// 缺目录/空目录返空数组；JSON 损坏文件跳过不报错。
	ProjectsRelDir = "crm/projects"

	// CustomersRelDir 客户档案目录（相对 CRMDataDir）。
	// 单个客户文件 = "{customer_id}.json"；用于商机列表按 customer_id 反查客户名称/邮箱/意向等级。
	CustomersRelDir = "crm/customers"

	// TasksRelDir 代办任务目录（相对 CRMDataDir）。
	// 目录下每个 .json 是一个代办任务（文件名 = "{id}.json"，id 形如 TASK-{ts}-{hex}）。
	// 缺目录/空目录返空数组；JSON 损坏文件跳过不报错。
	TasksRelDir = "crm/tasks"

	// OpportunitiesRelDir 公开信息目录（相对 CRMDataDir）。
	// 目录下每个 .json 是一个公开商机记录（文件名 = "{id}.json"，id 形如 OPP-{ts}-{hex}）。
	// 缺目录/空目录返空数组；JSON 损坏文件跳过不报错。
	// 仅读取以 OPP 开头的文件，其余视为 stray 跳过。
	OpportunitiesRelDir = "crm/opportunities"

	// AppLogDir 服务日志目录（绝对路径）。
	// 启动时由 handlers.InitLogger 创建（不存在则 mkdir）；按天写入 app.YYYY-MM-DD.log；保留 30 天。
	AppLogDir = "/opt"

	// DockerContainerName 重启时强删的容器名。
	DockerContainerName = "xingyue-gateway"

	// DockerComposeFile docker compose 配置文件绝对路径。
	DockerComposeFile = BASE_DIR + "/docker-compose.yml"
)
