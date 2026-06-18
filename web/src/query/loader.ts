// fetchJSON：统一的 JSON 加载器
// - 处理 4xx/5xx HTTP 状态
// - 处理网络错误（fetch reject）
// - 处理 JSON 解析错误
// 统一抛出 CRMFetchError，调用方可按 status 与 message 区分错误类型

export class CRMFetchError extends Error {
  constructor(
    public readonly path: string,
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "CRMFetchError";
  }
}

export async function fetchJSON<T>(path: string): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path);
  } catch (err) {
    throw new CRMFetchError(path, 0, `网络错误: ${(err as Error).message}`);
  }
  if (!res.ok) {
    throw new CRMFetchError(path, res.status, `HTTP ${res.status} ${res.statusText}`);
  }
  try {
    return (await res.json()) as T;
  } catch (err) {
    throw new CRMFetchError(path, res.status, `JSON 解析失败: ${(err as Error).message}`);
  }
}
