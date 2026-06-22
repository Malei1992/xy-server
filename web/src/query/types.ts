// CRM 数据的严格类型定义，严格匹配 schema.md
// 数据变体在字段处用 union 标注，运行时校验放在 loader 处处理

// ----- 枚举 -----

export type Status =
  | "active"
  | "dormant"
  | "closed"
  | "invalid"
  | "needs_manual_review";

export type IntentLevel = "S" | "A" | "B" | "C" | "" | "unknown";

export type Grade = "A" | "B" | "C";

export type Risk = "低" | "中" | "高" | "禁止" | "待定";

export type DemandStrength = "强" | "中" | "弱" | "无" | "数据不足";

export type ParkFitRating = "A" | "B" | "C" | "N/A";

// 合规维度评级。schema.md 中各维度枚举不同（外资准入 3 值、税务 3 值、劳工 3 值、
// 外汇 3 值、行业壁垒 3 值），用 string 承载；
// 实际数据中存在两套字段命名：
//   - 老数据：`rating` + `detail`
//   - 新数据：`level`  + `note`
// 运行时按"哪个有就读哪个"处理；类型上同时声明两个以便通过 TypeScript 检查
export type ComplianceLevel = {
  rating?: string;
  detail?: string;
  level?: string;
  note?: string;
};

export interface ComplianceRisk {
  foreign_investment?: ComplianceLevel;
  tax?: ComplianceLevel;
  labor?: ComplianceLevel;
  forex?: ComplianceLevel;
  industry_barrier?: ComplianceLevel;
}

// ----- 客户 -----

export interface Basic {
  name: string;
  country: string;
  industry: string;
  // schema.md 标为 string[]；实际数据中常见 string 与 "" 形态
  contacts: string | string[];
  phones: string | string[];
  scale?: string;
  sector?: string;
  website?: string;
}

export interface Prospecting {
  grade?: Grade;
  grade_note?: string;
  overall_risk?: Risk;
  park_fit_rating?: ParkFitRating;
  park_fit_note?: string;
  demand_strength?: DemandStrength;
  screening_basis?: string;
  screening_justification?: string;
  screening_reasons?: string[];
  prohibited_industries?: string[];
  compliance_risk?: ComplianceRisk;
  source_url?: string;
  source_site?: string;
  source_extracted_at?: string;
  investment_history?: string;
  land_deal?: string;
}

export interface Profile {
  company_strength?: "强" | "中" | "弱";
  industry_match_score?: number;
  overseas_expansion_likelihood?: "强" | "中" | "弱";
  cooperation_potential?: number;
  notes?: string;
}

export interface Analysis {
  profile?: Profile;
  advantage_matches?: { park_advantage: string; fit_explanation: string }[];
  email_script?: string;
  key_talking_points?: string[];
  negotiation_notes?: string[];
  compliance_cultural_notes?: string[];
  recommended_action?: string;
  urgency_level?: "高" | "中" | "低";
  analyzed_at?: string;
}

export interface Engagement {
  status: Status;
  intent_level?: IntentLevel;
  key_questions?: string[];
  concerns?: string[];
  last_email_id?: string;
  assigned_to?: string;
  analysis?: Analysis;
  decision_maker?: string;
  last_contact?: string;
}

export interface TimelineEvent {
  event: string;
  by: string;
  at: string;
  detail?: string;
  email_id?: string;
  note?: string;
}

export interface Customer {
  id: string;
  created_at?: string;
  updated_at?: string;
  basic: Basic;
  prospecting?: Prospecting;
  engagement?: Engagement;
  timeline: TimelineEvent[];
  last_email_id?: string;
}

// ----- 邮件 -----

export type EmailStatus =
  | "pending"
  | "pending_review"
  | "approved"
  | "rejected"
  | "sent"
  | "send_failed"
  | "processed";

export type EmailDirection = "sent" | "received";

export type EmailType =
  | "invitation"
  | "intro"
  | "reply_invitation"
  | "reply_consultation"
  | "incoming"
  | "nurture";

