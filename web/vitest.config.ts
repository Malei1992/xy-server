import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import { resolve } from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    // 在 bun runtime 下，vi.mock 的相对路径解析不一致；
    // 通过 alias 把 src/* 路径显式映射到绝对路径，确保 mock 路径匹配。
    // 通用 alias "@/" → "src/" 覆盖所有子路径；具体路径保留以便 mock 时显式覆盖。
    alias: {
      "@": resolve(__dirname, "src"),
      "@/query": resolve(__dirname, "src/query/index.ts"),
      "@/config": resolve(__dirname, "src/config.ts"),
      "@/ui/format": resolve(__dirname, "src/ui/format.ts"),
      "@/ui/components/CustomerTable": resolve(__dirname, "src/ui/components/CustomerTable.tsx"),
      "@/ui/pages/CustomerDetail": resolve(__dirname, "src/ui/pages/CustomerDetail.tsx"),
      "@/ui/pages/CustomerList": resolve(__dirname, "src/ui/pages/CustomerList.tsx"),
      "@/ui/pages/Workbench": resolve(__dirname, "src/ui/pages/Workbench.tsx"),
      "@/ui/pages/NotFound": resolve(__dirname, "src/ui/pages/NotFound.tsx"),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: [resolve(__dirname, "tests/setup.ts")],
  },
});
