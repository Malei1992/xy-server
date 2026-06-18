// 用户与登录相关 API 封装
// 沿用 fetchJSON / CRMFetchError 模式:
//   - 失败统一抛 CRMFetchError(message 来自 body.error)
//   - 调用方可按 e.status 区分 401(密码错) / 404(账号不存在) / 409(账号已存在)

import { API_BASE_URL } from "@/config";
import { CRMFetchError } from "./loader";
import type {
  UserListItem,
  LoginRequest,
  LoginResponse,
  CreateUserRequest,
  ChangePasswordRequest,
} from "./types";

// 通用 fetch 辅助:走 /api/*,自动 JSON,失败抛 CRMFetchError(message 取 body.error)
async function postJSON<T>(path: string, body: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE_URL}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  } catch (err) {
    throw new CRMFetchError(path, 0, `网络错误: ${(err as Error).message}`);
  }
  if (!res.ok) {
    const errBody = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new CRMFetchError(path, res.status, errBody?.error ?? `HTTP ${res.status}`);
  }
  return (await res.json()) as T;
}

async function patchJSON<T>(path: string, body: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE_URL}${path}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  } catch (err) {
    throw new CRMFetchError(path, 0, `网络错误: ${(err as Error).message}`);
  }
  if (!res.ok) {
    const errBody = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new CRMFetchError(path, res.status, errBody?.error ?? `HTTP ${res.status}`);
  }
  return (await res.json()) as T;
}

async function getJSON<T>(path: string): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE_URL}${path}`);
  } catch (err) {
    throw new CRMFetchError(path, 0, `网络错误: ${(err as Error).message}`);
  }
  if (!res.ok) {
    const errBody = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new CRMFetchError(path, res.status, errBody?.error ?? `HTTP ${res.status}`);
  }
  return (await res.json()) as T;
}

// POST /api/login
// 200 → { ok: true, account }
// 401 → 账号或密码错(后端 body.error = "账号或密码错误")
// 404 → 账号不存在(后端 body.error = "账号不存在")
// 400 → 校验失败
export function login(req: LoginRequest): Promise<LoginResponse> {
  return postJSON<LoginResponse>("/login", req);
}

// GET /api/users → UserListItem[] (只含 account,不含密码)
export function listUsers(): Promise<UserListItem[]> {
  return getJSON<UserListItem[]>("/users");
}

// POST /api/users
// 201 → { ok: true, account }
// 409 → 账号已存在
// 400 → 校验失败
export function createUser(req: CreateUserRequest): Promise<{ ok: true; account: string }> {
  return postJSON<{ ok: true; account: string }>("/users", req);
}

// PATCH /api/users/:account
// 200 → { ok: true }
// 400 → 校验失败 / 两次新密码不一致
// 401 → 旧密码错
// 404 → 账号不存在
export function changePassword(
  account: string,
  req: ChangePasswordRequest,
): Promise<{ ok: true }> {
  return patchJSON<{ ok: true }>(`/users/${encodeURIComponent(account)}`, req);
}