export type EmailLanguage = "th" | "vi" | "en" | "zh" | "ar";

export interface ReviewHistoryEntry {
  version: number;
  action: "pending_review" | "revised" | "approved" | "rejected";
  content_chinese?: string;
  content_target?: string;
  content?: string;
  reviewer?: string;
  note?: string;
}

export interface Email {
  id: string;
  direction: EmailDirection;
  customer_id: string;
  type: EmailType;
  subject: string;
  language: EmailLanguage;
  status: EmailStatus;
  from?: string;
  reply_to?: string;
  summary_zh?: unknown;
  classification?: "有意向" | "咨询" | "拒绝" | "无效回复" | "垃圾邮件";
  send_attempts?: number;
  review_history?: ReviewHistoryEntry[];
  created_at: string;
  updated_at?: string;
}

// ----- 索引与筛选 -----

export interface IndexSummary {
  customers: string[];
  // 扩展字段：实际由外部服务在 index.json 中维护，本期邮件查询依赖
  // schema.md 中未定义，本字段为可选，缺失时 listEmails 返回空
  emails?: string[];
  by_status: Partial<Record<Status, string[]>>;
  by_intent: Partial<Record<IntentLevel, string[]>>;
  by_country: Record<string, string[]>;
}

export interface CustomerFilter {
  status?: Status[];
  country?: string[];
  intentLevel?: IntentLevel[];
  grade?: Grade[];
}

export interface EmailFilter {
  customerId?: string;
  type?: EmailType[];
  status?: EmailStatus[];
  direction?: EmailDirection[];
}

// ----- 公开数据源 -----

// 单条公开数据源。
// 5 个字段：name（必填，唯一标识）/ url（必填）/ country / industry / type。
// 后三个为可选项（后端用 omitempty 序列化；空字符串不会出现在 JSON 中）。
// 与后端 handlers/sites.go 的 TargetSite 一一对应。
export interface Site {
  name: string;
  url: string;
  country?: string;
  industry?: string;
  type?: string;
}

// ----- 商机信息 -----

// 项目跟进状态枚举。状态机由后端 / 业务决定，前端只展示。
export type ProjectStatus = "跟进中" | "谈判中" | "签约中" | "已落地" | "已关闭";

// 项目状态下拉数据源(spec 2026-06-22 表格)
// - 顺序 = 状态机推进顺序(跟进中 → 谈判中 → 签约中 → 已落地 → 已关闭)
// - label 与 value 相同(项目状态本身就是中文)
// - 用 ReadonlyArray + as const 保证类型推断稳定
export const PROJECT_STATUS_OPTIONS: ReadonlyArray<{ value: ProjectStatus; label: string }> = [
  { value: "跟进中", label: "跟进中" },
  { value: "谈判中", label: "谈判中" },
  { value: "签约中", label: "签约中" },
  { value: "已落地", label: "已落地" },
  { value: "已关闭", label: "已关闭" },
] as const;

// 单条商机记录（已 join 客户的展示字段）。
// 与后端 handlers/projects.go 的 ProjectWithCustomer 一一对应。
// customer_name / customer_email 在 customer_id 找不到客户时为空字符串。
// 意向等级 intent_level 来自项目自身（S / A / B / C 枚举）；项目未填时为 ""（前端展示「无」）。
export interface Project {
  id: string;
  created_at: string;
  updated_at: string;
  project_name: string;
  customer_id: string;
  customer_name: string;
  intent_level: IntentLevel;
  customer_email: string;
  status: ProjectStatus;
  assigned_to?: string;
  notes?: string;
  related_email_ids?: string[];
}

// ----- 代办任务 -----

// 任务来源 Agent / 系统枚举。与 schema.md 中 task.source 一致。
export type TaskSource =
  | "ProspectorAgent"
  | "CourierAgent"
  | "AnalystAgent"
  | "System";

