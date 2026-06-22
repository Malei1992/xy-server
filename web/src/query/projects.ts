// 商机信息（projects）的独立 query 封装
// - listProjects / updateProjectStatus 都是 CRMQuery 类的薄包装
// - 业务代码可以选择「直接用 CRMQuery」或「从该文件 import 函数式 API」
// - 跟 users.ts / tasks.ts / opportunities.ts 同结构（spec 2026-06-22 状态修改设计）

import { CRMQuery } from "./index";
import type { Project, ProjectStatus } from "./types";

const q = new CRMQuery();

// GET /api/projects → Project[]
// 后端从 crm/projects/*.json 读 + join 客户展示字段。
// 项目目录不存在 / 空 → 200 + []。
export function listProjects(): Promise<Project[]> {
  return q.listProjects();
}

// PATCH /api/projects/:id/status，body { status: <new> }
// 成功：{ ok: true, status: <new> }
// 失败：抛 CRMFetchError（400 status 非法 / 404 记录不存在 / 500 后端 I/O）
export function updateProjectStatus(
  id: string,
  status: ProjectStatus,
): Promise<{ ok: true; status: ProjectStatus }> {
  return q.updateProjectStatus(id, status);
}
