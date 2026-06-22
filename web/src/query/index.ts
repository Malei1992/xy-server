// CRMQuery：对外统一查询入口
// - 数据源为 Go 后端（参见 server/），由 vite dev 代理转发
// - 所有方法均为 async，内部使用 fetchJSON 加载 JSON
// - 客户/邮件筛选在内存中完成

import { API_BASE_URL } from "@/config";
import { fetchJSON, CRMFetchError } from "./loader";
import type {
  Customer, Email, CustomerFilter, EmailFilter,
  IndexSummary, TimelineEvent, Site,
  Project, ProjectStatus, Task, TaskStatus, Opportunity, OpportunityStatus,
  WechatBindSubmitResult, WechatBindPollResult,
} from "./types";

export class CRMQuery {
  constructor(private base: string = API_BASE_URL) {}

  // 加载全局索引（含 customers/emails 列表与分桶统计）
  async getIndex(): Promise<IndexSummary> {
    return fetchJSON<IndexSummary>(`${this.base}/index`);
  }

  // 加载单个客户档案
  async getCustomer(id: string): Promise<Customer> {
    return fetchJSON<Customer>(`${this.base}/customers/${encodeURIComponent(id)}`);
  }

  // 加载并按筛选条件过滤客户列表
  // 后端从 crm/customers/*.json 一次性返回全量 Customer 列表（避免 N+1 fetch）。
  // 客户端仅做轻量内存过滤（filter 形参）。
  async listCustomers(filter?: CustomerFilter): Promise<Customer[]> {
    const customers = await fetchJSON<Customer[]>(`${this.base}/customers`);
    return applyCustomerFilter(customers, filter);
  }

  // 加载单封邮件
  async getEmail(id: string): Promise<Email> {
    return fetchJSON<Email>(`${this.base}/emails/${encodeURIComponent(id)}`);
  }

  // 加载并按筛选条件过滤邮件列表
  async listEmails(filter?: EmailFilter): Promise<Email[]> {
    const idx = await this.getIndex();
    const ids = idx.emails ?? [];
    const results = await Promise.allSettled(ids.map((id) => this.getEmail(id)));
    const emails = results
      .filter((r): r is PromiseFulfilledResult<Email> => r.status === "fulfilled")
      .map((r) => r.value);
    return applyEmailFilter(emails, filter);
  }

  // 加载某个客户的所有邮件
  async getCustomerEmails(customerId: string): Promise<Email[]> {
    return this.listEmails({ customerId });
  }

  // 加载某客户的时间线（倒序：最近在前）
  async getCustomerTimeline(customerId: string): Promise<TimelineEvent[]> {
    const c = await this.getCustomer(customerId);
    return [...c.timeline].sort((a, b) => (a.at < b.at ? 1 : -1));
  }

  // 上传 FAQ（.docx，≤5MB，ZIP magic 校验）
  // 响应也是 JSON（{ok, path, size}），但 Content-Type 是 multipart，
  // 不能用 fetchJSON 那个 res.json 之前的 ok 校验流程——其实可以，
  // fetchJSON 只读 body 拿 JSON，错误时抛 CRMFetchError。但我们
  // 需要 body 是 FormData 而不是默认的 JSON，所以另写一个不走 fetchJSON。
  async uploadFaq(file: File): Promise<UploadResponse> {
    return this.postFile("/uploads/faq", file);
  }

  // 上传"外宣材料"（.pdf，≤50MB，magic bytes 校验）
  async uploadAttachmentMoonstar(file: File): Promise<UploadResponse> {
    return this.postFile("/uploads/attachment-moonstar", file);
  }

