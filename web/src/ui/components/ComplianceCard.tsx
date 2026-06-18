import type { ComplianceRisk, Risk } from "@/query/types";
import { RiskBadge } from "./RiskBadge";

// 合规分析卡片：5 维度 + 整体风险
// 实际数据中存在两套字段命名（rating/detail 与 level/note），
// 这里按"哪个有就读哪个"处理，保证新老数据都能渲染
const DIMS: { key: keyof ComplianceRisk; label: string }[] = [
  { key: "foreign_investment", label: "外资准入" },
  { key: "tax", label: "税务政策" },
  { key: "labor", label: "劳工法规" },
  { key: "forex", label: "外汇管制" },
  { key: "industry_barrier", label: "行业壁垒" },
];

export function ComplianceCard({
  risk, overall,
}: { risk?: ComplianceRisk; overall?: Risk }) {
  if (!risk) return null;
  return (
    <div style={{
      background: "white", border: "1px solid #e5e7eb",
      borderRadius: 8, padding: 16,
    }}>
      <div style={{
        display: "flex", justifyContent: "space-between",
        alignItems: "center", marginBottom: 12,
      }}>
        <h3 style={{ fontSize: 16 }}>合规分析</h3>
        {overall && <RiskBadge risk={overall} />}
      </div>
      <div style={{ display: "grid", gap: 8 }}>
        {DIMS.map(({ key, label }) => {
          const d = risk[key];
          if (!d) return null;
          return (
            <div
              key={key}
              style={{
                display: "grid", gridTemplateColumns: "100px 80px 1fr",
                gap: 8, fontSize: 14,
              }}
            >
              <span style={{ color: "#6b7280" }}>{label}</span>
              <span style={{ fontWeight: 600 }}>{d.rating || d.level || "—"}</span>
              <span style={{ color: "#374151" }}>{d.detail || d.note || "—"}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
