import { describe, it, expect, vi, beforeEach } from "vitest";
import { CRMQuery } from "../../src/query";
import type { Project } from "../../src/query/types";

const P1: Project = {
  id: "PRJ-1",
  created_at: "2026-06-15T10:00:00Z",
  updated_at: "2026-06-16T10:00:00Z",
  project_name: "华为泰国数据中心",
  customer_id: "CUST-1",
  customer_name: "Siam Cement",
  intent_level: "A",
  customer_email: "contact@example.com",
  status: "谈判中",
  assigned_to: "张三",
  notes: "客户对价格敏感",
};
const P2: Project = {
  id: "PRJ-2",
  created_at: "2026-06-15T11:00:00Z",
  updated_at: "2026-06-16T11:00:00Z",
  project_name: "云服务采购",
  customer_id: "CUST-2",
  customer_name: "Bangkok Bank",
  intent_level: "B",
  customer_email: "",
  status: "跟进中",
};

describe("CRMQuery.listProjects", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and returns the projects list", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true, status: 200, json: () => Promise.resolve([P1, P2]),
    }));
    const q = new CRMQuery("/api");
    const list = await q.listProjects();
    expect(list).toHaveLength(2);
    expect(list[0].id).toBe("PRJ-1");
    expect(list[0].customer_name).toBe("Siam Cement");
    expect(list[0].intent_level).toBe("A");
    expect(list[1].customer_email).toBe("");
  });

  it("returns empty array when backend has no projects", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true, status: 200, json: () => Promise.resolve([]),
    }));
    const q = new CRMQuery("/api");
    const list = await q.listProjects();
    expect(list).toEqual([]);
  });

  it("calls /api/projects endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true, status: 200, json: () => Promise.resolve([]),
    });
    vi.stubGlobal("fetch", fetchMock);
    const q = new CRMQuery("/api");
    await q.listProjects();
    const [calledUrl] = fetchMock.mock.calls[0];
    expect(calledUrl).toBe("/api/projects");
  });

  it("propagates HTTP error as thrown", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false, status: 500, statusText: "Internal Server Error",
      json: () => Promise.reject(new Error("not json")),
    }));
    const q = new CRMQuery("/api");
    await expect(q.listProjects()).rejects.toThrow(/HTTP 500/);
  });
});
