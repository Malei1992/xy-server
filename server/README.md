# CRM 后端服务

给 CRM Web UI 提供数据 API 的 Go + Gin 服务。

## 跑起来

工作目录必须在 `server/` 下（testdata、go.mod 都在这里）：

```bash
cd server
go run .
```

启动后会打印 4 行：

```
crm-server starting
  listen addr : :15373
  crm data dir: /Users/duwei/ml/mutliAgent/park_acquisition/workspace/data
  env file    : /Users/duwei/ml/mutliAgent/park_acquisition/.env
  version     : 0.1.0
```

健康检查：

```bash
curl http://localhost:15373/healthz
# -> ok
```

## 改路径

**所有路径都是 Go 常量，不读环境变量**。改 `server/paths.go` 里的三个常量即可：

```go
const (
    CRMDataDir = "/Users/duwei/ml/mutliAgent/park_acquisition/workspace/data"      // 数据根目录（含 crm/ 子目录）
    EnvFilePath = "/Users/duwei/ml/mutliAgent/park_acquisition/.env"      // .env 文件
    ListenAddr = ":15373"                                // 监听端口
)
```

部署到生产时改这两个绝对路径 + 端口，编译即可。

## 记录 ID 前缀约定

每个实体目录下的 `.json` 文件名 / ID 必须以约定前缀开头，否则不视为该实体的记录：

| 实体     | 目录                          | 必带前缀  | 来源     |
|----------|-------------------------------|-----------|----------|
| 客户     | `crm/customers/`              | `CUST-`   | schema   |
| 商机     | `crm/projects/`               | `PRJ-`    | 前端生成 |
| 代办任务 | `crm/tasks/`                  | `TASK-`   | Agent 写 |
| 公开信息 | `crm/opportunities/`          | `OPP-`    | 前端生成 |

list 端点（`/api/customers` / `/api/projects` / `/api/tasks` / `/api/opportunities`）会**跳过**不符合前缀的文件，目录里混着 `TEMP.json` / `prj-lowercase.json` 等 stray 文件不会被返回。single 端点 `/api/customers/:id` 在 id 不以 `CUST` 开头时直接 404，防止 stray 文件被当成客户档案返回。

## API 列表

所有响应 `Content-Type: application/json; charset=utf-8`，404 体形如 `{"error":"..."}`。

