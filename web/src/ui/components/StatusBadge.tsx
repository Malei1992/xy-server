import type { Status } from "../../query/types";

// 客户状态徽章：颜色 + 文本标签
const LABELS: Record<Status, { text: string; color: string; bg: string }> = {
  active: { text: "活跃", color: "#065f46", bg: "#d1fae5" },
  dormant: { text: "休眠", color: "#92400e", bg: "#fef3c7" },
  closed: { text: "关闭", color: "#374151", bg: "#e5e7eb" },
  invalid: { text: "无效", color: "#991b1b", bg: "#fee2e2" },
  needs_manual_review: { text: "待人工", color: "#9a3412", bg: "#ffedd5" },
};

export function StatusBadge({ status }: { status: Status }) {
  const s = LABELS[status] ?? LABELS.invalid;
  return (
    <span style={{
      display: "inline-block", padding: "2px 8px", borderRadius: 12,
      fontSize: 12, color: s.color, background: s.bg,
    }}>{s.text}</span>
  );
}
