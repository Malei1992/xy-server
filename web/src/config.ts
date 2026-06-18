// 后端 API 的 base URL。
// 所有浏览器侧的 fetch 都拼成 `${API_BASE_URL}/xxx`，由 vite dev 代理（web/vite.config.ts）
// 转发到 Go 后端 server/。生产部署时通过 nginx 同源反代或环境变量覆盖。
export const API_BASE_URL = "/api";
