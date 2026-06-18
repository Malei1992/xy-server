import { describe, it, expect, vi, beforeEach } from "vitest";
import { CRMQuery } from "../../src/query";
import type { Task } from "../../src/query/types";

// 正常任务：完整字段 + 已 join 客户
const T1: Task = {
  id: "TASK-1778151719582-952a61",
  created_at: "2026-06-15T10:00:00Z",
  updated_at: "2026-06-16T10:00:00Z",
  source: "ProspectorAgent",
  type: "compliance_blocked",
  priority: "P1",
  status: "pending",
  title: "合规文件缺失：泰国 BOI",
  description: "客户缺少投资促进委员会证明，请尽快补充",
  customer_id: "CUST-1",
  customer_name: "Siam Cement Group",
  email_id: "MAIL-1",
  assigned_to: "张三",
  resolution: "已补充 BOI 证书",
  resolved_at: "2026-06-16T11:00:00Z",
};

// 已 resolved 任务：resolved_at + resolution 应保留
const T2: Task = {
  id: "TASK-1778152000000-deadbe",
  created_at: "2026-06-15T11:00:00Z",
  updated_at: "2026-06-16T12:00:00Z",
  type: "anomaly_alert",
  priority: "P0",
  status: "resolved",
  title: "异常告警：客户重复注册",
  customer_id: "CUST-2",
  customer_name: "Bangkok Bank",
};

// customer_id 找不到客户 → customer_name 为空字符串
const T3: Task = {
  id: "TASK-1778152100000-cafeb0",
  created_at: "2026-06-15T12:00:00Z",
  updated_at: "2026-06-16T13:00:00Z",
  type: "llm_failure",
  priority: "P2",
  status: "in_progress",
  title: "LLM 调用失败",
  customer_id: "CUST-ghost",
  customer_name: "",
};

describe("CRMQuery.listTasks", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and returns the tasks list", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true, status: 200, json: () => Promise.resolve([T1, T2, T3]),
    }));
    const q = new CRMQuery("/api");
    const list = await q.listTasks();
    expect(list).toHaveLength(3);
    expect(list[0].id).toBe("TASK-1778151719582-952a61");
    expect(list[0].priority).toBe("P1");
    expect(list[0].customer_name).toBe("Siam Cement Group");
    expect(list[1].status).toBe("resolved");
    expect(list[2].customer_name).toBe("");
  });

  it("returns empty array when backend has no tasks", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true, status: 200, json: () => Promise.resolve([]),
    }));
    const q = new CRMQuery("/api");
    const list = await q.listTasks();
    expect(list).toEqual([]);
  });

  it("calls /api/tasks endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true, status: 200, json: () => Promise.resolve([]),
    });
    vi.stubGlobal("fetch", fetchMock);
    const q = new CRMQuery("/api");
    await q.listTasks();
    const [calledUrl] = fetchMock.mock.calls[0];
    expect(calledUrl).toBe("/api/tasks");
  });

  it("propagates HTTP error as thrown", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false, status: 500, statusText: "Internal Server Error",
      json: () => Promise.reject(new Error("not json")),
    }));
    const q = new CRMQuery("/api");
    await expect(q.listTasks()).rejects.toThrow(/HTTP 500/);
  });
});
