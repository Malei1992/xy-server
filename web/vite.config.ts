/// <reference types="node" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));

// 后端监听地址（与 server/paths.go 的 ListenAddr 对齐）。
// 同机部署：后端监听 0.0.0.0:15373，nginx 在 127.0.0.1 代理 /api/*；dev 模式也走 loopback。
const BACKEND_ORIGIN = "http://127.0.0.1:15373";

export default defineConfig({
  // .env 文件目录：项目根目录的 .env（由 Go 后端 server/ 读取）
  envDir: resolve(__dirname, ".."),
  // 允许非 VITE_ 前缀的变量被注入到 import.meta.env，
  // 以支持 SMTP_/IMAP_/EMAIL_/REVIEWER_ 等业务变量在系统页直接展示
  envPrefix: ["VITE_", "SMTP_", "IMAP_", "EMAIL_", "REVIEWER_"],
  resolve: {
    alias: {
      "@": resolve(__dirname, "src"),
    },
  },
  server: {
    // 绑死 IPv4 127.0.0.1:5173（macOS 上 localhost 走 ::1 跟 bun+vite 兼容性差）
    host: "127.0.0.1",
    port: 5173,
    // 把 /api/* 转发到 Go 后端；前端所有 fetch('/api/...') 都走这里
    proxy: {
      "/api": {
        target: BACKEND_ORIGIN,
        changeOrigin: true,
      },
    },
  },
  plugins: [react()],
});
