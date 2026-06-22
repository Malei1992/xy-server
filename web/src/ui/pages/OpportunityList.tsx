import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { CRMQuery } from "@/query";
import type { Opportunity, OpportunityStatus } from "@/query/types";
import { OPPORTUNITY_STATUS_OPTIONS } from "@/query/types";
import { OpportunityTable } from "../components/OpportunityTable";

const q = new CRMQuery();

export function OpportunityList() {
  const [opps, setOpps] = useState<Opportunity[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const navigate = useNavigate();

  useEffect(() => {
    let cancelled = false;
    q.listOpportunities()
      .then((list) => {
        if (cancelled) return;
        setOpps(list);
        setError(null);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
      })
      .finally(() => {
        if (cancelled) return;
        setLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // 客户端按 名称 / 客户名称 / 来源类型 / 状态 模糊搜索
  const filtered = useMemo(() => {
    const k = search.trim().toLowerCase();
    if (!k) return opps;
    return opps.filter((o) => {
      return (
        o.opportunity_name.toLowerCase().includes(k) ||
        o.customer_name.toLowerCase().includes(k) ||
        o.source_type.toLowerCase().includes(k) ||
        o.status.toLowerCase().includes(k)
      );
    });
  }, [opps, search]);

  // 内联状态修改:Table 通过 onStatusChange 调 PATCH,成功才更新本地 list
  // 失败时 InlineStatusSelect 内部已展示错误,这里把 error throw 回去让 select 知道
  const handleStatusChange = async (id: string, newStatus: OpportunityStatus) => {
    await q.updateOpportunityStatus(id, newStatus);
    setOpps((prev) => prev.map((o) => (o.id === id ? { ...o, status: newStatus } : o)));
  };

  return (
    <div style={{ padding: 24 }}>
      <div style={{
        display: "flex", alignItems: "center",
        justifyContent: "space-between", marginBottom: 16, gap: 12,
      }}>
        <h2 style={{ fontSize: 18 }}>公开信息</h2>
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="搜索 名称 / 客户 / 来源类型 / 状态"
          aria-label="搜索公开信息"
          data-testid="search-input"
          style={{
            width: 280,
            padding: "6px 10px",
            border: "1px solid var(--border)",
            borderRadius: 4,
            fontSize: 14,
            outline: "none",
          }}
        />
      </div>

      <div style={{
        background: "white", border: "1px solid var(--border)", borderRadius: 8,
      }}>
        {!loaded
          ? <div style={{ padding: 24, color: "var(--text-muted)" }}>加载中...</div>
          : error
            ? (
              <div style={{ padding: 24, color: "var(--danger, #dc2626)" }}>
                加载失败：{error}
              </div>
            )
            : (
              <OpportunityTable
                opportunities={filtered}
                onCustomerClick={(customerId) => navigate(`/customers/${encodeURIComponent(customerId)}`)}
                statusOptions={OPPORTUNITY_STATUS_OPTIONS}
                onStatusChange={handleStatusChange}
              />
            )}
      </div>
    </div>
  );
}