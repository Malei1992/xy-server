import { describe, it, expect, vi, beforeEach } from "vitest";
import { CRMQuery } from "../../src/query";

describe("CRMQuery.getIndex", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and returns index.json", async () => {
    const idx = {
      customers: ["CUST-A", "CUST-B"],
      by_status: { active: ["CUST-A", "CUST-B"] },
      by_intent: { unknown: ["CUST-A"], "": ["CUST-B"] },
      by_country: { 泰国: ["CUST-A", "CUST-B"] },
    };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true, status: 200,
      json: () => Promise.resolve(idx),
    }));
    const q = new CRMQuery("/api");
    const result = await q.getIndex();
    expect(result.customers).toEqual(["CUST-A", "CUST-B"]);
    expect(result.by_country["泰国"]).toEqual(["CUST-A", "CUST-B"]);
    expect(result.by_intent["unknown"]).toEqual(["CUST-A"]);
  });
});
