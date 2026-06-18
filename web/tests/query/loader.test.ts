import { describe, it, expect, vi, beforeEach } from "vitest";
import { fetchJSON, CRMFetchError } from "../../src/query/loader";

describe("fetchJSON", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("returns parsed JSON on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ foo: 1 }),
      }),
    );
    const result = await fetchJSON<{ foo: number }>("/api/index");
    expect(result).toEqual({ foo: 1 });
  });

  it("throws CRMFetchError on 404", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({ ok: false, status: 404, statusText: "Not Found" }),
    );
    await expect(fetchJSON("/api/customers/missing")).rejects.toBeInstanceOf(CRMFetchError);
  });

  it("throws CRMFetchError on network error", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network down")));
    await expect(fetchJSON("/api/x")).rejects.toBeInstanceOf(CRMFetchError);
  });

  it("throws CRMFetchError on invalid JSON", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.reject(new SyntaxError("bad json")),
      }),
    );
    await expect(fetchJSON("/api/bad")).rejects.toBeInstanceOf(CRMFetchError);
  });
});
