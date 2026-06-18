import type { Risk } from "../../query/types";

// 风险等级徽章：低/中/高/禁止/待定
const COLORS: Record<Risk, { bg: string; color: string }> = {
  低: { bg: "#d1fae5", color: "#065f46" },
  中: { bg: "#fef3c7", color: "#92400e" },
  高: { bg: "#fee2e2", color: "#991b1b" },
  禁止: { bg: "#7f1d1d", color: "white" },
  待定: { bg: "#e5e7eb", color: "#374151" },
};

export function RiskBadge({ risk }: { risk: Risk }) {
  const c = COLORS[risk];
  return (
    <span style={{
      display: "inline-block", padding: "2px 8px", borderRadius: 4,
      fontSize: 12, background: c.bg, color: c.color,
    }}>{risk}</span>
  );
}