// 任务类型枚举。与 schema.md 中 task.type 一致。
// 8 个值：data_insufficient / compliance_blocked / llm_failure /
//        human_notify_failed / review_timeout / complex_inquiry /
//        anomaly_alert / low_confidence / send_failed。
export type TaskType =
  | "data_insufficient"
  | "compliance_blocked"
  | "llm_failure"
  | "human_notify_failed"
  | "review_timeout"
  | "complex_inquiry"
  | "anomaly_alert"
  | "low_confidence"
  | "send_failed";

// 任务等级枚举。4 档：P0 严重告警 / P1 高优 / P2 普通 / P3 低优。
export type TaskPriority = "P0" | "P1" | "P2" | "P3";

// 任务状态枚举。状态机：pending → in_progress → resolved/dismissed。
export type TaskStatus = "pending" | "in_progress" | "resolved" | "dismissed";

// 任务状态下拉数据源(spec 2026-06-22 表格)
// - value 是英文 enum(API 传输用)
// - label 是中文(UI 展示用,跟 spec 表格一致)
export const TASK_STATUS_OPTIONS: ReadonlyArray<{ value: TaskStatus; label: string }> = [
  { value: "pending", label: "待处理" },
  { value: "in_progress", label: "处理中" },
  { value: "resolved", label: "已解决" },
  { value: "dismissed", label: "已驳回" },
] as const;

// 任务状态英文 → 中文 的全局映射,方便列表页 cell 展示。
// 跟 TASK_STATUS_OPTIONS 的 label 完全一致(放这里方便「只知道 enum,想转 label」的场景)。
export const TASK_STATUS_LABELS: Record<TaskStatus, string> = {
  pending: "待处理",
  in_progress: "处理中",
  resolved: "已解决",
  dismissed: "已驳回",
};

// 单条代办任务记录（已 join 客户的展示字段）。
// 与后端 handlers/tasks.go 的 TaskWithCustomer 一一对应。
// customer_name 在 customer_id 找不到客户时为空字符串（前端展示「无」）。
// priority 用前端 PriorityBadge 渲染为圆形徽章（P0 红 / P1 黄 / P2 绿 / P3 灰）。
export interface Task {
  id: string;
  created_at: string;
  updated_at: string;
  source?: TaskSource;
  type: TaskType;
  priority: TaskPriority;
  status: TaskStatus;
  title: string;
  description?: string;
  customer_id?: string;
  customer_name: string;
  email_id?: string;
  assigned_to?: string;
  resolved_at?: string;
  resolution?: string;
}

// ----- 公开信息 -----

// 公开信息来源类型枚举。
// 与后端 handlers/opportunities.go 的 OpportunityWithCustomer.source_type 一致。
// 5 个值：新闻搜索 / 行业报告 / 招标公告 / 企业公告 / 其他。
export type OpportunitySourceType =
  | "新闻搜索"
  | "行业报告"
  | "招标公告"
  | "企业公告"
  | "其他";

// 公开信息状态枚举。
// 与后端 handlers/opportunities.go 的 OpportunityWithCustomer.status 一致。
// 4 个值：待评估 / 跟进中 / 已转化 / 已关闭。
export type OpportunityStatus =
  | "待评估"
  | "跟进中"
  | "已转化"
  | "已关闭";

// 公开信息状态下拉数据源(spec 2026-06-22 表格)
export const OPPORTUNITY_STATUS_OPTIONS: ReadonlyArray<{ value: OpportunityStatus; label: string }> = [
  { value: "待评估", label: "待评估" },
  { value: "跟进中", label: "跟进中" },
  { value: "已转化", label: "已转化" },
  { value: "已关闭", label: "已关闭" },
] as const;

// 单条公开信息记录（已 join 客户的展示字段）。
// 与后端 handlers/opportunities.go 的 OpportunityWithCustomer 一一对应。
// customer_name 在 customer_id 找不到客户时为空字符串（前端展示「—」）。
// 可选字段（customer_id / opportunity_info / source_url / notes）缺失时
// 在前端展示「无」。
// id 由后端生成，格式 OPP-{timestamp_ms}-{random_hex}（仅以 OPP 前缀开头才读取）。
export interface Opportunity {
  id: string;
  created_at: string;
  updated_at: string;
  opportunity_name: string;
  customer_id?: string;
  customer_name: string;
  opportunity_info?: string;
  source_url?: string;
  source_type: OpportunitySourceType;
  status: OpportunityStatus;
  notes?: string;
}

