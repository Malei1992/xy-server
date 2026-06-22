// 代办任务（tasks）的独立 query 封装
// - listTasks / updateTaskStatus 都是 CRMQuery 类的薄包装
// - Task 状态是英文 enum（pending / in_progress / resolved / dismissed），
//   UI 层用中文 label 展示（见 format.ts / STATUS_LABELS）

import { CRMQuery } from "./index";
import type { Task, TaskStatus } from "./types";

const q = new CRMQuery();

// GET /api/tasks → Task[]
// 后端从 crm/tasks/*.json 读 + join 客户的 customer_name。
export function listTasks(): Promise<Task[]> {
  return q.listTasks();
}

// PATCH /api/tasks/:id/status，body { status: <new> }
// 成功：{ ok: true, status: <new> }
// 失败：抛 CRMFetchError
export function updateTaskStatus(
  id: string,
  status: TaskStatus,
): Promise<{ ok: true; status: TaskStatus }> {
  return q.updateTaskStatus(id, status);
}
