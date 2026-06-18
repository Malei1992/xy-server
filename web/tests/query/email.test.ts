import { describe, it, expect, vi, beforeEach } from "vitest";
import { CRMQuery } from "../../src/query";
import type { Email } from "../../src/query/types";

const E1: Email = {
  id: "E1", direction: "sent", customer_id: "C1", type: "invitation",
  subject: "邀请", language: "th", status: "sent", created_at: "2026-06-01T00:00:00Z",
};
const E2: Email = {
  id: "E2", direction: "received", customer_id: "C1", type: "incoming",
  subject: "回信", language: "en", status: "processed", created_at: "2026-06-02T00:00:00Z",
};
const E3: Email = {
  id: "E3", direction: "sent", customer_id: "C2", type: "nurture",
  subject: "培育", language: "th", status: "approved", created_at: "2026-06-03T00:00:00Z",
};

function mockEmailFetch() {
  return vi.fn().mockImplementation(async (url: string) => {
    if (url.endsWith("/api/index")) {
      return {
        ok: true, status: 200,
        json: () => Promise.resolve({
          customers: [], emails: ["E1", "E2", "E3"],
          by_status: {}, by_intent: {}, by_country: {},
        }),
      };
    }
    // URL 形式：/api/emails/<id>  （无 .json 后缀）
    const m = url.match(/\/api\/emails\/([^/?#]+)$/);
    const id = m ? decodeURIComponent(m[1]) : "";
    const map: Record<string, Email> = { E1, E2, E3 };
    return { ok: true, status: 200, json: () => Promise.resolve(map[id]) };
  });
}

describe("CRMQuery emails", () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it("getEmail loads by id", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true, status: 200, json: () => Promise.resolve(E1),
    }));
    const q = new CRMQuery("/api");
    const e = await q.getEmail("E1");
    expect(e.subject).toBe("邀请");
  });

  it("listEmails returns all when no filter", async () => {
    vi.stubGlobal("fetch", mockEmailFetch());
    const q = new CRMQuery("/api");
    const list = await q.listEmails();
    expect(list).toHaveLength(3);
  });

  it("listEmails filters by customerId", async () => {
    vi.stubGlobal("fetch", mockEmailFetch());
    const q = new CRMQuery("/api");
    const list = await q.listEmails({ customerId: "C1" });
    expect(list.map(e => e.id).sort()).toEqual(["E1", "E2"]);
  });

  it("listEmails filters by direction and status", async () => {
    vi.stubGlobal("fetch", mockEmailFetch());
    const q = new CRMQuery("/api");
    const list = await q.listEmails({ direction: ["sent"], status: ["sent"] });
    expect(list.map(e => e.id)).toEqual(["E1"]);
  });
});
