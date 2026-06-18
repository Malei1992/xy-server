import type { Grade } from "../../query/types";

// 客户等级色块：A=绿（高潜力）/B=蓝/C=灰
const COLORS: Record<Grade, { bg: string; color: string }> = {
  A: { bg: "#d1fae5", color: "#065f46" },
  B: { bg: "#dbeafe", color: "#1e40af" },
  C: { bg: "#f3f4f6", color: "#374151" },
};

export function GradeChip({ grade }: { grade: Grade }) {
  const c = COLORS[grade];
  return (
    <span style={{
      display: "inline-block", minWidth: 24, textAlign: "center",
      padding: "2px 6px", borderRadius: 4, fontSize: 12, fontWeight: 600,
      background: c.bg, color: c.color,
    }}>{grade}</span>
  );
}
