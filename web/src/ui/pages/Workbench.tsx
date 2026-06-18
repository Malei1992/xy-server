import { useEffect, useState } from "react";
import { CRMQuery } from "@/query";
import type { Customer, IndexSummary, TimelineEvent } from "@/query/types";

const q = new CRMQuery();

// 合并时间线事件并附带客户名
type RecentEvent = TimelineEvent & { customerName: string };

export function Workbench() {
  const [summary, setSummary] = useState<IndexSummary | null>(null);
  const [customers, setCustomers] = useState<Customer[]>([]);

  useEffect(() => {
    q.getIndex().then(setSummary);
    q.listCustomers().then(setCustomers);
  }, []);

  if (!summary) return <div style={{ padding: 24 }}>加载中...</div>;

  const total = summary.customers.length;
  const active = summary.by_status.active?.length ?? 0;
  // 未知意向 = intentLevel 为 "unknown" 或 "" 的客户数
  const unknownIntent = (summary.by_intent["unknown"]?.length ?? 0)
    + (summary.by_intent[""]?.length ?? 0);
  const needsReview = summary.by_status.needs_manual_review?.length ?? 0;

  // 最近 5 条 timeline 事件（跨客户汇总，倒序）
  const recentTimeline: RecentEvent[] = customers
    .flatMap((c) => c.timeline.map((e) => ({ ...e, customerName: c.basic.name })))
    .sort((a, b) => (a.at < b.at ? 1 : -1))
    .slice(0, 5);

  return (
    <div style={{ padding: 24, display: "grid", gap: 16 }}>
      <h2>代办</h2>

      {/* 顶部 4 张统计卡片 */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 12 }}>
        <Card label="客户总数" value={total} />
        <Card label="活跃客户" value={active} />
        <Card label="未知意向" value={unknownIntent} />
        <Card label="待人工复核" value={needsReview} />
      </div>

      {/* 状态分布进度条 */}
      <div style={{
        background: "white", border: "1px solid #e5e7eb",
        borderRadius: 8, padding: 16,
      }}>
        <h3 style={{ fontSize: 16, marginBottom: 12 }}>状态分布</h3>
        {Object.entries(summary.by_status).map(([k, v]) => (
          <div key={k} style={{
            display: "flex", alignItems: "center", gap: 8, marginBottom: 4,
          }}>
            <span style={{ width: 140 }}>{k}</span>
            <div style={{
              flex: 1, height: 8, background: "#f3f4f6", borderRadius: 4,
            }}>
              <div style={{
                width: `${(v.length / total) * 100}%`, height: 8,
                background: "#2563eb", borderRadius: 4,
              }} />
            </div>
            <span>{v.length}</span>
          </div>
        ))}
      </div>

      {/* 最近活动 */}
      <div style={{
        background: "white", border: "1px solid #e5e7eb",
        borderRadius: 8, padding: 16,
      }}>
        <h3 style={{ fontSize: 16, marginBottom: 12 }}>最近活动</h3>
        {recentTimeline.length === 0
          ? <div style={{ color: "#6b7280" }}>暂无活动</div>
          : (
            <ol style={{ listStyle: "none", padding: 0 }}>
              {recentTimeline.map((e, i) => (
                <li key={i} style={{ padding: "6px 0", fontSize: 14 }}>
                  <strong>{e.customerName}</strong> · {e.event} · {new Date(e.at).toLocaleString("zh-CN")}
                </li>
              ))}
            </ol>
          )}
      </div>
    </div>
  );
}

function Card({ label, value }: { label: string; value: number }) {
  return (
    <div style={{
      background: "white", border: "1px solid #e5e7eb",
      borderRadius: 8, padding: 16,
    }}>
      <div style={{ color: "#6b7280", fontSize: 13 }}>{label}</div>
      <div style={{ fontSize: 28, fontWeight: 600, marginTop: 4 }}>{value}</div>
    </div>
  );
}