| Method | Path | 说明 |
|---|---|---|
| GET | `/healthz` | 健康检查，返回 `ok` |
| GET | `/api/index` | `data/crm/index.json` 原文（IndexSummary） |
| GET | `/api/customers` | 列出所有客户档案（`data/crm/customers/*.json` 原文拼成数组，按 id 升序）；目录不存在/空 → `200 []`；损坏/空文件/非 .json/子目录/**不以 `CUST` 开头** → 跳过 |
| GET | `/api/customers/:id` | `data/crm/customers/{id}.json`（Customer）；**`{id}` 必须以 `CUST` 开头**，否则 404 |
| GET | `/api/emails/:id` | `data/crm/emails/{id}.json`（Email） |
| GET | `/api/config` | 已解析的 `.env`，形如 `{"env": {"KEY": "value", ...}}` |
| PATCH | `/api/config` | 修改 `.env` 中的 SMTP/IMAP/邮件审核 keys（白名单校验）；成功返回 `{ok, env, updated}`，原子写（tmp + rename） |
| POST | `/api/uploads/faq` | 上传 FAQ（.doc 或 .docx，≤10MB；.doc 校验 OLE2 magic，.docx 校验 ZIP magic），落盘统一存为 `data/knowledge/FAQ.doc` |
| POST | `/api/uploads/attachment-moonstar` | 上传外宣材料（.pdf），保存到 `data/attachments/MOONSTAR_Investment.pdf` |
| GET | `/api/grading-rules` | 读 `data/crm/grading_rules/enterprise_grade_rules.json`（{A,B,C} → 规则）；文件不存在/为空 → `200 {"A":"","B":"","C":""}`（value level 只有 3 个 keys，**没有 S**）；损坏 → `500` |
| PATCH | `/api/grading-rules` | 替换写 `enterprise_grade_rules.json`，body 必须含 A/B/C 全部 3 个 keys，原子写；不允许 S key（value level 没有 S） |
| GET | `/api/interest-level` | 读 `data/crm/grading_rules/intent_grade_rules.json`（{S,A,B,C} → 规则）；文件不存在/为空 → `200 {"S":"","A":"","B":"","C":""}`；损坏 → `500` |
| PATCH | `/api/interest-level` | 替换写 `intent_grade_rules.json`，body 必须含 S/A/B/C 全部 4 个 keys，原子写 |
| GET | `/api/target-sites` 或 `/api/target-sites?q=xxx` | 读 `data/target_sites.json`（数组，每条 `{name,url,country,industry,type}`）；文件不存在/为空 → `200 []`；`?q=` 按 name 子串模糊过滤；损坏 → `500` |
| POST | `/api/target-sites` | 新增一条；body `{name, url, country?, industry?, type?}`，name+url 必填，name 不可与现有重复（→ 400）；原子写（tmp + rename） |
| PATCH | `/api/target-sites?name=xxx` | 按 name 精确匹配并部分更新；body 只能含 url/country/industry/type（**不能含 name**），name 是 identifier；name 不存在 → 400 |
| DELETE | `/api/target-sites?name=xxx` | 按 name 精确删除；name 不存在 → 400 |
| GET | `/api/projects` | 列出所有商机（`crm/projects/*.json`），并按 `customer_id` 反查 `crm/customers/{id}.json` 合并 `basic.name` / `basic.contacts` 返回。响应字段：`id, created_at, updated_at, project_name, customer_id, customer_name, intent_level, customer_email, status, assigned_to, notes, related_email_ids`。**`intent_level` 来自项目自身的 grade 字段（S/A/B/C 枚举），未填时为空字符串**；customer 字段来自 join。项目目录不存在 / 空 → `200 []`；项目文件损坏 → 跳过；**不以 `PRJ` 开头**的 .json → 跳过；`customer_id` 找不到对应客户 → customer_* 字段为空字符串 |
| GET | `/api/tasks` | 列出所有代办任务（`crm/tasks/*.json`），并按 `customer_id` 反查 `crm/customers/{id}.json` 合并 `basic.name` 返回。响应字段：`id, created_at, updated_at, source, type, priority, status, title, description, customer_id, customer_name, email_id, assigned_to, resolved_at, resolution`。任务目录不存在 / 空 → `200 []`；任务文件损坏 → 跳过；**不以 `TASK` 开头**的 .json → 跳过；`customer_id` 找不到对应客户 / 客户文件损坏 → `customer_name` 为空字符串 |
| GET | `/api/opportunities` | 列出所有公开信息（`crm/opportunities/*.json`），并按 `customer_id` 反查 `crm/customers/{id}.json` 合并 `basic.name` 返回。响应字段：`id, created_at, updated_at, opportunity_name, customer_id, customer_name, opportunity_info, source_url, source_type, status, notes`。`source_type` 枚举：`新闻搜索 / 行业报告 / 招标公告 / 企业公告 / 其他`；`status` 枚举：`待评估 / 跟进中 / 已转化 / 已关闭`。目录不存在 / 空 → `200 []`；文件损坏 → 跳过；**不以 `OPP` 开头**的 .json → 跳过；`customer_id` 找不到对应客户 / 客户文件损坏 → `customer_name` 为空字符串 |

### CORS

开发期对所有 origin 放行（vite 在 5173 调用）。生产用 nginx 收口，不要直接暴露。

### 错误码

- 文件不存在 → `404 {"error": "..."}`
- 路径非法（绝对路径、`..` 等穿越尝试）→ `400 {"error": "..."}`
- 其他 IO 错误 → `500 {"error": "..."}`
- 上传失败（缺文件 / 后缀错 / 超大 / magic bytes 不匹配）→ `400 {"error": "..."}` 或 `413 {"error": "..."}`
- `/api/config` 在 `.env` 缺失时返回 `200 {"env": {}}`（不视为错误）
- PATCH `/api/config`：未知 / 非白名单 key → `400 {"error": "unknown or non-editable key: ..."}`
- PATCH `/api/config`：值非法（端口非数字、EMAIL_REQUIRE_REVIEW 非 true/false/1/0/yes/no、REVIEWER_EMAIL 不含 `@`）→ `400 {"error": "KEY: ..."}`
- PATCH `/api/config`：写文件失败 → `500 {"error": "write .env: ..."}`
- PATCH `/api/grading-rules`（value）：未知 key → `400 {"error": "unknown level key(s): X (allowed: A,B,C)"}`；body 不含全部 A/B/C 三个 keys → `400 {"error": "body must contain all 3 levels [A B C], got N"}`；**S key 不被允许**（value level 没有 S）
- PATCH `/api/interest-level`（intent）：未知 key → `400 {"error": "unknown level key(s): X (allowed: S,A,B,C)"}`；body 不含全部 S/A/B/C 四个 keys → `400 {"error": "body must contain all 4 levels [S A B C], got N"}`
- GET `/api/grading-rules` 文件缺失 → `200 {"A":"","B":"","C":""}`（value level 全空骨架）
- GET `/api/interest-level` 文件缺失 → `200 {"S":"","A":"","B":"","C":""}`（intent level 全空骨架）
- GET / PATCH `/api/grading-rules` / `/api/interest-level`：读/写文件失败 → `500 {"error": "read: ..."}` 或 `500 {"error": "write: ..."}`
- `/api/target-sites`：文件不存在 / 空 / `[]` → 200 + `[]`（不是 404）
- POST `/api/target-sites`：name/url 缺失 → `400 {"error": "name is required"}` / `{"error": "url is required"}`；name 重复 → `400 {"error": "site already exists: ..."}`
- PATCH `/api/target-sites`：缺 `?name=` → `400 {"error": "name query is required"}`；body 含 name → `400 {"error": "cannot modify name via PATCH (name is identifier)"}`；name 不存在 → `400 {"error": "site not found: ..."}`；空 body → `400 {"error": "empty body"}`
- DELETE `/api/target-sites`：缺 `?name=` → `400 {"error": "name query is required"}`；name 不存在 → `400 {"error": "site not found: ..."}`

### 已知限制

- PATCH `/api/config` 写回会**重写整个 `.env`**：去注释、key 按字母序排序、所有值双引号包裹（双引号转义为 `\"`）。如果原 `.env` 里有注释或格式注解，会丢失。
- PATCH 只接受白名单 keys（`SMTP_*` / `IMAP_*` / `EMAIL_REQUIRE_REVIEW` / `REVIEWER_EMAIL`）。改其他 key 用直接编辑文件 + 重启服务。

## 测试

```bash
cd server
go test ./... -v
```

测试用 `testdata/` 下的 fixture，不读真实 `data/crm`，不会污染数据。

## 文件结构

```
server/
├── main.go                # 入口、路由
├── paths.go               # 路径常量
├── env.go                 # parseEnv（与前端 parseEnv.ts 逻辑一致）
├── env_test.go            # parseEnv 单元测试
├── safe_path.go           # 路径安全 helper（防 ../etc/passwd）
├── safe_path_test.go      # safeJoin 单元测试
├── handlers/
│   ├── index.go           # GET /api/index
│   ├── customers.go       # GET /api/customers/:id
│   ├── emails.go          # GET /api/emails/:id
│   ├── config.go          # GET/PATCH /api/config
│   ├── env_io.go          # ParseEnvContent / ReadEnvFile / WriteEnvFile（原子写）
│   ├── level_io.go        # ReadLevelFile / WriteLevelFile（JSON 等级文件，原子写）
│   ├── levels.go          # GET/PATCH /api/{grading-rules,interest-level}
│   ├── upload.go          # POST /api/uploads/{faq,attachment-moonstar}
│   ├── sites.go           # GET/POST/PATCH/DELETE /api/target-sites
│   ├── project_io.go      # Project 类型 + ReadProjects helper（glob crm/projects/*.json）
│   ├── projects.go        # GET /api/projects（关联 join customer 字段）
│   ├── project_io_test.go # ReadProjects 单测
│   ├── task_io.go         # Task 类型 + ReadTasks helper（glob crm/tasks/*.json）
│   ├── task_io_test.go    # ReadTasks 单测
│   ├── tasks.go           # GET /api/tasks（关联 join customer 字段）
│   └── helpers.go         # readJSONFile
├── main_test.go           # 端到端测试（每个端点 happy + 404）
├── config_patch_test.go   # PATCH /api/config 端到端测试（白名单 / 原子写 / 校验）
├── sites_test.go          # /api/target-sites 4 个端点端到端测试
├── projects_test.go       # /api/projects 端到端测试（join customer）
├── customers_list_test.go # /api/customers 端到端测试（聚合）
├── tasks_test.go          # /api/tasks 端到端测试（join customer）
├── test_handlers_test.go  # 测试用 handler 包装器
├── testdata/
│   ├── .env
│   └── crm/
│       ├── index.json
│       ├── customers/{CUST-test-001,002}.json
│       └── emails/MSG-test-001.json
├── go.mod
└── README.md
```

## 没做

按需求明确排除：缓存、聚合、搜索、auth、Docker。