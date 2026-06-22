import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { CRMQuery } from "@/query";
import type { Task, TaskStatus, TaskPriority } from "@/query/types";
import { TASK_STATUS_OPTIONS } from "@/query/types";
import { TaskTable } from "../components/TaskTable";

const q = new CRMQuery();

const STATUS_OPTIONS: { value: "" | TaskStatus; label: string }[] = [
  { value: "", label: "全部状态" },
  { value: "pending", label: "待处理" },
  { value: "in_progress", label: "处理中" },
  { value: "resolved", label: "已解决" },
  { value: "dismissed", label: "已驳回" },
];

const PRIORITY_OPTIONS: { value: "" | TaskPriority; label: string }[] = [
  { value: "", label: "全部等级" },
  { value: "P0", label: "P0" },
  { value: "P1", label: "P1" },
  { value: "P2", label: "P2" },
  { value: "P3", label: "P3" },
];

export function TaskList() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<"" | TaskStatus>("");
  const [priorityFilter, setPriorityFilter] = useState<"" | TaskPriority>("");
  const navigate = useNavigate();

  useEffect(() => {
    let cancelled = false;
    q.listTasks()
      .then((list) => {
        if (cancelled) return;
        setTasks(list);
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

  // 客户端按 搜索词 + 状态 + 等级 筛选
  const filtered = useMemo(() => {
    const k = search.trim().toLowerCase();
    return tasks.filter((t) => {
      if (k &&
        !t.title.toLowerCase().includes(k) &&
        !t.customer_name.toLowerCase().includes(k) &&
        !(t.assigned_to ?? "").toLowerCase().includes(k) &&
        !t.type.toLowerCase().includes(k)
      ) {
        return false;
      }
      if (statusFilter && t.status !== statusFilter) return false;
      if (priorityFilter && t.priority !== priorityFilter) return false;
      return true;
    });
  }, [tasks, search, statusFilter, priorityFilter]);

  // 内联状态修改:Table 通过 onStatusChange 调 PATCH,成功才更新本地 list
  // 失败时 InlineStatusSelect 内部已展示错误,这里把 error throw 回去让 select 知道
  // 发 API 用英文 enum(API 传输),UI 显示中文 label(由 InlineStatusSelect + TASK_STATUS_OPTIONS 处理)
  const handleStatusChange = async (id: string, newStatus: TaskStatus) => {
    await q.updateTaskStatus(id, newStatus);
    setTasks((prev) => prev.map((t) => (t.id === id ? { ...t, status: newStatus } : t)));
  };

  const selectStyle: React.CSSProperties = {
    padding: "6px 10px",
    border: "1px solid var(--border)",
    borderRadius: 4,
    fontSize: 14,
    outline: "none",
    background: "white",
  };

  return (
    <div style={{ padding: 24 }}>
      <div style={{
        display: "flex", alignItems: "center",
        justifyContent: "space-between", marginBottom: 16, gap: 12,
      }}>
        <h2 style={{ fontSize: 18 }}>代办任务</h2>
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as "" | TaskStatus)}
            data-testid="filter-status"
            style={selectStyle}
          >
            {STATUS_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
          <select
            value={priorityFilter}
            onChange={(e) => setPriorityFilter(e.target.value as "" | TaskPriority)}
            data-testid="filter-priority"
            style={selectStyle}
          >
            {PRIORITY_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
          <input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索 任务 / 客户 / 负责人 / 类型"
            aria-label="搜索代办任务"
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
              <TaskTable
                tasks={filtered}
                onCustomerClick={(customerId) => navigate(`/customers/${encodeURIComponent(customerId)}`)}
                statusOptions={TASK_STATUS_OPTIONS}
                onStatusChange={handleStatusChange}
              />
            )}
      </div>
    </div>
  );
}