// ----- 用户与登录 -----

// 登录请求 body：账号 + 密码
// 成功：200 + { ok: true, account: string }
// 失败：400 校验 / 401 账号或密码错 / 404 账号不存在
export interface LoginRequest {
  account: string;
  password: string;
}

// 登录成功响应：ok=true + 返回的 account（用于前端写入 localStorage）
export interface LoginResponse {
  ok: true;
  account: string;
}

// GET /api/users 响应中的单条记录：只含账号（后端不返密码）
export interface UserListItem {
  account: string;
}

// 新建用户请求 body
// 失败：400 校验 / 409 账号已存在
export interface CreateUserRequest {
  account: string;
  password: string;
}

// 修改密码请求 body（PATCH /api/users/:account）
// 前端先校验 newPassword === confirmNewPassword 再发请求
// 失败：400 校验 / 401 旧密码错 / 404 账号不存在
export interface ChangePasswordRequest {
  oldPassword: string;
  newPassword: string;
  confirmNewPassword: string;
}

// ----- 微信绑定 -----

// POST /api/wechat/bind 同步成功响应（已废弃，新协议走异步，见下）
// 保留类型供旧调用方参考；新代码应改用 WechatBindSubmitResult + 轮询。
//   - link: 微信扫码链接（用户也可在桌面端直接打开）
//   - qr: 二维码 Unicode 块字符，多行字符串，前端用 <pre> 等宽字体展示
//   - expired: 是否后端超时/异常（前端据此决定是否提示重试）
//   - raw: 后端 stdout 原文（一般 UI 不展示，仅排错用）
export interface WechatBindResult {
  ok: true;
  link: string;
  qr: string;
  expired: boolean;
  raw?: string;
}

// POST /api/wechat/bind 异步提交响应（202）。
// 后端把原本 ~2 分钟的二维码生成改成异步任务：
//   - POST 立即返 202 + task_id（不再等生成结果）
//   - 前端用 GET /api/wechat/bind/:task_id 轮询拿结果
// 字段说明：
//   - task_id: 后端任务 ID（格式 wt-...），用于后续 GET 轮询
//   - status: 提交时永远为 "pending"（实际状态由 GET 返回）
export interface WechatBindSubmitResult {
  task_id: string;
  status: "pending";
}

// GET /api/wechat/bind/:task_id 响应（200）的 status 字段。
//   - pending/running: 后端还在生成中，前端继续轮询
//   - done: 生成成功，response 里 link/qr 可用，前端展示 modal
//   - failed: 生成失败，response 里 error 字段有后端错误信息
//   - expired: 任务超时/被清理（前端显示「二维码生成超时」）
export type WechatBindTaskStatus =
  | "pending"
  | "running"
  | "done"
  | "failed"
  | "expired";

// GET /api/wechat/bind/:task_id 响应（200）。
// 与 WechatBindResult 字段一致，额外带 task_id / status / error（仅 failed/expired 有）。
// 关键字段：
//   - bound: 后端扫到 openclaw 成功标记（"已将此 OpenClaw 连接到微信。"）时为 true，
//            此时前端切到「绑定成功」状态（replaces QR 区，提示用户重新点绑定）
export interface WechatBindPollResult {
  task_id: string;
  status: WechatBindTaskStatus;
  link?: string;
  qr?: string;
  raw?: string;
  expired?: boolean;
  bound?: boolean;
  error?: string;
  // already_bound:openclaw 检测到该 gateway 早就连过微信,本次是 noop。
  // 前端应展示「该用户已绑定」而不是「绑定成功」,文案更准确。
  // omitempty:新连场景(已成功连接)不返回此字段,前端按 false 处理。
  already_bound?: boolean;
}
