import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { CRMQuery } from "@/query";
import type { Customer } from "@/query/types";
import { CustomerTable } from "../components/CustomerTable";

const q = new CRMQuery();

export function CustomerList() {
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const navigate = useNavigate();

  useEffect(() => {
    let cancelled = false;
    // 防御：listCustomers 内部已用 allSettled 跳过 404，
    // 这里仍加 .catch，避免 index.json 整体拉取失败时页面卡在"加载中..."
    q.listCustomers()
      .then((list) => {
        if (cancelled) return;
        setCustomers(list);
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

  // 客户端按客户名称模糊搜索：大小写不敏感的子串匹配
  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return customers;
    return customers.filter((c) => c.basic.name.toLowerCase().includes(q));
  }, [customers, search]);

  return (
    <div style={{ padding: 24 }}>
      <div style={{
        display: "flex", alignItems: "center",
        justifyContent: "space-between", marginBottom: 16, gap: 12,
      }}>
        <h2 style={{ fontSize: 18 }}>客户信息</h2>
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="搜索客户名称"
          aria-label="搜索客户名称"
          data-testid="search-input"
          style={{
            width: 240,
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
            : <CustomerTable customers={filtered} onDetail={(id) => navigate(`/customers/${id}`)} />}
      </div>
    </div>
  );
}
