import { describe, it, expect, vi, beforeEach } from "vitest";
import { login, listUsers, createUser, changePassword } from "../src/query/users";
import { CRMFetchError } from "../src/query/loader";
import type { UserListItem } from "../src/query/types";

const mockFetch = vi.fn();
beforeEach(() => {
  mockFetch.mockReset();
  vi.stubGlobal("fetch", mockFetch);
});

// 解析 fetch 入参,返回 { url, method, body }
function lastCall(): { url: string; method: string; body: unknown } {
  const [url, init] = mockFetch.mock.calls.at(-1) as [string, RequestInit];
  const method = (init?.method ?? "GET") as string;
  const body = init?.body ? JSON.parse(init.body as string) : undefined;
  return { url, method, body };
}

function jsonResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
    statusText: String(status),
  } as unknown as Response;
}

describe("query/users", () => {
  describe("login", () => {
    it("POST /api/login, body 含 account + password;成功时返回 { ok, account }", async () => {
      mockFetch.mockResolvedValue(jsonResponse(200, { ok: true, account: "admin" }));
      const res = await login({ account: "admin", password: "admin123" });
      expect(res).toEqual({ ok: true, account: "admin" });

      const c = lastCall();
      expect(c.url).toBe("/api/login");
      expect(c.method).toBe("POST");
      expect(c.body).toEqual({ account: "admin", password: "admin123" });
    });

    it("401 (账号或密码错) 抛 CRMFetchError 携带后端 error 文案", async () => {
      mockFetch.mockResolvedValue(jsonResponse(401, { ok: false, error: "账号或密码错误" }));
      await expect(login({ account: "admin", password: "wrong" })).rejects.toMatchObject({
        name: "CRMFetchError",
        status: 401,
        message: "账号或密码错误",
      });
    });

    it("404 (账号不存在) 抛 CRMFetchError 携带后端 error 文案", async () => {
      mockFetch.mockResolvedValue(jsonResponse(404, { ok: false, error: "账号不存在" }));
      await expect(login({ account: "nobody", password: "x" })).rejects.toBeInstanceOf(CRMFetchError);
      try {
        await login({ account: "nobody", password: "x" });
      } catch (e) {
        expect((e as CRMFetchError).status).toBe(404);
        expect((e as CRMFetchError).message).toBe("账号不存在");
      }
    });

    it("网络错误(status=0) 也抛 CRMFetchError", async () => {
      mockFetch.mockRejectedValue(new TypeError("Failed to fetch"));
      await expect(login({ account: "a", password: "b" })).rejects.toBeInstanceOf(CRMFetchError);
    });
  });

  describe("listUsers", () => {
    it("GET /api/users, 返回 UserListItem[]", async () => {
      const list: UserListItem[] = [{ account: "admin" }, { account: "alice" }];
      mockFetch.mockResolvedValue(jsonResponse(200, list));
      const res = await listUsers();
      expect(res).toEqual(list);

      const c = lastCall();
      expect(c.url).toBe("/api/users");
      expect(c.method).toBe("GET");
    });

    it("失败抛 CRMFetchError", async () => {
      mockFetch.mockResolvedValue(jsonResponse(500, { error: "boom" }));
      await expect(listUsers()).rejects.toBeInstanceOf(CRMFetchError);
    });
  });

  describe("createUser", () => {
    it("POST /api/users, body 含 account + password;成功返 { ok, account }", async () => {
      mockFetch.mockResolvedValue(jsonResponse(201, { ok: true, account: "alice" }));
      const res = await createUser({ account: "alice", password: "pw123" });
      expect(res).toEqual({ ok: true, account: "alice" });

      const c = lastCall();
      expect(c.url).toBe("/api/users");
      expect(c.method).toBe("POST");
      expect(c.body).toEqual({ account: "alice", password: "pw123" });
    });

    it("409 (账号已存在) 抛 CRMFetchError 携带后端 error", async () => {
      mockFetch.mockResolvedValue(jsonResponse(409, { ok: false, error: "账号已存在" }));
      await expect(createUser({ account: "admin", password: "x" })).rejects.toMatchObject({
        name: "CRMFetchError",
        status: 409,
        message: "账号已存在",
      });
    });
  });

  describe("changePassword", () => {
    it("PATCH /api/users/:account, body 是 { oldPassword, newPassword, confirmNewPassword }", async () => {
      mockFetch.mockResolvedValue(jsonResponse(200, { ok: true }));
      const res = await changePassword("admin", {
        oldPassword: "old",
        newPassword: "new1",
        confirmNewPassword: "new1",
      });
      expect(res).toEqual({ ok: true });

      const c = lastCall();
      expect(c.url).toBe("/api/users/admin");
      expect(c.method).toBe("PATCH");
      expect(c.body).toEqual({
        oldPassword: "old",
        newPassword: "new1",
        confirmNewPassword: "new1",
      });
    });

    it("401 (旧密码错) 抛 CRMFetchError 携带后端 error", async () => {
      mockFetch.mockResolvedValue(jsonResponse(401, { ok: false, error: "旧密码错误" }));
      await expect(
        changePassword("admin", { oldPassword: "wrong", newPassword: "n", confirmNewPassword: "n" }),
      ).rejects.toMatchObject({
        name: "CRMFetchError",
        status: 401,
        message: "旧密码错误",
      });
    });

    it("404 (账号不存在) 抛 CRMFetchError", async () => {
      mockFetch.mockResolvedValue(jsonResponse(404, { ok: false, error: "账号不存在" }));
      await expect(
        changePassword("nobody", { oldPassword: "x", newPassword: "y", confirmNewPassword: "y" }),
      ).rejects.toBeInstanceOf(CRMFetchError);
    });
  });
});
