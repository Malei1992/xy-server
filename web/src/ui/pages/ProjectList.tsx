import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { CRMQuery } from "@/query";
import type { Project } from "@/query/types";
import { ProjectTable } from "../components/ProjectTable";

const q = new CRMQuery();

export function ProjectList() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const navigate = useNavigate();

  useEffect(() => {
    let cancelled = false;
    q.listProjects()
      .then((list) => {
        if (cancelled) return;
        setProjects(list);
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

  // 客户端按项目名称 / 客户名称 / 跟进状态 / 负责人 模糊搜索
  const filtered = useMemo(() => {
    const k = search.trim().toLowerCase();
    if (!k) return projects;
    return projects.filter((p) => {
      return (
        p.project_name.toLowerCase().includes(k) ||
        p.customer_name.toLowerCase().includes(k) ||
        p.status.toLowerCase().includes(k) ||
        (p.assigned_to ?? "").toLowerCase().includes(k)
      );
    });
  }, [projects, search]);

  return (
    <div style={{ padding: 24 }}>
      <div style={{
        display: "flex", alignItems: "center",
        justifyContent: "space-between", marginBottom: 16, gap: 12,
      }}>
        <h2 style={{ fontSize: 18 }}>商机信息</h2>
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="搜索 项目 / 客户 / 状态 / 负责人"
          aria-label="搜索商机"
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
              <ProjectTable
                projects={filtered}
                onCustomerClick={(customerId) => navigate(`/customers/${encodeURIComponent(customerId)}`)}
              />
            )}
      </div>
    </div>
  );
}
