import { describe, it, expect, vi, beforeEach } from "vitest";
import { CRMQuery } from "../../src/query";
import type { Customer } from "../../src/query/types";

const C1: Customer = {
  id: "C1", created_at: "2026-06-01T00:00:00Z", updated_at: "2026-06-01T00:00:00Z",
  basic: { name: "A公司", country: "泰国", industry: "X", contacts: "a@x.com", phones: "+66" },
  engagement: { status: "active", intent_level: "A" },
  prospecting: { grade: "A", overall_risk: "低" },
  timeline: [],
};
const C2: Customer = {
  id: "C2", created_at: "2026-06-01T00:00:00Z", updated_at: "2026-06-01T00:00:00Z",
  basic: { name: "B公司", country: "越南", industry: "Y", contacts: "b@x.com", phones: "+84" },
  engagement: { status: "dormant", intent_level: "C" },
  prospecting: { grade: "B", overall_risk: "中" },
  timeline: [],
};
const C3: Customer = {
  id: "C3", created_at: "2026-06-01T00:00:00Z", updated_at: "2026-06-01T00:00:00Z",
  basic: { name: "C公司", country: "泰国", industry: "Z", contacts: "c@x.com", phones: "+66" },
  engagement: { status: "active", intent_level: "unknown" },
  prospecting: { grade: "C", overall_risk: "高" },
  timeline: [],
};

function mockFetchAll() {
  return vi.fn().mockImplementation(async (url: string) => {
    // 后端聚合端点：一次拉全量
    if (url === "/api/customers" || url.endsWith("/api/customers")) {
      return {
        ok: true, status: 200,
        json: () => Promise.resolve([C1, C2, C3]),
      };
    }
    throw new Error(`unexpected URL in test: ${url}`);
  });
}

describe("CRMQuery.getCustomer", () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it("loads customer by id", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true, status: 200, json: () => Promise.resolve(C1),
    }));
    const q = new CRMQuery("/api");
    const c = await q.getCustomer("C1");
    expect(c.id).toBe("C1");
    expect(c.basic.name).toBe("A公司");
  });
});

describe("CRMQuery.listCustomers", () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it("returns all customers when no filter", async () => {
    vi.stubGlobal("fetch", mockFetchAll());
    const q = new CRMQuery("/api");
    const list = await q.listCustomers();
    expect(list).toHaveLength(3);
  });

  it("filters by status", async () => {
    vi.stubGlobal("fetch", mockFetchAll());
    const q = new CRMQuery("/api");
    const list = await q.listCustomers({ status: ["active"] });
    expect(list.map(c => c.id).sort()).toEqual(["C1", "C3"]);
  });

  it("filters by country", async () => {
    vi.stubGlobal("fetch", mockFetchAll());
    const q = new CRMQuery("/api");
    const list = await q.listCustomers({ country: ["泰国"] });
    expect(list.map(c => c.id).sort()).toEqual(["C1", "C3"]);
  });

  it("filters by intentLevel excluding unknown/empty", async () => {
    vi.stubGlobal("fetch", mockFetchAll());
    const q = new CRMQuery("/api");
    const list = await q.listCustomers({ intentLevel: ["A", "C"] });
    expect(list.map(c => c.id).sort()).toEqual(["C1", "C2"]);
  });

  it("filters by grade", async () => {
    vi.stubGlobal("fetch", mockFetchAll());
    const q = new CRMQuery("/api");
    const list = await q.listCustomers({ grade: ["A", "B"] });
    expect(list.map(c => c.id).sort()).toEqual(["C1", "C2"]);
  });
});