  // 修改 .env 中的 SMTP/IMAP/审核白名单 keys（后端负责写盘与原子替换）
  // 请求：PATCH /api/config，body 是 { KEY: value, ... }
  // 成功：{ ok: true, env, updated }
  // 失败：抛出 CRMFetchError，message 来自 body.error
  async patchConfig(
    updates: Record<string, string>,
  ): Promise<{ ok: true; env: Record<string, string>; updated: string[] }> {
    const path = `${this.base}/config`;
    let res: Response;
    try {
      res = await fetch(path, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(updates),
      });
    } catch (err) {
      throw new CRMFetchError(path, 0, `网络错误: ${(err as Error).message}`);
    }
    if (!res.ok) {
      const body = (await res.json().catch(() => null)) as { error?: string } | null;
      throw new CRMFetchError(path, res.status, body?.error ?? `HTTP ${res.status}`);
    }
    return (await res.json()) as { ok: true; env: Record<string, string>; updated: string[] };
  }

  // 读取客户价值等级标准（4 个 level 标准）
  // 后端从 data/crm/grading_rules.json 读，文件缺失/空时返回 {}
  // 失败：抛出 CRMFetchError，message 来自 body.error
  async getGradingRules(): Promise<Record<string, string>> {
    return fetchJSON<Record<string, string>>(`${this.base}/grading-rules`);
  }

  // 写入客户价值等级标准
  // 请求：PATCH /api/grading-rules，body 必须含 4 个 keys（S/A/B/C）
  // 成功：返回服务端确认后的对象（同样是 4 个 keys）
  // 失败：400/500 → 抛 CRMFetchError（message 来自 body.error）
  async patchGradingRules(
    levels: Record<string, string>,
  ): Promise<Record<string, string>> {
    return this.patchJSON("/grading-rules", levels);
  }

  // 读取客户意向等级标准
  async getInterestLevel(): Promise<Record<string, string>> {
    return fetchJSON<Record<string, string>>(`${this.base}/interest-level`);
  }

  // 写入客户意向等级标准
  async patchInterestLevel(
    levels: Record<string, string>,
  ): Promise<Record<string, string>> {
    return this.patchJSON("/interest-level", levels);
  }

  // 更新客户基本信息中的联系人/电话字段
  // 请求：PATCH /api/customers/${id}，body 为 { contacts?, phones? }
  // 成功：返回更新后的完整 Customer 对象
  // 失败：抛出 CRMFetchError，message 来自 body.error
  async patchCustomer(
    id: string,
    partial: { contacts?: string | string[]; phones?: string | string[] },
  ): Promise<Customer> {
    const path = `${this.base}/customers/${encodeURIComponent(id)}`;
    let res: Response;
    try {
      res = await fetch(path, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(partial),
      });
    } catch (err) {
      throw new CRMFetchError(path, 0, `网络错误: ${(err as Error).message}`);
    }
    if (!res.ok) {
      const body = (await res.json().catch(() => null)) as { error?: string } | null;
      throw new CRMFetchError(path, res.status, body?.error ?? `HTTP ${res.status}`);
    }
    return (await res.json()) as Customer;
  }

  // 重启服务：调用 POST /api/restart，后端执行 start.sh -r
  async restart(): Promise<{ ok: true; output: string }> {
    const path = `${this.base}/restart`;
    let res: Response;
    try {
      res = await fetch(path, { method: "POST" });
    } catch (err) {
      throw new CRMFetchError(path, 0, `网络错误: ${(err as Error).message}`);
    }
    if (!res.ok) {
      const body = (await res.json().catch(() => null)) as { error?: string } | null;
      throw new CRMFetchError(path, res.status, body?.error ?? `HTTP ${res.status}`);
    }
    return (await res.json()) as { ok: true; output: string };
  }

  // ===== 微信绑定 =====

  // 提交微信绑定任务（异步）。调用后端 POST /api/wechat/bind，后端立即返 202 + task_id，
  // 不再同步等二维码生成（原本可能阻塞 ~2 分钟）。前端拿到 task_id 后用
  // getWechatBindStatus(task_id) 轮询拿结果。
  // 成功：{ task_id, status: "pending" }
  // 失败：抛出 CRMFetchError；body 可能是 { error, output, expired }
  //   - error: 错误描述（前端用于提示用户）
  //   - expired: 同上语义
  // 不发 body：契约要求无 body
  async bindWechat(): Promise<WechatBindSubmitResult> {
    const path = `${this.base}/wechat/bind`;
    let res: Response;
    try {
      res = await fetch(path, { method: "POST" });
    } catch (err) {
      throw new CRMFetchError(path, 0, `网络错误: ${(err as Error).message}`);
    }
    if (!res.ok) {
      // 5xx 时 body 含 error / output / expired 字段；4xx 走通用路径
      const body = (await res.json().catch(() => null)) as
        | { error?: string; output?: string; expired?: boolean }
        | null;
      throw new CRMFetchError(path, res.status, body?.error ?? `HTTP ${res.status}`);
    }
    return (await res.json()) as WechatBindSubmitResult;
  }

  // 轮询微信绑定任务状态。调用 GET /api/wechat/bind/:task_id。
  // 成功：{ task_id, status, link?, qr?, raw?, expired?, bound?, error? }
  //   - status: pending/running → 继续轮询；done → link/qr 可用；failed → error 有信息；expired → 任务超时
  //   - bound: true 时表示 openclaw 已成功连接微信（用户扫码后输出的标记），前端切到「绑定成功」状态
  // 失败：抛出 CRMFetchError
  //   - 404 → task 不存在（前端当作 expired 处理：「任务已过期，请重试」）
  //   - 5xx → 透传 body.error 到 message
  async getWechatBindStatus(task_id: string): Promise<WechatBindPollResult> {
    const path = `${this.base}/wechat/bind/${encodeURIComponent(task_id)}`;
    let res: Response;
    try {
      res = await fetch(path);
    } catch (err) {
      throw new CRMFetchError(path, 0, `网络错误: ${(err as Error).message}`);
    }
    if (!res.ok) {
      const body = (await res.json().catch(() => null)) as
        | { error?: string }
        | null;
      throw new CRMFetchError(path, res.status, body?.error ?? `HTTP ${res.status}`);
    }
    return (await res.json()) as WechatBindPollResult;
  }

  // 取消微信绑定任务。调用 POST /api/wechat/bind/:task_id/cancel。
  // 后端会通过 context cancel SIGKILL 整个 exec 进程组,免等 2 分钟 timeout 兜底。
  //
  // 语义:
  //   - 200 + cancelled: true   → 任务还在 running,已发 cancel 信号
  //   - 200 + cancelled: false  → 任务已终态(done/failed/expired),不重复 kill
  //   - 404                      → task 不存在(已 TTL 清理或从未存在)
  //   - 5xx                      → 透传 body.error 到 message
  //
  // 调用方通常 fire-and-forget: 关闭 modal 时调用,不管后端返啥都不阻塞用户操作。
  async cancelWechatBind(task_id: string): Promise<{ cancelled: boolean; reason?: string }> {
    const path = `${this.base}/wechat/bind/${encodeURIComponent(task_id)}/cancel`;
    let res: Response;
    try {
      res = await fetch(path, { method: "POST" });
    } catch (err) {
      throw new CRMFetchError(path, 0, `网络错误: ${(err as Error).message}`);
    }
    if (!res.ok) {
      const body = (await res.json().catch(() => null)) as
        | { error?: string }
        | null;
      throw new CRMFetchError(path, res.status, body?.error ?? `HTTP ${res.status}`);
    }
    return (await res.json()) as { cancelled: boolean; reason?: string };
  }

  // ===== 公开数据源 =====

  // 列出全部 sites；可选 q 按 name 做子串模糊过滤。
  // 后端缺文件 / 空文件 → 200 + []（不会 404）。
  async getSites(q?: string): Promise<Site[]> {
    const qs = q ? `?q=${encodeURIComponent(q)}` : "";
    return fetchJSON<Site[]>(`${this.base}/target-sites${qs}`);
  }

  // 新增一条 site。name + url 必填（前端可提前校验；后端也会校验）。
  // name 与现有重复 → 后端返 400 → 抛 CRMFetchError。
  async addSite(site: Site): Promise<Site> {
    const path = `${this.base}/target-sites`;
    let res: Response;
    try {
      res = await fetch(path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(site),
      });
    } catch (err) {
      throw new CRMFetchError("/target-sites", 0, `网络错误: ${(err as Error).message}`);
    }
    if (!res.ok) {
      const body = (await res.json().catch(() => null)) as { error?: string } | null;
      throw new CRMFetchError("/target-sites", res.status, body?.error ?? `HTTP ${res.status}`);
    }
    return (await res.json()) as Site;
  }

  // 按 name 精确匹配并部分更新；partial 不能含 name（name 是 identifier）。
  // 失败：name 不存在 / body 含 name → 抛 CRMFetchError。
  async updateSite(name: string, partial: Partial<Omit<Site, "name">>): Promise<Site> {
    const qs = `?name=${encodeURIComponent(name)}`;
    const path = `${this.base}/target-sites${qs}`;
    let res: Response;
    try {
      res = await fetch(path, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(partial),
      });
    } catch (err) {
      throw new CRMFetchError("/target-sites", 0, `网络错误: ${(err as Error).message}`);
    }
    if (!res.ok) {
      const body = (await res.json().catch(() => null)) as { error?: string } | null;
      throw new CRMFetchError("/target-sites", res.status, body?.error ?? `HTTP ${res.status}`);
    }
    return (await res.json()) as Site;
  }

  // 按 name 精确删除。
  async deleteSite(name: string): Promise<{ deleted: string }> {
    const qs = `?name=${encodeURIComponent(name)}`;
    const path = `${this.base}/target-sites${qs}`;
    let res: Response;
    try {
      res = await fetch(path, { method: "DELETE" });
    } catch (err) {
      throw new CRMFetchError("/target-sites", 0, `网络错误: ${(err as Error).message}`);
    }
    if (!res.ok) {
      const body = (await res.json().catch(() => null)) as { error?: string } | null;
      throw new CRMFetchError("/target-sites", res.status, body?.error ?? `HTTP ${res.status}`);
    }
    return (await res.json()) as { deleted: string };
  }

  // ===== 商机信息 =====

  // 列出全部商机（已 join 客户的展示字段）。
  // 后端从 crm/projects/*.json 读 + crm/customers/{id}.json 关联。
  // 项目目录不存在 / 空 → 200 + []。
  async listProjects(): Promise<Project[]> {
    return fetchJSON<Project[]>(`${this.base}/projects`);
  }

  // 修改单个商机的跟进状态
  // 请求：PATCH /api/projects/:id/status，body { status: <new> }
  // 成功：{ ok: true, status: <new> }
  // 失败：400 status 不在枚举内 / 404 项目不存在 / 500 后端 I/O 失败
  // 抛出 CRMFetchError，message 来自 body.error
  async updateProjectStatus(
    id: string,
    status: ProjectStatus,
  ): Promise<{ ok: true; status: ProjectStatus }> {
    return this.patchJSONRaw<{ ok: true; status: ProjectStatus }>(
      `/projects/${encodeURIComponent(id)}/status`,
      { status },
    );
  }

  // ===== 代办任务 =====

  // 列出全部代办任务（已 join 客户的 customer_name）。
  // 后端从 crm/tasks/*.json 读 + crm/customers/{id}.json 关联。
  // 任务目录不存在 / 空 → 200 + []。
  // customer_id 找不到 / 客户文件损坏 → 该条 customer_name 字段空字符串。
  async listTasks(): Promise<Task[]> {
    return fetchJSON<Task[]>(`${this.base}/tasks`);
  }

  // 修改单个任务的状态
  // 请求：PATCH /api/tasks/:id/status，body { status: <new> }
  // 成功：{ ok: true, status: <new> }
  // 失败：400 status 不在枚举内 / 404 任务不存在 / 500 后端 I/O 失败
  async updateTaskStatus(
    id: string,
    status: TaskStatus,
  ): Promise<{ ok: true; status: TaskStatus }> {
    return this.patchJSONRaw<{ ok: true; status: TaskStatus }>(
      `/tasks/${encodeURIComponent(id)}/status`,
      { status },
    );
  }

  // ===== 公开信息 =====

  // 列出全部公开信息（已 join 客户的 customer_name）。
  // 后端从 crm/opportunities/*.json 读 + crm/customers/{id}.json 关联。
  // 目录不存在 / 空 → 200 + []。
  // customer_id 找不到 / 客户文件损坏 → 该条 customer_name 字段空字符串。
  // 后端仅读取以 OPP 开头的文件，其余视为 stray 跳过。
  async listOpportunities(): Promise<Opportunity[]> {
    return fetchJSON<Opportunity[]>(`${this.base}/opportunities`);
  }

  // 修改单个公开信息的状态
  // 请求：PATCH /api/opportunities/:id/status，body { status: <new> }
  // 成功：{ ok: true, status: <new> }
  // 失败：400 status 不在枚举内 / 404 记录不存在 / 500 后端 I/O 失败
  async updateOpportunityStatus(
    id: string,
    status: OpportunityStatus,
  ): Promise<{ ok: true; status: OpportunityStatus }> {
    return this.patchJSONRaw<{ ok: true; status: OpportunityStatus }>(
      `/opportunities/${encodeURIComponent(id)}/status`,
      { status },
    );
  }

  // 通用 JSON PATCH 辅助
  // - 用于 grading-rules / interest-level 这类"替换写 4 个 keys"端点
  // - 失败时从 body.error 提取 message（body 非 JSON 时退回 HTTP <status>）
  private async patchJSON(
    path: string,
    body: Record<string, string>,
  ): Promise<Record<string, string>> {
    return this.patchJSONRaw<Record<string, string>>(path, body);
  }

  // 通用 JSON PATCH 辅助（带泛型 body / response）
  // - 跟 loader.patchJSON 行为一致（失败抛 CRMFetchError，message 取 body.error）
  // - 区别：拼上 base 前缀（base = API_BASE_URL by default）
  // - 用途：updateXxxStatus 等需要传强类型 body / 拿强类型 response 的端点
  private async patchJSONRaw<T>(path: string, body: unknown): Promise<T> {
    const fullPath = `${this.base}${path}`;
    let res: Response;
    try {
      res = await fetch(fullPath, {
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

  // 通用 multipart 上传辅助
  private async postFile(path: string, file: File): Promise<UploadResponse> {
    const fd = new FormData();
    fd.append("file", file);
    let res: Response;
    try {
      res = await fetch(`${this.base}${path}`, {
        method: "POST",
        body: fd,
      });
    } catch (err) {
      throw new CRMFetchError(path, 0, `网络错误: ${(err as Error).message}`);
    }
    if (!res.ok) {
      // 尝试读 JSON 错误体
      const body = (await res.json().catch(() => null)) as { error?: string } | null;
      const msg = body?.error ?? `HTTP ${res.status}`;
      throw new CRMFetchError(path, res.status, msg);
    }
    return (await res.json()) as UploadResponse;
  }
}

// ----- 内部工具函数：客户端内存筛选 -----

function applyCustomerFilter(list: Customer[], filter?: CustomerFilter): Customer[] {
  if (!filter) return list;
  return list.filter((c) => {
    if (filter.status?.length && (!c.engagement?.status || !filter.status.includes(c.engagement.status))) {
      return false;
    }
    if (filter.country?.length && !filter.country.includes(c.basic.country)) {
      return false;
    }
    if (filter.intentLevel?.length) {
      const lvl = c.engagement?.intent_level;
      if (!lvl || !filter.intentLevel.includes(lvl)) return false;
    }
    if (filter.grade?.length) {
      const g = c.prospecting?.grade;
      if (!g || !filter.grade.includes(g)) return false;
    }
    return true;
  });
}

function applyEmailFilter(list: Email[], filter?: EmailFilter): Email[] {
  if (!filter) return list;
  return list.filter((e) => {
    if (filter.customerId && e.customer_id !== filter.customerId) return false;
    if (filter.direction?.length && !filter.direction.includes(e.direction)) return false;
    if (filter.status?.length && !filter.status.includes(e.status)) return false;
    if (filter.type?.length && !filter.type.includes(e.type)) return false;
    return true;
  });
}

// 静默使用：调用方需要知道"被跳过的 404 数量"时可通过此函数统计
export function countRejected(results: PromiseSettledResult<unknown>[]): number {
  return results.filter((r) => r.status === "rejected").length;
}

// 重新导出 CRMFetchError,方便调用方判断 404
export { CRMFetchError };

// 上传端点成功响应：{ok: true, path, size}
export type UploadResponse = { ok: true; path: string; size: number };
