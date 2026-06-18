import type { TimelineEvent } from "../../query/types";

// 时间线事件流
// - 倒序展示（由调用方传入已排序的事件）
// - 每行展示事件名/时间/操作人
export function Timeline({ events }: { events: TimelineEvent[] }) {
  if (events.length === 0) {
    return <div style={{ color: "#6b7280", fontSize: 14 }}>暂无时间线事件</div>;
  }
  return (
    <ol style={{ listStyle: "none", padding: 0 }}>
      {events.map((e, i) => (
        <li key={i} style={{
          padding: "8px 0", borderBottom: "1px solid #f3f4f6",
        }}>
          <div style={{ display: "flex", gap: 8, fontSize: 14 }}>
            <span style={{ fontWeight: 600, minWidth: 140 }}>{e.event}</span>
            <span style={{ color: "#6b7280" }}>
              {new Date(e.at).toLocaleString("zh-CN")}
            </span>
            <span style={{ color: "#9ca3af" }}>by {e.by}</span>
          </div>
          {e.detail && (
            <div style={{ fontSize: 13, color: "#374151", marginTop: 4 }}>
              {e.detail}
            </div>
          )}
        </li>
      ))}
    </ol>
  );
}
