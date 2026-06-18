import { describe, it, expect, vi, beforeEach } from "vitest";
import { CRMQuery } from "../../src/query";
import type { Opportunity } from "../../src/query/types";

// 正常商机：完整字段 + 已 join 客户
const O1: Opportunity = {
  id: "OPP-1778151719582-952a61",
  created_at: "2026-06-15T10:00:00Z",
  updated_at: "2026-06-16T10:00:00Z",
  opportunity_name: "泰国正大集团拟新建食品加工厂",
  customer_id: "CUST-1",
  customer_name: "Siam Cement Group",
  opportunity_info: "占地约 200 亩，预计投资 5 亿美元",
  source_url: "https://example.com/news/123",
  source_type: "新闻搜索",
  status: "待评估",
  notes: "与张三跟进重叠",
};

// customer_id 找不到客户 → customer_name 为空字符串
const O2: Opportunity = {
  id: "OPP-1778152000000-deadbe",
  created_at: "2026-06-15T11:00:00Z",
  updated_at: "2026-06-16T12:00:00Z",
  opportunity_name: "招标公告：曼谷工业园厂房租赁",
  customer_id: "CUST-ghost",
  customer_name: "",
  opportunity_info: "建筑面积 50,000 平方米",
  source_url: "https://example.com/bid/456",
  source_type: "招标公告",
  status: "跟进中",
  notes: "",
};

// 已转化商机：所有可选字段均缺失
const O3: Opportunity = {
  id: "OPP-1778152100000-cafeb0",
  created_at: "2026-06-15T12:00:00Z",
  updated_at: "2026-06-16T13:00:00Z",
  opportunity_name: "已落地：越南胡志明工业园扩建",
  customer_id: "CUST-3",
  customer_name: "Bangkok Bank",
  source_type: "企业公告",
  status: "已转化",
};

describe("CRMQuery.listOpportunities", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and returns the opportunities list", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true, status: 200, json: () => Promise.resolve([O1, O2, O3]),
    }));
    const q = new CRMQuery("/api");
    const list = await q.listOpportunities();
    expect(list).toHaveLength(3);
    expect(list[0].id).toBe("OPP-1778151719582-952a61");
    expect(list[0].customer_name).toBe("Siam Cement Group");
    expect(list[0].source_type).toBe("新闻搜索");
    expect(list[0].status).toBe("待评估");
    expect(list[1].customer_name).toBe("");
    expect(list[2].status).toBe("已转化");
  });

  it("returns empty array when backend has no opportunities", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true, status: 200, json: () => Promise.resolve([]),
    }));
    const q = new CRMQuery("/api");
    const list = await q.listOpportunities();
    expect(list).toEqual([]);
  });

  it("calls /api/opportunities endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true, status: 200, json: () => Promise.resolve([]),
    });
    vi.stubGlobal("fetch", fetchMock);
    const q = new CRMQuery("/api");
    await q.listOpportunities();
    const [calledUrl] = fetchMock.mock.calls[0];
    expect(calledUrl).toBe("/api/opportunities");
  });

  it("propagates HTTP error as thrown", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false, status: 500, statusText: "Internal Server Error",
      json: () => Promise.reject(new Error("not json")),
    }));
    const q = new CRMQuery("/api");
    await expect(q.listOpportunities()).rejects.toThrow(/HTTP 500/);
  });
});
