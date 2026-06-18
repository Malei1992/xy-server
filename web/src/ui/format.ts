// 显示格式化工具

// 联系人、电话等多形态字段的统一展示
// schema.md 标为 string[]，实际数据中常为 string 或 ""（空字符串为缺省值）
// 空值（undefined / "" / []）统一显示为 "—"，字符串原样返回，数组用 "，" 拼接
export function formatList(v: string | string[] | undefined): string {
  if (v === undefined || v === "") return "—";
  if (Array.isArray(v)) return v.length === 0 ? "—" : v.join("，");
  return v;
}

// 通用字段展示：缺失值（undefined / null / "" / []）显示 "无"
// 数组用 "，" 拼接，对象/日期原样（由调用方决定如何渲染）
// 不在 formatList 的客户列表中用，避免 "无" 与 "—" 语义混淆
export function formatValue(v: unknown): string {
  if (v === undefined || v === null || v === "") return "无";
  if (Array.isArray(v)) return v.length === 0 ? "无" : v.join("，");
  if (typeof v === "object") return "无"; // 复杂对象不在此处渲染
  return String(v);
}

// ISO8601 时间戳格式化为本地时间；缺失返回 "无"
export function formatDateTime(v: string | undefined | null): string {
  if (!v) return "无";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return "无";
  return d.toLocaleString("zh-CN", { hour12: false });
}

// ===== 代办任务 =====

// 任务类型中文化。9 个枚举对应 schema.md 中 task.type。
// 未知 / 空值 → "无"（与其它"缺失"语义一致）。
// 来源 raw 字段在 cell title 里展示，方便开发者对照 enum。
const TASK_TYPE_LABELS: Record<string, string> = {
  data_insufficient: "数据不足",
  compliance_blocked: "合规阻断",
  llm_failure: "LLM 失败",
  human_notify_failed: "人工通知失败",
  review_timeout: "审查超时",
  complex_inquiry: "复杂咨询",
  anomaly_alert: "异常告警",
  low_confidence: "低置信度",
  send_failed: "发送失败",
};

export function formatTaskType(v: string | undefined | null): string {
  if (!v) return "无";
  return TASK_TYPE_LABELS[v] ?? v;
}

// 任务状态中文化。4 个枚举对应 schema.md 中 task.status。
const TASK_STATUS_LABELS: Record<string, string> = {
  pending: "待处理",
  in_progress: "处理中",
  resolved: "已解决",
  dismissed: "已驳回",
};

export function formatTaskStatus(v: string | undefined | null): string {
  if (!v) return "无";
  return TASK_STATUS_LABELS[v] ?? v;
}
