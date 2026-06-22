// 公开信息（opportunities）的独立 query 封装
// - listOpportunities / updateOpportunityStatus 都是 CRMQuery 类的薄包装

import { CRMQuery } from "./index";
import type { Opportunity, OpportunityStatus } from "./types";

const q = new CRMQuery();

// GET /api/opportunities → Opportunity[]
// 后端从 crm/opportunities/*.json 读 + join 客户的 customer_name。
export function listOpportunities(): Promise<Opportunity[]> {
  return q.listOpportunities();
}

// PATCH /api/opportunities/:id/status，body { status: <new> }
// 成功：{ ok: true, status: <new> }
// 失败：抛 CRMFetchError
export function updateOpportunityStatus(
  id: string,
  status: OpportunityStatus,
): Promise<{ ok: true; status: OpportunityStatus }> {
  return q.updateOpportunityStatus(id, status);
}
