import type { TaskPriority } from "../../query/types";

// 任务等级圆形徽章。
// P0=红(严重告警) / P1=黄(高优) / P2=绿(普通) / P3=灰(低优)。
// 圆形（border-radius: 50%）+ 等宽 + 居中字母：宽度 22px、高度 22px。
// 未知等级 → 灰底 "?"，避免布局跳变。
const COLORS: Record<TaskPriority, { bg: string; color: string }> = {
  P0: { bg: "#dc2626", color: "#ffffff" },
  P1: { bg: "#facc15", color: "#1f2937" },
  P2: { bg: "#16a34a", color: "#ffffff" },
  P3: { bg: "#9ca3af", color: "#ffffff" },
};

export function PriorityBadge({ priority }: { priority: TaskPriority }) {
  const c = COLORS[priority] ?? { bg: "#e5e7eb", color: "#6b7280" };
  const label = priority in COLORS ? priority : "?";
  return (
    <span
      title={`任务等级 ${priority}`}
      aria-label={`任务等级 ${priority}`}
      data-testid={`priority-${priority}`}
      style={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        width: 22,
        height: 22,
        borderRadius: "50%",
        fontSize: 11,
        fontWeight: 700,
        background: c.bg,
        color: c.color,
        lineHeight: 1,
      }}
    >
      {label}
    </span>
  );
}
