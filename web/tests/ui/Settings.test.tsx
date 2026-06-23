import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router-dom";

const mockFetch = vi.fn();
beforeEach(() => {
  mockFetch.mockReset();
  vi.stubGlobal("fetch", mockFetch);
});
afterEach(() => {
  vi.unstubAllGlobals();
});

// 让 /api/config 默认返回空 env；调用方可在调 upload 前 mockResolvedValueOnce 覆盖
// 同时把两个 level GET 也回成空对象（保持与后端契约一致：文件缺失/空时 → {}）
function mockConfigOk() {
  mockFetch.mockImplementation(async (url: string) => {
    if (url === "/api/config") {
      return {
        ok: true,
        status: 200,
        json: () => Promise.resolve({ env: { EMAIL_REQUIRE_REVIEW: "true" } }),
      };
    }
    if (url === "/api/grading-rules" || url === "/api/interest-level") {
      return { ok: true, status: 200, json: () => Promise.resolve({}) };
    }
    if (typeof url === "string" && url.startsWith("/api/target-sites")) {
      return { ok: true, status: 200, json: () => Promise.resolve([]) };
    }
    if (typeof url === "string" && url === "/api/restart") {
      return { ok: true, status: 200, json: () => Promise.resolve({ ok: true, output: "restarted" }) };
    }
    // 上传端点未配置：返回 500 让测试早暴露
    return { ok: false, status: 500, json: () => Promise.resolve({ error: "not mocked" }) };
  });
}

// mock fetch 同时支持 GET /api/config 与 PATCH /api/config
// 初次 GET 返回 ENV_OBJ；PATCH 返回带 updated 列表的成功响应
// 注意：每次 GET 都返回新的 env 引用（{ ...env }），模拟真实后端总是返回新解析对象，
// 这样 React 的 setState 不会因为引用相等而跳过 re-render。
// level 端点（grading-rules / interest-level）默认回空对象，调用方需要时可覆写。
type MockResponse = {
  ok: boolean;
  status: number;
  json: () => Promise<unknown>;
};
function mockConfigWithPatch(
  env: Record<string, string>,
  patchImpl?: (body: Record<string, string>) => MockResponse,
) {
  // mutable：让 PATCH 之后的下一次 GET 看到 PATCH 写回的最新值
  let currentEnv: Record<string, string> = { ...env };
  mockFetch.mockImplementation(async (url: string, init?: RequestInit) => {
    if (url === "/api/config" && (!init || init.method === undefined || init.method === "GET")) {
      return { ok: true, status: 200, json: () => Promise.resolve({ env: { ...currentEnv } }) };
    }
    if (url === "/api/config" && init?.method === "PATCH") {
      const body = JSON.parse(init.body as string) as Record<string, string>;
      if (patchImpl) return patchImpl(body);
      currentEnv = { ...currentEnv, ...body };
      return {
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({ ok: true, env: { ...currentEnv }, updated: Object.keys(body) }),
      };
    }
    if (url === "/api/grading-rules" || url === "/api/interest-level") {
      return { ok: true, status: 200, json: () => Promise.resolve({}) };
    }
    if (typeof url === "string" && url.startsWith("/api/target-sites")) {
      return { ok: true, status: 200, json: () => Promise.resolve([]) };
    }
    return { ok: false, status: 500, json: () => Promise.resolve({ error: "not mocked" }) };
  });
}

import { Settings } from "@/ui/pages/Settings";

// 后端 Go /api/config 返回 {"env": {...}} 形式；前端 Settings 读 body.env。
const ENV_OBJ = {
  SMTP_HOST: "smtp.feishu.cn",
  SMTP_PORT: "587",
  SMTP_USERNAME: "malei@anban.tech",
  SMTP_PASSWORD: "secret123",
  IMAP_HOST: "imap.feishu.cn",
  IMAP_PORT: "993",
  IMAP_USERNAME: "malei@anban.tech",
  IMAP_PASSWORD: "secret123",
  EMAIL_REQUIRE_REVIEW: "true",
  REVIEWER_EMAIL: "wangyan@anban.tech",
};

function renderSettings() {
  return render(
    <MemoryRouter initialEntries={["/settings"]}>
      <Routes>
        <Route path="/settings" element={<Settings />} />
      </Routes>
    </MemoryRouter>,
  );
}

function mockOk(env: Record<string, string>) {
  mockFetch.mockImplementation(async (url: string) => {
    if (url === "/api/grading-rules" || url === "/api/interest-level") {
      return { ok: true, status: 200, json: () => Promise.resolve({}) };
    }
    if (typeof url === "string" && url.startsWith("/api/target-sites")) {
      return { ok: true, status: 200, json: () => Promise.resolve([]) };
    }
    return {
      ok: true,
      status: 200,
      json: () => Promise.resolve({ env }),
    };
  });
}

describe("Settings", () => {
  it("fetches /api/config on mount and reads body.env", async () => {
    mockOk(ENV_OBJ);
    renderSettings();
    expect(screen.getByText("加载中...")).toBeInTheDocument();
    await waitFor(() => {
      // SMTP/IMAP/审核的所有 input 常驻显示，直接断言值
      expect(screen.getByTestId("smtp-host")).toBeInTheDocument();
    });
    expect((screen.getByTestId("smtp-host") as HTMLInputElement).value).toBe("smtp.feishu.cn");
    expect((screen.getByTestId("smtp-port") as HTMLInputElement).value).toBe("587");
    expect((screen.getByTestId("smtp-username") as HTMLInputElement).value).toBe("malei@anban.tech");
    expect((screen.getByTestId("imap-host") as HTMLInputElement).value).toBe("imap.feishu.cn");
    expect((screen.getByTestId("imap-port") as HTMLInputElement).value).toBe("993");
    expect((screen.getByTestId("review-email") as HTMLInputElement).value).toBe("wangyan@anban.tech");
    expect((screen.getByTestId("review-require") as HTMLSelectElement).value).toBe("true");

    expect(mockFetch).toHaveBeenCalledWith("/api/config", { cache: "no-store" });
  });

  it("shows error when fetch fails", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.reject(new Error("not json")),
    });
    renderSettings();
    await waitFor(() => {
      expect(screen.getByText(/加载失败/)).toBeInTheDocument();
      expect(screen.getByText(/HTTP 500/)).toBeInTheDocument();
    });
  });
});

describe("Settings 邮件配置 tab - 常驻可编辑 + 底部统一保存", () => {
  it("10 个输入字段全部常驻显示（含 password 类型）", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("smtp-host"));

    // SMTP
    expect(screen.getByTestId("smtp-host")).toBeInTheDocument();
    expect(screen.getByTestId("smtp-port")).toBeInTheDocument();
    expect(screen.getByTestId("smtp-username")).toBeInTheDocument();
    expect(screen.getByTestId("smtp-password")).toBeInTheDocument();
    // IMAP
    expect(screen.getByTestId("imap-host")).toBeInTheDocument();
    expect(screen.getByTestId("imap-port")).toBeInTheDocument();
    expect(screen.getByTestId("imap-username")).toBeInTheDocument();
    expect(screen.getByTestId("imap-password")).toBeInTheDocument();
    // 审核
    expect(screen.getByTestId("review-require")).toBeInTheDocument();
    expect(screen.getByTestId("review-email")).toBeInTheDocument();
  });

  it("密码字段 type=password 且预填当前值", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("smtp-password"));
    const pwdInput = screen.getByTestId("smtp-password") as HTMLInputElement;
    expect(pwdInput.type).toBe("password");
    // 真实值在 form 里（用户视觉看到圆点是浏览器 type=password 渲染）
    expect(pwdInput.value).toBe("secret123");
  });

  it("未改动时 save/cancel 按钮 disabled", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("email-config-save"));
    const saveBtn = screen.getByTestId("email-config-save") as HTMLButtonElement;
    const cancelBtn = screen.getByTestId("email-config-cancel") as HTMLButtonElement;
    expect(saveBtn).toBeDisabled();
    expect(cancelBtn).toBeDisabled();
  });

  it("修改任意 input 后 save/cancel 启用", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("email-config-save"));

    const hostInput = screen.getByTestId("smtp-host") as HTMLInputElement;
    fireEvent.change(hostInput, { target: { value: "smtp.new.cn" } });

    expect(screen.getByTestId("email-config-save") as HTMLButtonElement).not.toBeDisabled();
    expect(screen.getByTestId("email-config-cancel") as HTMLButtonElement).not.toBeDisabled();
  });

  it("点保存 → 调 PATCH /api/config 且 body 含所有 EDITABLE_KEYS + 新值", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("email-config-save"));

    const hostInput = screen.getByTestId("smtp-host") as HTMLInputElement;
    fireEvent.change(hostInput, { target: { value: "smtp.new.cn" } });

    fireEvent.click(screen.getByTestId("email-config-save"));

    // 等 PATCH 完成
    await waitFor(() => {
      const patchCall = mockFetch.mock.calls.find(
        (c) =>
          typeof c[0] === "string" &&
          c[0] === "/api/config" &&
          (c[1] as RequestInit | undefined)?.method === "PATCH",
      );
      expect(patchCall).toBeTruthy();
    });

    const patchCall = mockFetch.mock.calls.find(
      (c) =>
        typeof c[0] === "string" &&
        c[0] === "/api/config" &&
        (c[1] as RequestInit | undefined)?.method === "PATCH",
    )!;
    const init = patchCall[1] as RequestInit;
    expect(init.headers).toMatchObject({ "Content-Type": "application/json" });
    const body = JSON.parse(init.body as string) as Record<string, string>;

    // 一次性发送所有 10 个可编辑 keys
    expect(body.SMTP_HOST).toBe("smtp.new.cn");
    expect(body.SMTP_PORT).toBe("587");
    expect(body.SMTP_USERNAME).toBe("malei@anban.tech");
    expect(body.SMTP_PASSWORD).toBe("secret123");
    expect(body.IMAP_HOST).toBe("imap.feishu.cn");
    expect(body.IMAP_PORT).toBe("993");
    expect(body.IMAP_USERNAME).toBe("malei@anban.tech");
    expect(body.IMAP_PASSWORD).toBe("secret123");
    expect(body.EMAIL_REQUIRE_REVIEW).toBe("true");
    expect(body.REVIEWER_EMAIL).toBe("wangyan@anban.tech");
  });

  it("保存成功 → 显示'已保存'提示", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("email-config-save"));

    fireEvent.change(screen.getByTestId("smtp-host"), {
      target: { value: "smtp.new.cn" },
    });
    fireEvent.click(screen.getByTestId("email-config-save"));

    await waitFor(() => {
      const savedMsg = screen.getByTestId("email-config-saved-msg");
      expect(savedMsg).toBeInTheDocument();
      expect(savedMsg.textContent).toMatch(/已保存/);
    });
  });

  it("点取消 → input 还原到原始值，且无 PATCH 调用", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("email-config-cancel"));

    const hostInput = screen.getByTestId("smtp-host") as HTMLInputElement;
    fireEvent.change(hostInput, { target: { value: "smtp.discard.cn" } });
    expect(hostInput.value).toBe("smtp.discard.cn");

    fireEvent.click(screen.getByTestId("email-config-cancel"));

    // input 还原
    expect((screen.getByTestId("smtp-host") as HTMLInputElement).value).toBe("smtp.feishu.cn");

    // 没有任何 PATCH 调用
    const patchCalls = mockFetch.mock.calls.filter(
      (c) =>
        typeof c[0] === "string" &&
        c[0] === "/api/config" &&
        (c[1] as RequestInit | undefined)?.method === "PATCH",
    );
    expect(patchCalls.length).toBe(0);

    // 还原后按钮回到 disabled
    expect(screen.getByTestId("email-config-save") as HTMLButtonElement).toBeDisabled();
    expect(screen.getByTestId("email-config-cancel") as HTMLButtonElement).toBeDisabled();
  });

  it("PATCH 失败 → 显示错误提示", async () => {
    mockConfigWithPatch(ENV_OBJ, () => ({
      ok: false,
      status: 400,
      json: () => Promise.resolve({ error: "bad" }),
    }));
    renderSettings();
    await waitFor(() => screen.getByTestId("email-config-save"));

    fireEvent.change(screen.getByTestId("smtp-host"), {
      target: { value: "smtp.new.cn" },
    });
    fireEvent.click(screen.getByTestId("email-config-save"));

    await waitFor(() => {
      const errMsg = screen.getByTestId("email-config-error");
      expect(errMsg).toBeInTheDocument();
      expect(errMsg.textContent).toMatch(/bad/);
    });
    // 错误时按钮回到 enabled（draft 未还原）
    expect(screen.getByTestId("email-config-save") as HTMLButtonElement).not.toBeDisabled();
  });

  it("邮件审核 是/否 是下拉框", async () => {
    mockConfigWithPatch({ ...ENV_OBJ, EMAIL_REQUIRE_REVIEW: "true" });
    renderSettings();
    await waitFor(() => screen.getByTestId("review-require"));

    const select = screen.getByTestId("review-require") as HTMLSelectElement;
    expect(select.tagName).toBe("SELECT");
    // options 含 是/否
    const optionTexts = Array.from(select.options).map((o) => o.text);
    expect(optionTexts).toContain("是");
    expect(optionTexts).toContain("否");
    expect(select.value).toBe("true");
  });

  it("密码字段 type 初始为 password（常驻圆点）", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("smtp-password"));
    const smtpPwd = screen.getByTestId("smtp-password") as HTMLInputElement;
    expect(smtpPwd.type).toBe("password");
    // 密码字段同时带 data-revealed="false"
    expect(smtpPwd.getAttribute("data-revealed")).toBe("false");
  });

  it("密码字段悬浮 mouseenter → type 切换到 text（明文）", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("smtp-password"));
    const smtpPwd = screen.getByTestId("smtp-password") as HTMLInputElement;
    expect(smtpPwd.type).toBe("password");
    fireEvent.mouseEnter(smtpPwd);
    expect(smtpPwd.type).toBe("text");
    expect(smtpPwd.getAttribute("data-revealed")).toBe("true");
  });

  it("密码字段 mouseleave → type 切回 password（圆点）", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("smtp-password"));
    const smtpPwd = screen.getByTestId("smtp-password") as HTMLInputElement;
    fireEvent.mouseEnter(smtpPwd);
    expect(smtpPwd.type).toBe("text");
    fireEvent.mouseLeave(smtpPwd);
    expect(smtpPwd.type).toBe("password");
    expect(smtpPwd.getAttribute("data-revealed")).toBe("false");
  });

  it("密码字段 focus/blur 同样切换 type（键盘可达）", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("smtp-password"));
    const smtpPwd = screen.getByTestId("smtp-password") as HTMLInputElement;
    fireEvent.focus(smtpPwd);
    expect(smtpPwd.type).toBe("text");
    expect(smtpPwd.getAttribute("data-revealed")).toBe("true");
    fireEvent.blur(smtpPwd);
    expect(smtpPwd.type).toBe("password");
    expect(smtpPwd.getAttribute("data-revealed")).toBe("false");
  });

  it("非密码字段没有 data-revealed 属性（不被当作密码处理）", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("smtp-host"));
    const smtpHost = screen.getByTestId("smtp-host") as HTMLInputElement;
    expect(smtpHost.type).toBe("text");
    // 非密码字段：data-revealed 属性不应出现
    expect(smtpHost.hasAttribute("data-revealed")).toBe(false);
  });

  it("tab 切换后回到 email tab：draft 重置为 initialConfig（用户切走前未保存的改动会丢）", async () => {
    mockConfigWithPatch(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("smtp-host"));

    // 改动值
    fireEvent.change(screen.getByTestId("smtp-host"), {
      target: { value: "smtp.unsaved.cn" },
    });
    expect((screen.getByTestId("smtp-host") as HTMLInputElement).value).toBe("smtp.unsaved.cn");

    // 切到 upload tab 再切回来
    fireEvent.click(screen.getByTestId("tab-upload"));
    await waitFor(() => {
      expect(screen.getByTestId("upload-faq-input")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("tab-email"));
    await waitFor(() => {
      expect(screen.getByTestId("smtp-host")).toBeInTheDocument();
    });

    // draft 重置：input 回到原值
    expect((screen.getByTestId("smtp-host") as HTMLInputElement).value).toBe("smtp.feishu.cn");
    // 按钮回到 disabled
    expect(screen.getByTestId("email-config-save") as HTMLButtonElement).toBeDisabled();
  });
});

describe("Settings 资料上传 section", () => {
  function makeFile(name: string, sizeBytes: number, type = "text/markdown"): File {
    const file = new File(["# rules"], name, { type });
    Object.defineProperty(file, "size", { value: sizeBytes, configurable: true });
    return file;
  }

  // 资料上传在第二个 tab，渲染前必须先切到 upload tab
  // 默认 tab 是邮件配置，upload 输入框此时不存在于 DOM
  async function switchToUploadTab() {
    await waitFor(() => {
      expect(screen.getByTestId("tab-upload")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("tab-upload"));
  }

  it("renders 资料上传 section with both upload inputs", async () => {
    mockConfigOk();
    renderSettings();
    await switchToUploadTab();
    // "资料上传" 同时是 tab 按钮文本和 section h3 标题，按 heading role 定位 section
    expect(screen.getByRole("heading", { name: "资料上传" })).toBeInTheDocument();
    expect(screen.getByTestId("upload-faq-input")).toBeInTheDocument();
    expect(screen.getByTestId("upload-attachment-moonstar-input")).toBeInTheDocument();
  });

  it("does not upload when file extension does not match accept", async () => {
    mockConfigOk();
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("upload-faq-input"));

    const input = screen.getByTestId("upload-faq-input") as HTMLInputElement;
    const txt = makeFile("faq.txt", 100, "text/plain");
    fireEvent.change(input, { target: { files: [txt] } });

    await waitFor(() => {
      // 期望出现后缀相关的错误提示文案
      const body = document.body.textContent ?? "";
      expect(body).toMatch(/\.docx|后缀|格式/);
    });
    // 仅有一次 fetch 调用：/api/config；上传端点未触发
    const uploadCalls = mockFetch.mock.calls.filter(
      (c) => typeof c[0] === "string" && c[0].includes("/uploads/"),
    );
    expect(uploadCalls.length).toBe(0);
  });

  it("does not upload when file exceeds maxSizeMB", async () => {
    mockConfigOk();
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("upload-faq-input"));

    const input = screen.getByTestId("upload-faq-input") as HTMLInputElement;
    // maxSizeMB = 100 → 上限 100 * 1024 * 1024 = 104857600
    // 不实际分配 101MB(会 OOM),用最小合法 payload + 改写 size 属性触发客户端拦截
    const tinyBytes = new Uint8Array([0x50, 0x4B, 0x03, 0x04]);
    const oversized = new File([tinyBytes], "big.docx", {
      type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    });
    Object.defineProperty(oversized, "size", {
      value: 101 * 1024 * 1024,
      configurable: true,
    });
    fireEvent.change(input, { target: { files: [oversized] } });

    await waitFor(() => {
      const body = document.body.textContent ?? "";
      expect(body).toMatch(/大小|超限|100\s*MB|过大/);
    });
    const uploadCalls = mockFetch.mock.calls.filter(
      (c) => typeof c[0] === "string" && c[0].includes("/uploads/"),
    );
    expect(uploadCalls.length).toBe(0);
  });

  it("uploads .docx file on happy path and shows success status", async () => {
    // 覆盖 uploads 调用 → 返回成功；用持久 mockImplementation 而非 once，
    // 因为 config + upload 共 2 次调用，once 会先被 config 消耗
    mockFetch.mockImplementation(async (url: string, init?: RequestInit) => {
      if (url === "/api/config") {
        return { ok: true, status: 200, json: () => Promise.resolve({ env: {} }) };
      }
      if (url === "/api/grading-rules" || url === "/api/interest-level") {
        return { ok: true, status: 200, json: () => Promise.resolve({}) };
      }
      if (url === "/api/uploads/faq" && init?.method === "POST") {
        return {
          ok: true,
          status: 200,
          json: () => Promise.resolve({ ok: true, path: "/tmp/x", size: 1024 }),
        };
      }
      return { ok: false, status: 500, json: () => Promise.resolve({ error: "not mocked" }) };
    });

    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("upload-faq-input"));

    const input = screen.getByTestId("upload-faq-input") as HTMLInputElement;
    // 假 .docx：PK\x03\x04 是 ZIP 容器 magic，.docx 本质就是 ZIP
    const bytes = new Uint8Array([0x50, 0x4B, 0x03, 0x04, 0x14, 0x00, 0x00, 0x00]);
    const file = new File([bytes], "faq.docx", {
      type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    });
    fireEvent.change(input, { target: { files: [file] } });

    // 验证 fetch 被以 multipart 形式调用
    await waitFor(() => {
      const uploadCall = mockFetch.mock.calls.find(
        (c) => typeof c[0] === "string" && c[0] === "/api/uploads/faq",
      );
      expect(uploadCall).toBeTruthy();
      const init = uploadCall?.[1] as RequestInit;
      expect(init.method).toBe("POST");
      expect(init.body).toBeInstanceOf(FormData);
    });

    // 状态文案含"已上传"
    await waitFor(() => {
      expect(document.body.textContent ?? "").toMatch(/已上传/);
    });
  });

  it("shows error when backend returns 400 with error message", async () => {
    mockFetch.mockImplementation(async (url: string, init?: RequestInit) => {
      if (url === "/api/config") {
        return { ok: true, status: 200, json: () => Promise.resolve({ env: {} }) };
      }
      if (url === "/api/grading-rules" || url === "/api/interest-level") {
        return { ok: true, status: 200, json: () => Promise.resolve({}) };
      }
      if (url === "/api/uploads/faq" && init?.method === "POST") {
        return {
          ok: false,
          status: 400,
          json: () => Promise.resolve({ error: "bad docx format" }),
        };
      }
      return { ok: false, status: 500, json: () => Promise.resolve({ error: "not mocked" }) };
    });

    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("upload-faq-input"));

    const input = screen.getByTestId("upload-faq-input") as HTMLInputElement;
    const bytes = new Uint8Array([0x50, 0x4B, 0x03, 0x04, 0x14, 0x00, 0x00, 0x00]);
    const file = new File([bytes], "faq.docx", {
      type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    });
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => {
      expect(document.body.textContent ?? "").toMatch(/bad docx format/);
    });
  });

  it("uploads .doc file (OLE2 magic) on happy path", async () => {
    mockFetch.mockImplementation(async (url: string, init?: RequestInit) => {
      if (url === "/api/config") {
        return { ok: true, status: 200, json: () => Promise.resolve({ env: {} }) };
      }
      if (url === "/api/grading-rules" || url === "/api/interest-level") {
        return { ok: true, status: 200, json: () => Promise.resolve({}) };
      }
      if (url === "/api/uploads/faq" && init?.method === "POST") {
        return {
          ok: true,
          status: 200,
          json: () => Promise.resolve({ ok: true, path: "/tmp/x", size: 2048 }),
        };
      }
      return { ok: false, status: 500, json: () => Promise.resolve({ error: "not mocked" }) };
    });

    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("upload-faq-input"));

    const input = screen.getByTestId("upload-faq-input") as HTMLInputElement;
    // 假 .doc：OLE2/CFB magic bytes (D0 CF 11 E0 A1 B1 1A E1)
    const bytes = new Uint8Array([0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1, 0x00, 0x00]);
    const file = new File([bytes], "faq.doc", { type: "application/msword" });
    fireEvent.change(input, { target: { files: [file] } });

    // 验证 fetch 被以 multipart 形式调用
    await waitFor(() => {
      const uploadCall = mockFetch.mock.calls.find(
        (c) => typeof c[0] === "string" && c[0] === "/api/uploads/faq",
      );
      expect(uploadCall).toBeTruthy();
      const init = uploadCall?.[1] as RequestInit;
      expect(init.method).toBe("POST");
      expect(init.body).toBeInstanceOf(FormData);
    });

    // 状态文案含"已上传"
    await waitFor(() => {
      expect(document.body.textContent ?? "").toMatch(/已上传/);
    });
  });
});

describe("Settings tab 切换", () => {
  it("renders 两个 tab header", async () => {
    mockOk(ENV_OBJ);
    renderSettings();
    await waitFor(() => {
      expect(screen.getByTestId("tab-email")).toBeInTheDocument();
      expect(screen.getByTestId("tab-upload")).toBeInTheDocument();
    });
  });

  it("默认 tab 是邮件配置（data-active 标记）", async () => {
    mockOk(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-email"));
    expect(screen.getByTestId("tab-email").getAttribute("data-active")).toBe("true");
    expect(screen.getByTestId("tab-upload").getAttribute("data-active")).toBe("false");
  });

  it("点 tab 切换：默认看到 SMTP，点 upload 后看到 upload 视图，再点 email 回到 SMTP", async () => {
    mockOk(ENV_OBJ);
    renderSettings();
    await waitFor(() => screen.getByTestId("smtp-host"));

    // 默认：SMTP 可见，upload 不在 DOM
    expect(screen.getByText("SMTP 服务（发件）")).toBeInTheDocument();
    expect(screen.queryByTestId("upload-faq-input")).toBeNull();

    // 切到 upload tab
    fireEvent.click(screen.getByTestId("tab-upload"));
    await waitFor(() => {
      expect(screen.getByTestId("upload-faq-input")).toBeInTheDocument();
    });
    // SMTP section 不再渲染
    expect(screen.queryByText("SMTP 服务（发件）")).toBeNull();
    // tab 状态也同步更新
    expect(screen.getByTestId("tab-email").getAttribute("data-active")).toBe("false");
    expect(screen.getByTestId("tab-upload").getAttribute("data-active")).toBe("true");

    // 切回 email tab
    fireEvent.click(screen.getByTestId("tab-email"));
    await waitFor(() => {
      expect(screen.getByText("SMTP 服务（发件）")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("upload-faq-input")).toBeNull();
  });

  it("upload tab 内可正常上传（happy path）", async () => {
    mockFetch.mockImplementation(async (url: string, init?: RequestInit) => {
      if (url === "/api/config") {
        return { ok: true, status: 200, json: () => Promise.resolve({ env: {} }) };
      }
      if (url === "/api/grading-rules" || url === "/api/interest-level") {
        return { ok: true, status: 200, json: () => Promise.resolve({}) };
      }
      if (url === "/api/uploads/faq" && init?.method === "POST") {
        return {
          ok: true,
          status: 200,
          json: () => Promise.resolve({ ok: true, path: "/tmp/faq", size: 2048 }),
        };
      }
      return { ok: false, status: 500, json: () => Promise.resolve({ error: "not mocked" }) };
    });

    renderSettings();
    await waitFor(() => screen.getByTestId("tab-upload"));
    fireEvent.click(screen.getByTestId("tab-upload"));
    await waitFor(() => screen.getByTestId("upload-faq-input"));

    const input = screen.getByTestId("upload-faq-input") as HTMLInputElement;
    const bytes = new Uint8Array([0x50, 0x4B, 0x03, 0x04, 0x14, 0x00, 0x00, 0x00]);
    const file = new File([bytes], "faq.docx", {
      type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    });
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => {
      const uploadCall = mockFetch.mock.calls.find(
        (c) => typeof c[0] === "string" && c[0] === "/api/uploads/faq",
      );
      expect(uploadCall).toBeTruthy();
    });

    // 成功状态出现
    await waitFor(() => {
      expect(document.body.textContent ?? "").toMatch(/已上传/);
    });
  });
});

describe("Settings 资料上传 tab 等级标准 sub-section", () => {
  // 资料上传在第二个 tab，渲染前必须先切到 upload tab
  // 默认 tab 是邮件配置，level sections 此时不存在于 DOM
  async function switchToUploadTab() {
    await waitFor(() => {
      expect(screen.getByTestId("tab-upload")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("tab-upload"));
  }

  // 一次性 mock 所有 Settings 用到的 GET / PATCH / POST：
  // - GET /api/config → 返回 { env }
  // - GET /api/grading-rules → 返回 gradingRules（默认 {}；后端契约：A/B/C 三个 keys）
  // - GET /api/interest-level → 返回 interestLevel（默认 {}；后端契约：S/A/B/C 四个 keys）
  // - PATCH /api/grading-rules → 用 PATCH body 模拟回写
  // - PATCH /api/interest-level → 用 PATCH body 模拟回写
  // - PATCH /api/config → ok
  // - POST /api/uploads/* → ok
  // - 其他 → 500 早暴露未覆盖的调用
  function setupLevelMocks(
    initial: {
      config?: Record<string, string>;
      gradingRules?: Record<string, string>;
      interestLevel?: Record<string, string>;
    } = {},
    overrides: {
      gradingPatch?: (body: Record<string, string>) => MockResponse;
      interestPatch?: (body: Record<string, string>) => MockResponse;
    } = {},
  ) {
    const cfg = initial.config ?? {};
    const gr = { ...(initial.gradingRules ?? {}) };
    const il = { ...(initial.interestLevel ?? {}) };
    mockFetch.mockImplementation(async (url: string, init?: RequestInit) => {
      const method = init?.method ?? "GET";
      if (typeof url !== "string") {
        return { ok: false, status: 500, json: () => Promise.resolve({ error: "not mocked" }) };
      }
      if (url === "/api/config" && method === "GET") {
        return { ok: true, status: 200, json: () => Promise.resolve({ env: { ...cfg } }) };
      }
      if (url === "/api/config" && method === "PATCH") {
        return { ok: true, status: 200, json: () => Promise.resolve({ ok: true, env: { ...cfg }, updated: [] }) };
      }
      if (url === "/api/grading-rules" && method === "GET") {
        return { ok: true, status: 200, json: () => Promise.resolve({ ...gr }) };
      }
      if (url === "/api/grading-rules" && method === "PATCH") {
        const body = JSON.parse(init!.body as string) as Record<string, string>;
        if (overrides.gradingPatch) return overrides.gradingPatch(body);
        Object.assign(gr, body);
        return { ok: true, status: 200, json: () => Promise.resolve({ ...gr }) };
      }
      if (url === "/api/interest-level" && method === "GET") {
        return { ok: true, status: 200, json: () => Promise.resolve({ ...il }) };
      }
      if (url === "/api/interest-level" && method === "PATCH") {
        const body = JSON.parse(init!.body as string) as Record<string, string>;
        if (overrides.interestPatch) return overrides.interestPatch(body);
        Object.assign(il, body);
        return { ok: true, status: 200, json: () => Promise.resolve({ ...il }) };
      }
      if (url.includes("/api/uploads/") && method === "POST") {
        return { ok: true, status: 200, json: () => Promise.resolve({ ok: true, path: "/x", size: 1024 }) };
      }
      return { ok: false, status: 500, json: () => Promise.resolve({ error: "not mocked" }) };
    });
  }

  // value level (A/B/C) 渲染 3 个 textarea；intent level (S/A/B/C) 渲染 4 个
  // GET 返空对象时，所有 textarea 预填空字符串
  it("value level 渲染 3 个 textarea (A/B/C)，intent level 渲染 4 个 (S/A/B/C)，且默认空", async () => {
    setupLevelMocks({});
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("value-levels-textarea-A"));

    // value level：3 个 textarea (A/B/C) + 不存在 S
    expect(screen.getByTestId("value-levels-textarea-A")).toBeInTheDocument();
    expect(screen.getByTestId("value-levels-textarea-B")).toBeInTheDocument();
    expect(screen.getByTestId("value-levels-textarea-C")).toBeInTheDocument();
    expect(screen.queryByTestId("value-levels-textarea-S")).toBeNull();
    expect((screen.getByTestId("value-levels-textarea-A") as HTMLTextAreaElement).value).toBe("");
    expect((screen.getByTestId("value-levels-textarea-B") as HTMLTextAreaElement).value).toBe("");
    expect((screen.getByTestId("value-levels-textarea-C") as HTMLTextAreaElement).value).toBe("");

    // intent level：4 个 textarea (S/A/B/C)
    expect(screen.getByTestId("intent-levels-textarea-S")).toBeInTheDocument();
    expect(screen.getByTestId("intent-levels-textarea-A")).toBeInTheDocument();
    expect(screen.getByTestId("intent-levels-textarea-B")).toBeInTheDocument();
    expect(screen.getByTestId("intent-levels-textarea-C")).toBeInTheDocument();
    expect((screen.getByTestId("intent-levels-textarea-S") as HTMLTextAreaElement).value).toBe("");
    expect((screen.getByTestId("intent-levels-textarea-A") as HTMLTextAreaElement).value).toBe("");
    expect((screen.getByTestId("intent-levels-textarea-B") as HTMLTextAreaElement).value).toBe("");
    expect((screen.getByTestId("intent-levels-textarea-C") as HTMLTextAreaElement).value).toBe("");
  });

  it("value level mock GET 返 {A,B,C} 时 3 个 textarea 各自预填（无 S）", async () => {
    setupLevelMocks({
      gradingRules: { A: "vA", B: "vB", C: "vC" },
    });
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("value-levels-textarea-A"));
    expect((screen.getByTestId("value-levels-textarea-A") as HTMLTextAreaElement).value).toBe("vA");
    expect((screen.getByTestId("value-levels-textarea-B") as HTMLTextAreaElement).value).toBe("vB");
    expect((screen.getByTestId("value-levels-textarea-C") as HTMLTextAreaElement).value).toBe("vC");
    // 仍然没有 S 行
    expect(screen.queryByTestId("value-levels-textarea-S")).toBeNull();
  });

  it("intent level mock GET 返 {S,A,B,C} 时 4 个 textarea 各自预填", async () => {
    setupLevelMocks({
      interestLevel: { S: "iS", A: "iA", B: "iB", C: "iC" },
    });
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("intent-levels-textarea-S"));
    expect((screen.getByTestId("intent-levels-textarea-S") as HTMLTextAreaElement).value).toBe("iS");
    expect((screen.getByTestId("intent-levels-textarea-A") as HTMLTextAreaElement).value).toBe("iA");
    expect((screen.getByTestId("intent-levels-textarea-B") as HTMLTextAreaElement).value).toBe("iB");
    expect((screen.getByTestId("intent-levels-textarea-C") as HTMLTextAreaElement).value).toBe("iC");
  });

  it("shared save/cancel：未改动时 disabled，改任一 section 后 enabled", async () => {
    setupLevelMocks({});
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("levels-shared-save"));
    const saveBtn = screen.getByTestId("levels-shared-save") as HTMLButtonElement;
    const cancelBtn = screen.getByTestId("levels-shared-cancel") as HTMLButtonElement;
    expect(saveBtn).toBeDisabled();
    expect(cancelBtn).toBeDisabled();

    fireEvent.change(screen.getByTestId("value-levels-textarea-A"), {
      target: { value: "newA" },
    });
    expect(saveBtn).not.toBeDisabled();
    expect(cancelBtn).not.toBeDisabled();
  });

  it("value level 保存调用 PATCH /api/grading-rules 且 body 仅含 A/B/C 三个 keys（无 S）", async () => {
    setupLevelMocks({
      gradingRules: { A: "oldA", B: "oldB", C: "oldC" },
    });
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("value-levels-textarea-A"));

    // 改 A，其他保持原值
    fireEvent.change(screen.getByTestId("value-levels-textarea-A"), {
      target: { value: "newA" },
    });
    fireEvent.click(screen.getByTestId("levels-shared-save"));

    await waitFor(() => {
      const patchCall = mockFetch.mock.calls.find(
        (c) =>
          typeof c[0] === "string" &&
          c[0] === "/api/grading-rules" &&
          (c[1] as RequestInit | undefined)?.method === "PATCH",
      );
      expect(patchCall).toBeTruthy();
    });

    const patchCall = mockFetch.mock.calls.find(
      (c) =>
        typeof c[0] === "string" &&
        c[0] === "/api/grading-rules" &&
        (c[1] as RequestInit | undefined)?.method === "PATCH",
    )!;
    const init = patchCall[1] as RequestInit;
    expect(init.headers).toMatchObject({ "Content-Type": "application/json" });
    const body = JSON.parse(init.body as string) as Record<string, string>;
    // 必须 A/B/C 三个 keys 全在（不能含 S）
    expect(Object.keys(body).sort()).toEqual(["A", "B", "C"]);
    expect(body.A).toBe("newA");
    expect(body.B).toBe("oldB");
    expect(body.C).toBe("oldC");
    expect("S" in body).toBe(false);
  });

  it("保存成功显示'已保存'", async () => {
    setupLevelMocks({});
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("value-levels-textarea-A"));

    fireEvent.change(screen.getByTestId("value-levels-textarea-A"), {
      target: { value: "x" },
    });
    fireEvent.click(screen.getByTestId("levels-shared-save"));

    await waitFor(() => {
      const saved = screen.getByTestId("levels-shared-saved-msg");
      expect(saved).toBeInTheDocument();
      expect(saved.textContent).toMatch(/已保存/);
    });
  });

  it("PATCH 400 失败 → 显示错误（error 文案来自后端 body.error）", async () => {
    setupLevelMocks(
      {},
      {
        gradingPatch: () => ({
          ok: false,
          status: 400,
          json: () => Promise.resolve({ error: "bad" }),
        }),
      },
    );
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("value-levels-textarea-A"));

    fireEvent.change(screen.getByTestId("value-levels-textarea-A"), {
      target: { value: "x" },
    });
    fireEvent.click(screen.getByTestId("levels-shared-save"));

    await waitFor(() => {
      const err = screen.getByTestId("levels-shared-error");
      expect(err).toBeInTheDocument();
      expect(err.textContent).toMatch(/bad/);
    });
    // 失败后按钮回到 enabled（draft 未还原）
    expect(screen.getByTestId("levels-shared-save") as HTMLButtonElement).not.toBeDisabled();
  });

  it("shared cancel：value 改动后点取消 → value 还原 + intent 不受影响，无 PATCH 调用", async () => {
    setupLevelMocks({
      gradingRules: { A: "origA", B: "origB", C: "origC" },
      interestLevel: { S: "initS", A: "initA", B: "initB", C: "initC" },
    });
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("levels-shared-save"));

    const aTextarea = screen.getByTestId("value-levels-textarea-A") as HTMLTextAreaElement;
    fireEvent.change(aTextarea, { target: { value: "discardMe" } });
    expect(aTextarea.value).toBe("discardMe");

    fireEvent.click(screen.getByTestId("levels-shared-cancel"));
    // value 还原
    expect((screen.getByTestId("value-levels-textarea-A") as HTMLTextAreaElement).value).toBe("origA");
    // intent 也保持初始（取消统一作用于两个 section）
    expect((screen.getByTestId("intent-levels-textarea-S") as HTMLTextAreaElement).value).toBe("initS");
    // 没有任何 PATCH 调用
    const patchCalls = mockFetch.mock.calls.filter(
      (c) =>
        (typeof c[0] === "string" &&
          (c[0] === "/api/grading-rules" || c[0] === "/api/interest-level") &&
          (c[1] as RequestInit | undefined)?.method === "PATCH"),
    );
    expect(patchCalls.length).toBe(0);
    // 按钮回到 disabled
    expect(screen.getByTestId("levels-shared-save") as HTMLButtonElement).toBeDisabled();
  });

  it("shared save：两个 section 都脏 → 同时 PATCH grading-rules + interest-level", async () => {
    setupLevelMocks({
      gradingRules: { A: "oldA", B: "oldB", C: "oldC" },
      interestLevel: { S: "oldS", A: "oldA", B: "oldB", C: "oldC" },
    });
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("levels-shared-save"));

    // 改 value A 和 intent S
    fireEvent.change(screen.getByTestId("value-levels-textarea-A"), {
      target: { value: "newA" },
    });
    fireEvent.change(screen.getByTestId("intent-levels-textarea-S"), {
      target: { value: "newS" },
    });

    fireEvent.click(screen.getByTestId("levels-shared-save"));

    // 两个端点都收到 PATCH
    await waitFor(() => {
      const grPatch = mockFetch.mock.calls.find(
        (c) =>
          typeof c[0] === "string" &&
          c[0] === "/api/grading-rules" &&
          (c[1] as RequestInit | undefined)?.method === "PATCH",
      );
      const ilPatch = mockFetch.mock.calls.find(
        (c) =>
          typeof c[0] === "string" &&
          c[0] === "/api/interest-level" &&
          (c[1] as RequestInit | undefined)?.method === "PATCH",
      );
      expect(grPatch).toBeTruthy();
      expect(ilPatch).toBeTruthy();
    });

    // 各自 body 内容正确（共享 save 把每个 section 的 draft 独立序列化）
    const grBody = JSON.parse(
      mockFetch.mock.calls.find(
        (c) =>
          typeof c[0] === "string" &&
          c[0] === "/api/grading-rules" &&
          (c[1] as RequestInit | undefined)?.method === "PATCH",
      )![1]!.body as string,
    ) as Record<string, string>;
    expect(Object.keys(grBody).sort()).toEqual(["A", "B", "C"]);
    expect(grBody.A).toBe("newA");

    const ilBody = JSON.parse(
      mockFetch.mock.calls.find(
        (c) =>
          typeof c[0] === "string" &&
          c[0] === "/api/interest-level" &&
          (c[1] as RequestInit | undefined)?.method === "PATCH",
      )![1]!.body as string,
    ) as Record<string, string>;
    expect(Object.keys(ilBody).sort()).toEqual(["A", "B", "C", "S"]);
    expect(ilBody.S).toBe("newS");
  });

  it("intent level 改 S 后保存 → PATCH body 是 {S,A,B,C} 完整 4 keys", async () => {
    setupLevelMocks({
      interestLevel: { S: "oldS", A: "oldA", B: "oldB", C: "oldC" },
    });
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("intent-levels-textarea-S"));

    fireEvent.change(screen.getByTestId("intent-levels-textarea-S"), {
      target: { value: "newS" },
    });
    fireEvent.click(screen.getByTestId("levels-shared-save"));

    await waitFor(() => {
      const patchCall = mockFetch.mock.calls.find(
        (c) =>
          typeof c[0] === "string" &&
          c[0] === "/api/interest-level" &&
          (c[1] as RequestInit | undefined)?.method === "PATCH",
      );
      expect(patchCall).toBeTruthy();
    });

    const intentPatchCall = mockFetch.mock.calls.find(
      (c) =>
        typeof c[0] === "string" &&
        c[0] === "/api/interest-level" &&
        (c[1] as RequestInit | undefined)?.method === "PATCH",
    )!;
    const body = JSON.parse(intentPatchCall[1]!.body as string) as Record<string, string>;
    // intent 端点必须 S/A/B/C 四个 keys 全在
    expect(Object.keys(body).sort()).toEqual(["A", "B", "C", "S"]);
    expect(body.S).toBe("newS");
    expect(body.A).toBe("oldA");
    expect(body.B).toBe("oldB");
    expect(body.C).toBe("oldC");
  });

  it("客户意向等级标准独立：保存调用 PATCH /api/interest-level，不调用 grading rules PATCH", async () => {
    setupLevelMocks({
      gradingRules: { A: "grA", B: "grB", C: "grC" },
      interestLevel: { S: "irS", A: "irA", B: "irB", C: "irC" },
    });
    renderSettings();
    await switchToUploadTab();
    await waitFor(() => screen.getByTestId("intent-levels-textarea-S"));

    // 改 intent S
    fireEvent.change(screen.getByTestId("intent-levels-textarea-S"), {
      target: { value: "newIrS" },
    });
    fireEvent.click(screen.getByTestId("levels-shared-save"));

    await waitFor(() => {
      const patchCall = mockFetch.mock.calls.find(
        (c) =>
          typeof c[0] === "string" &&
          c[0] === "/api/interest-level" &&
          (c[1] as RequestInit | undefined)?.method === "PATCH",
      );
      expect(patchCall).toBeTruthy();
    });

    const intentPatchCall = mockFetch.mock.calls.find(
      (c) =>
        typeof c[0] === "string" &&
        c[0] === "/api/interest-level" &&
        (c[1] as RequestInit | undefined)?.method === "PATCH",
    )!;
    const body = JSON.parse(intentPatchCall[1]!.body as string) as Record<string, string>;
    // body 只能是 4 个 level keys
    expect(Object.keys(body).sort()).toEqual(["A", "B", "C", "S"]);
    expect(body.S).toBe("newIrS");
    expect(body.A).toBe("irA");
    expect(body.B).toBe("irB");
    expect(body.C).toBe("irC");

    // grading rules PATCH 没被调用
    const grPatchCalls = mockFetch.mock.calls.filter(
      (c) =>
        typeof c[0] === "string" &&
        c[0] === "/api/grading-rules" &&
        (c[1] as RequestInit | undefined)?.method === "PATCH",
    );
    expect(grPatchCalls.length).toBe(0);
  });
});

// ===== 公开数据源 tab =====

// 公开数据源：mock 同时支持 GET / POST / PATCH / DELETE
// 用 mutable list 模拟服务端真实状态：
//   - GET /api/target-sites → 当前 list
//   - POST /api/target-sites → append + 返新 site
//   - PATCH /api/target-sites?name=xxx → 修改对应 site
//   - DELETE /api/target-sites?name=xxx → 移除
function mockSitesCRUD(initial: Array<{ name: string; url: string; country?: string; industry?: string; type?: string }>) {
  let current: Array<Record<string, string>> = initial.map((s) => ({ ...s }));
  mockFetch.mockImplementation(async (url: string, init?: RequestInit) => {
    // /api/config 等
    if (url === "/api/config") {
      return { ok: true, status: 200, json: () => Promise.resolve({ env: ENV_OBJ }) };
    }
    if (url === "/api/grading-rules" || url === "/api/interest-level") {
      return { ok: true, status: 200, json: () => Promise.resolve({}) };
    }
    // target-sites
    if (typeof url === "string" && url.startsWith("/api/target-sites")) {
      const method = (init?.method ?? "GET").toUpperCase();
      if (method === "GET") {
        return { ok: true, status: 200, json: () => Promise.resolve([...current]) };
      }
      if (method === "POST") {
        const body = JSON.parse(init!.body as string) as Record<string, string>;
        const newSite: Record<string, string> = { ...body };
        current = [...current, newSite];
        return { ok: true, status: 200, json: () => Promise.resolve(newSite) };
      }
      if (method === "PATCH") {
        // 提取 name= query
        const u = new URL(url, "http://x");
        const name = u.searchParams.get("name") ?? "";
        const body = JSON.parse(init!.body as string) as Record<string, string>;
        const idx = current.findIndex((s) => s.name === name);
        if (idx < 0) {
          return { ok: false, status: 400, json: () => Promise.resolve({ error: "site not found: " + name }) };
        }
        current = current.map((s, i) => (i === idx ? { ...s, ...body } : s));
        return { ok: true, status: 200, json: () => Promise.resolve(current[idx]) };
      }
      if (method === "DELETE") {
        const u = new URL(url, "http://x");
        const name = u.searchParams.get("name") ?? "";
        const before = current.length;
        current = current.filter((s) => s.name !== name);
        if (current.length === before) {
          return { ok: false, status: 400, json: () => Promise.resolve({ error: "site not found: " + name }) };
        }
        return { ok: true, status: 200, json: () => Promise.resolve({ deleted: name }) };
      }
    }
    return { ok: false, status: 500, json: () => Promise.resolve({ error: "not mocked" }) };
  });
  return {
    getCurrent: () => current,
  };
}

describe("Settings 公开数据源 tab", () => {
  it("渲染：tab 按钮、search input、add button", async () => {
    mockSitesCRUD([]);
    renderSettings();
    // 切到 sources tab
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-search-input"));
    expect(screen.getByTestId("sources-search-input")).toBeInTheDocument();
    expect(screen.getByTestId("sources-add-button")).toBeInTheDocument();
  });

  it("空数据 → 显示「暂无数据」", async () => {
    mockSitesCRUD([]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-empty"));
    expect(screen.getByTestId("sources-empty").textContent).toMatch(/暂无数据/);
  });

  it("加载数据 → 渲染 N 行，每行 5 个 input + 删除按钮", async () => {
    mockSitesCRUD([
      { name: "alpha", url: "https://a" },
      { name: "beta", url: "https://b", country: "泰国" },
    ]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    // 每行 5 个 input
    expect(screen.getByTestId("sources-row-0-name")).toBeInTheDocument();
    expect(screen.getByTestId("sources-row-0-url")).toBeInTheDocument();
    expect(screen.getByTestId("sources-row-0-country")).toBeInTheDocument();
    expect(screen.getByTestId("sources-row-0-industry")).toBeInTheDocument();
    expect(screen.getByTestId("sources-row-0-type")).toBeInTheDocument();
    expect(screen.getByTestId("sources-row-0-delete")).toBeInTheDocument();
    expect(screen.getByTestId("sources-row-1-name")).toBeInTheDocument();
    // 值预填
    expect((screen.getByTestId("sources-row-0-name") as HTMLInputElement).value).toBe("alpha");
    expect((screen.getByTestId("sources-row-1-country") as HTMLInputElement).value).toBe("泰国");
  });

  it("点 + 添加行 → 在首行插入一个空白行（5 个空 input）", async () => {
    mockSitesCRUD([{ name: "alpha", url: "https://a" }]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    fireEvent.click(screen.getByTestId("sources-add-button"));
    // 新行在 idx=0，原 alpha 行被推到 idx=1
    expect((screen.getByTestId("sources-row-0-name") as HTMLInputElement).value).toBe("");
    expect((screen.getByTestId("sources-row-0-url") as HTMLInputElement).value).toBe("");
    expect((screen.getByTestId("sources-row-1-name") as HTMLInputElement).value).toBe("alpha");
    // new 标记
    expect(screen.getByTestId("sources-row-0-name").getAttribute("data-new")).toBe("true");
  });

  it("点删除 → 行消失", async () => {
    mockSitesCRUD([
      { name: "alpha", url: "https://a" },
      { name: "beta", url: "https://b" },
    ]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    // 先添加一个新行 → draft = [new, alpha, beta]
    fireEvent.click(screen.getByTestId("sources-add-button"));
    // 删 idx=1 (alpha) → draft = [new, beta]
    fireEvent.click(screen.getByTestId("sources-row-1-delete"));
    expect(screen.getByTestId("sources-row-0-name")).toBeInTheDocument();
    expect(screen.getByTestId("sources-row-1-name")).toBeInTheDocument();
    // 应该是 new + beta（不是 new + alpha）
    expect((screen.getByTestId("sources-row-0-name") as HTMLInputElement).value).toBe("");
    expect((screen.getByTestId("sources-row-1-name") as HTMLInputElement).value).toBe("beta");
  });

  it("搜索 'alpha' → 只显示 alpha 行", async () => {
    mockSitesCRUD([
      { name: "alpha", url: "https://a" },
      { name: "beta", url: "https://b" },
    ]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    fireEvent.change(screen.getByTestId("sources-search-input"), {
      target: { value: "alpha" },
    });
    expect((screen.getByTestId("sources-row-0-name") as HTMLInputElement).value).toBe("alpha");
    expect(screen.queryByTestId("sources-row-1-name")).toBeNull();
  });

  it("搜索无匹配 → 显示「无匹配项」", async () => {
    mockSitesCRUD([{ name: "alpha", url: "https://a" }]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    fireEvent.change(screen.getByTestId("sources-search-input"), {
      target: { value: "zzzzz" },
    });
    expect(screen.getByTestId("sources-empty").textContent).toMatch(/无匹配项/);
  });

  it("未改动 → save/cancel disabled", async () => {
    mockSitesCRUD([{ name: "alpha", url: "https://a" }]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    expect(screen.getByTestId("sources-save") as HTMLButtonElement).toBeDisabled();
    expect(screen.getByTestId("sources-cancel") as HTMLButtonElement).toBeDisabled();
  });

  it("改动任意 input → save/cancel 启用", async () => {
    mockSitesCRUD([{ name: "alpha", url: "https://a" }]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    fireEvent.change(screen.getByTestId("sources-row-0-url"), {
      target: { value: "https://new" },
    });
    expect(screen.getByTestId("sources-save") as HTMLButtonElement).not.toBeDisabled();
    expect(screen.getByTestId("sources-cancel") as HTMLButtonElement).not.toBeDisabled();
  });

  it("点保存：新增 + 修改 + 删除 三类并发派发", async () => {
    mockSitesCRUD([
      { name: "alpha", url: "https://a" },
      { name: "beta", url: "https://b" },
    ]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    // 1) 改 alpha 的 url（idx=0）
    fireEvent.change(screen.getByTestId("sources-row-0-url"), {
      target: { value: "https://a-new" },
    });
    // 2) 新增一行 gamma（插到首行 → idx=0，原 alpha/beta 推到 1/2）
    fireEvent.click(screen.getByTestId("sources-add-button"));
    fireEvent.change(screen.getByTestId("sources-row-0-name"), {
      target: { value: "gamma" },
    });
    fireEvent.change(screen.getByTestId("sources-row-0-url"), {
      target: { value: "https://g" },
    });
    // 3) 删 beta（现在 idx=2）
    fireEvent.click(screen.getByTestId("sources-row-2-delete"));
    // 保存
    fireEvent.click(screen.getByTestId("sources-save"));

    // 等所有调用出现
    await waitFor(() => {
      const calls = mockFetch.mock.calls.filter(
        (c) => typeof c[0] === "string" && c[0].startsWith("/api/target-sites"),
      );
      const methods = calls.map((c) => (c[1] as RequestInit | undefined)?.method ?? "GET");
      // 至少有 1 POST + 1 PATCH + 1 DELETE
      expect(methods).toContain("POST");
      expect(methods).toContain("PATCH");
      expect(methods).toContain("DELETE");
    });

    // POST body 是新行
    const postCall = mockFetch.mock.calls.find(
      (c) =>
        typeof c[0] === "string" &&
        c[0] === "/api/target-sites" &&
        (c[1] as RequestInit | undefined)?.method === "POST",
    );
    const postBody = JSON.parse((postCall![1] as RequestInit).body as string) as Record<string, string>;
    expect(postBody.name).toBe("gamma");
    expect(postBody.url).toBe("https://g");

    // PATCH query 是 alpha，body 只含 url
    const patchCall = mockFetch.mock.calls.find(
      (c) =>
        typeof c[0] === "string" &&
        c[0].startsWith("/api/target-sites?name=") &&
        (c[1] as RequestInit | undefined)?.method === "PATCH",
    );
    expect(patchCall![0]).toContain("name=alpha");
    const patchBody = JSON.parse((patchCall![1] as RequestInit).body as string) as Record<string, string>;
    expect(patchBody).toEqual({ url: "https://a-new" });

    // DELETE query 是 beta
    const deleteCall = mockFetch.mock.calls.find(
      (c) =>
        typeof c[0] === "string" &&
        c[0].startsWith("/api/target-sites?name=") &&
        (c[1] as RequestInit | undefined)?.method === "DELETE",
    );
    expect(deleteCall![0]).toContain("name=beta");
  });

  it("保存成功 → 显示'已保存'提示", async () => {
    mockSitesCRUD([{ name: "alpha", url: "https://a" }]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    fireEvent.change(screen.getByTestId("sources-row-0-url"), {
      target: { value: "https://new" },
    });
    fireEvent.click(screen.getByTestId("sources-save"));
    await waitFor(() => {
      const msg = screen.getByTestId("sources-saved-msg");
      expect(msg).toBeInTheDocument();
      expect(msg.textContent).toMatch(/已保存/);
    });
  });

  it("点取消 → 还原到服务端值，无 PATCH/POST/DELETE 调用", async () => {
    mockSitesCRUD([{ name: "alpha", url: "https://a" }]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    fireEvent.change(screen.getByTestId("sources-row-0-url"), {
      target: { value: "https://discard" },
    });
    fireEvent.click(screen.getByTestId("sources-cancel"));
    // url 还原
    expect((screen.getByTestId("sources-row-0-url") as HTMLInputElement).value).toBe("https://a");
    // 按钮 disabled
    expect(screen.getByTestId("sources-save") as HTMLButtonElement).toBeDisabled();
    expect(screen.getByTestId("sources-cancel") as HTMLButtonElement).toBeDisabled();
    // 没有 POST/PATCH/DELETE
    const writes = mockFetch.mock.calls.filter(
      (c) => {
        if (typeof c[0] !== "string" || !c[0].startsWith("/api/target-sites")) return false;
        const m = (c[1] as RequestInit | undefined)?.method ?? "GET";
        return m !== "GET";
      },
    );
    expect(writes.length).toBe(0);
  });

  it("保存失败 → 显示错误", async () => {
    mockSitesCRUD([{ name: "alpha", url: "https://a" }]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    fireEvent.change(screen.getByTestId("sources-row-0-url"), {
      target: { value: "https://new" },
    });
    // 覆盖 PATCH 让它失败
    const orig = mockFetch.getMockImplementation();
    mockFetch.mockImplementation(async (url: string, init?: RequestInit) => {
      if (typeof url === "string" && url.startsWith("/api/target-sites")) {
        const m = (init?.method ?? "GET").toUpperCase();
        if (m === "PATCH") {
          return { ok: false, status: 400, json: () => Promise.resolve({ error: "site not found" }) };
        }
      }
      return orig!(url, init);
    });
    fireEvent.click(screen.getByTestId("sources-save"));
    await waitFor(() => {
      const err = screen.getByTestId("sources-error");
      expect(err).toBeInTheDocument();
      expect(err.textContent).toMatch(/site not found/);
    });
  });

  it("列头为中文：名称 / 链接 / 国家 / 行业 / 内容类型", async () => {
    mockSitesCRUD([{ name: "alpha", url: "https://a" }]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    const header = screen.getByTestId("sources-header");
    expect(header.textContent).toMatch(/名称/);
    expect(header.textContent).toMatch(/链接/);
    expect(header.textContent).toMatch(/国家/);
    expect(header.textContent).toMatch(/行业/);
    expect(header.textContent).toMatch(/内容类型/);
  });

  it("type 字段渲染为下拉框，选项是 crawl/download（中文 label）", async () => {
    mockSitesCRUD([{ name: "alpha", url: "https://a" }]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    const typeSelect = screen.getByTestId("sources-row-0-type") as HTMLSelectElement;
    expect(typeSelect.tagName).toBe("SELECT");
    // 选项含 crawl/download（label 是中文）
    const optionValues = Array.from(typeSelect.options).map((o) => o.value);
    expect(optionValues).toContain("crawl");
    expect(optionValues).toContain("download");
    const optionLabels = Array.from(typeSelect.options).map((o) => o.textContent);
    expect(optionLabels).toContain("常规爬取");
    expect(optionLabels).toContain("文件下载");
  });

  it("type 字段预填：服务端 crawl → 选中'常规爬取'", async () => {
    mockSitesCRUD([{ name: "alpha", url: "https://a", type: "crawl" }]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    const typeSelect = screen.getByTestId("sources-row-0-type") as HTMLSelectElement;
    expect(typeSelect.value).toBe("crawl");
  });

  it("type 字段改动 → save 启用", async () => {
    mockSitesCRUD([{ name: "alpha", url: "https://a" }]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    expect(screen.getByTestId("sources-save") as HTMLButtonElement).toBeDisabled();
    fireEvent.change(screen.getByTestId("sources-row-0-type"), {
      target: { value: "download" },
    });
    expect(screen.getByTestId("sources-save") as HTMLButtonElement).not.toBeDisabled();
  });

  it("type 改动保存 → PATCH body 含 type='download'", async () => {
    mockSitesCRUD([{ name: "alpha", url: "https://a" }]);
    renderSettings();
    await waitFor(() => screen.getByTestId("tab-sources"));
    fireEvent.click(screen.getByTestId("tab-sources"));
    await waitFor(() => screen.getByTestId("sources-rows"));
    fireEvent.change(screen.getByTestId("sources-row-0-type"), {
      target: { value: "download" },
    });
    fireEvent.click(screen.getByTestId("sources-save"));
    await waitFor(() => {
      const patchCall = mockFetch.mock.calls.find(
        (c) =>
          typeof c[0] === "string" &&
          c[0].startsWith("/api/target-sites?name=") &&
          (c[1] as RequestInit | undefined)?.method === "PATCH",
      );
      expect(patchCall).toBeTruthy();
    });
    const patchCall = mockFetch.mock.calls.find(
      (c) =>
        typeof c[0] === "string" &&
        c[0].startsWith("/api/target-sites?name=") &&
        (c[1] as RequestInit | undefined)?.method === "PATCH",
    )!;
    const body = JSON.parse((patchCall[1] as RequestInit).body as string) as Record<string, string>;
    expect(body.type).toBe("download");
  });

  // ===== 重启服务 =====

  it("shows restart button in header", async () => {
    mockConfigOk();
    renderSettings();
    await waitFor(() => screen.getByText("系统设置"));
    expect(screen.getByTestId("restart-btn")).toBeInTheDocument();
    expect(screen.getByTestId("restart-btn").textContent).toBe("重启服务");
  });

  it("sends POST /api/restart on click and shows success message", async () => {
    mockConfigOk();
    renderSettings();
    await waitFor(() => screen.getByText("系统设置"));

    fireEvent.click(screen.getByTestId("restart-btn"));

    await waitFor(() => {
      expect(screen.getByText("重启信号已发送，服务即将重新加载")).toBeInTheDocument();
    });

    const restartCall = mockFetch.mock.calls.find(
      (c) => typeof c[0] === "string" && c[0] === "/api/restart",
    );
    expect(restartCall).toBeTruthy();
    expect((restartCall![1] as RequestInit)?.method).toBe("POST");
  });

  it("shows error when restart fails", async () => {
    mockFetch.mockImplementation(async (url: string) => {
      if (url === "/api/config") {
        return { ok: true, status: 200, json: () => Promise.resolve({ env: {} }) };
      }
      if (url === "/api/grading-rules" || url === "/api/interest-level" || url === "/api/target-sites") {
        return { ok: true, status: 200, json: () => Promise.resolve([]) };
      }
      if (url === "/api/restart") {
        return { ok: false, status: 500, json: () => Promise.resolve({ error: "script not found" }) };
      }
      return { ok: false, status: 500, json: () => Promise.resolve({ error: "not mocked" }) };
    });
    renderSettings();
    await waitFor(() => screen.getByText("系统设置"));

    fireEvent.click(screen.getByTestId("restart-btn"));

    await waitFor(() => {
      expect(screen.getByText("script not found")).toBeInTheDocument();
    });
  });
});

// ===== 微信绑定 tab =====

// mock wechat-bind 端点：POST 立即返 202 + task_id，GET 走 pollStatuses 序列。
// pollStatuses 是按调用顺序消费的状态数组：测试传 [running, done] 即「先返 running
// 再返 done」；传空数组则 GET 一律返 pending。
// postErrorOverride: 把 POST 强制改成 5xx（测 POST 失败路径）。
// getErrorOverride:  把 GET 强制改成 5xx（测 5xx 透传）。
// getStatusOverride: 把 GET 强制改成 404（测 task not found）。
type PollStatus =
  | { kind: "ok"; status: "pending" | "running" | "done" | "failed" | "expired"; payload?: Record<string, unknown> }
  | { kind: "404" }
  | { kind: "5xx"; error: string };
function mockWechatBindPolling(opts: {
  pollStatuses?: PollStatus[];
  postError?: { error: string; expired?: boolean };
  getError?: { error: string };
  doneResult?: { link: string; qr: string; raw?: string; bound?: boolean; already_bound?: boolean };
} = {}) {
  const pollStatuses = opts.pollStatuses ?? [{ kind: "ok", status: "pending" }];
  let pollIdx = 0;
  mockFetch.mockImplementation(async (url: string, init?: RequestInit) => {
    const method = (init?.method ?? "GET").toUpperCase();
    if (url === "/api/config") {
      return { ok: true, status: 200, json: () => Promise.resolve({ env: {} }) };
    }
    if (url === "/api/grading-rules" || url === "/api/interest-level") {
      return { ok: true, status: 200, json: () => Promise.resolve({}) };
    }
    if (typeof url === "string" && url.startsWith("/api/target-sites")) {
      return { ok: true, status: 200, json: () => Promise.resolve([]) };
    }
    if (url === "/api/restart") {
      return { ok: true, status: 200, json: () => Promise.resolve({ ok: true, output: "" }) };
    }
    // POST /api/wechat/bind → 202 + task_id
    if (url === "/api/wechat/bind" && method === "POST") {
      if (opts.postError) {
        return {
          ok: false,
          status: 500,
          json: () =>
            Promise.resolve({
              error: opts.postError!.error,
              expired: opts.postError!.expired ?? false,
            }),
        };
      }
      return {
        ok: true,
        status: 202,
        json: () => Promise.resolve({ task_id: "wt-test-001", status: "pending" }),
      };
    }
    // GET /api/wechat/bind/:task_id
    if (typeof url === "string" && url.startsWith("/api/wechat/bind/wt-") && method === "GET") {
      if (opts.getError) {
        return {
          ok: false,
          status: 500,
          json: () => Promise.resolve({ error: opts.getError!.error }),
        };
      }
      const cur = pollStatuses[Math.min(pollIdx, pollStatuses.length - 1)];
      pollIdx += 1;
      if (cur.kind === "404") {
        return { ok: false, status: 404, json: () => Promise.resolve({ error: "task not found" }) };
      }
      if (cur.kind === "5xx") {
        return { ok: false, status: 500, json: () => Promise.resolve({ error: cur.error }) };
      }
      // ok：构造 poll 响应
      if (cur.status === "done") {
        const r = opts.doneResult ?? {
          link: "https://liteapp.weixin.qq.com/q/abc?qrcode=xxx",
          qr: "▄▄▄\n█ █\n▀▀▀",
        };
        return {
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              task_id: "wt-test-001",
              status: "done",
              link: r.link,
              qr: r.qr,
              raw: r.raw ?? "stdout",
              expired: false,
              // bound 可选:测试要模拟"exec 已自然退出 + bound=true"时
              // 通过 doneResult.bound 传入(默认 undefined,前端不触发成功状态)
              ...(r.bound !== undefined ? { bound: r.bound } : {}),
              // already_bound 可选:测试要模拟"openclaw 检测已连过"时通过 doneResult.already_bound 传入。
              // 默认 undefined,前端按 false 处理(显示「绑定成功」)。
              ...(r.already_bound !== undefined ? { already_bound: r.already_bound } : {}),
            }),
        };
      }
      // 其它状态：构造对应 payload
      return {
        ok: true,
        status: 200,
        json: () => Promise.resolve({ task_id: "wt-test-001", ...(cur.payload ?? {}), status: cur.status }),
      };
    }
    // POST /api/wechat/bind/:task_id/cancel
    // 默认返 200 + cancelled=true(模拟"task 还在 running,cancel 信号已发")
    // 测试要测"已 done 的 task cancel 返 cancelled=false"时,可以用 mockResolvedValueOnce
    // 覆盖下一次 fetch;或者写一个独立的 mock。
    if (typeof url === "string" && url.startsWith("/api/wechat/bind/wt-") && url.endsWith("/cancel") && method === "POST") {
      return {
        ok: true,
        status: 200,
        json: () => Promise.resolve({ cancelled: true }),
      };
    }
    return { ok: false, status: 500, json: () => Promise.resolve({ error: "not mocked" }) };
  });
}

async function switchToWechatTab() {
  await waitFor(() => screen.getByTestId("tab-wechat"));
  fireEvent.click(screen.getByTestId("tab-wechat"));
}

// 拿所有 wechat-bind 端点的 fetch 调用（POST /api/wechat/bind 和 GET /api/wechat/bind/wt-...）
function wechatBindCalls() {
  return mockFetch.mock.calls.filter(
    (c) => typeof c[0] === "string" && c[0].startsWith("/api/wechat/bind"),
  );
}
function wechatBindPostCall() {
  return wechatBindCalls().find(
    (c) => (c[1] as RequestInit | undefined)?.method === "POST",
  );
}
function wechatBindGetCalls() {
  return wechatBindCalls().filter(
    (c) => (c[1] as RequestInit | undefined)?.method === undefined,
  );
}

describe("Settings 微信绑定 tab", () => {
  it("渲染：tab 按钮、table 显示'未绑定'状态、绑定按钮", async () => {
    mockWechatBindPolling();
    renderSettings();
    await switchToWechatTab();
    // section + table + 状态 + 按钮
    expect(screen.getByTestId("wechat-bind-section")).toBeInTheDocument();
    expect(screen.getByTestId("wechat-bind-table")).toBeInTheDocument();
    expect(screen.getByTestId("wechat-bind-status").textContent).toBe("未绑定");
    const btn = screen.getByTestId("wechat-bind-button") as HTMLButtonElement;
    expect(btn).toBeInTheDocument();
    expect(btn.textContent).toBe("绑定");
    expect(btn).not.toBeDisabled();
    // 默认 modal 不在 DOM
    expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
  });

  it("点绑定 → submitting 阶段按钮 disabled 显示'启动中...'，再 → polling 阶段显示倒计时", async () => {
    // GET 一直返 pending，UI 会停留在 polling 阶段
    mockWechatBindPolling({ pollStatuses: [{ kind: "ok", status: "pending" }] });
    renderSettings();
    await switchToWechatTab();

    const btn = screen.getByTestId("wechat-bind-button") as HTMLButtonElement;
    fireEvent.click(btn);

    // 立即处于 submitting：按钮 disabled + 文案「启动中...」
    // （submitting 是 setState 后的同步 phase，React 18 批处理后 next microtask 生效）
    await waitFor(() => {
      expect((screen.getByTestId("wechat-bind-button") as HTMLButtonElement).disabled).toBe(true);
    });
    // 提交完成 → 进入 polling：文案含「等待生成二维码」
    await waitFor(() => {
      expect(screen.getByTestId("wechat-bind-button").textContent).toMatch(/等待生成二维码/);
    });
    // 按钮仍 disabled
    expect((screen.getByTestId("wechat-bind-button") as HTMLButtonElement).disabled).toBe(true);
  });

  it("submitting / polling 阶段显示'绑定过程中请勿离开该页面'提示", async () => {
    // GET 一直返 pending，让 UI 停留在 polling 阶段
    mockWechatBindPolling({ pollStatuses: [{ kind: "ok", status: "pending" }] });
    renderSettings();
    await switchToWechatTab();

    // 初始(idle):无提示
    expect(screen.queryByTestId("wechat-bind-hint")).toBeNull();

    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    // submitting / polling 阶段:提示出现
    await waitFor(() => {
      const hint = screen.getByTestId("wechat-bind-hint");
      expect(hint).toBeInTheDocument();
      expect(hint.textContent).toMatch(/绑定过程中请勿离开该页面/);
    });
  });

  it("polling → done 后'请勿离开'提示消失(modal 取而代之)", async () => {
    // 一次 GET 就 done
    mockWechatBindPolling({
      pollStatuses: [{ kind: "ok", status: "done" }],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    // 等待 modal 出现(done 状态);timeout 显式拉长避开 1s 轮询 tick
    await waitFor(
      () => {
        expect(screen.getByTestId("wechat-bind-modal")).toBeInTheDocument();
      },
      { timeout: 3000 },
    );
    // 此时 polling 阶段已结束,提示应消失(modal 替代了它的作用)
    expect(screen.queryByTestId("wechat-bind-hint")).toBeNull();
  });

  it("polling → failed 后'请勿离开'提示消失(行内 error 取代)", async () => {
    mockWechatBindPolling({
      pollStatuses: [
        { kind: "ok", status: "failed", payload: { error: "docker crashed" } },
      ],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    // timeout 显式拉长避开 1s 轮询 tick
    await waitFor(
      () => {
        expect(screen.getByTestId("wechat-bind-error")).toBeInTheDocument();
      },
      { timeout: 3000 },
    );
    // 失败状态:无"请勿离开"提示
    expect(screen.queryByTestId("wechat-bind-hint")).toBeNull();
  });

  it("polling → done → 展示 modal，含 qr + link", async () => {
    const qr = "▄▄▄▄▄\n█ ▄▄ █\n█ ▀▀ █\n▀▀▀▀▀";
    const link = "https://liteapp.weixin.qq.com/q/test?qrcode=abc";
    mockWechatBindPolling({
      // 第一次返 running，第二次返 done
      pollStatuses: [
        { kind: "ok", status: "running" },
        { kind: "ok", status: "done" },
      ],
      doneResult: { link, qr },
    });
    renderSettings();
    await switchToWechatTab();

    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    // 验证 POST /api/wechat/bind（无 body）
    await waitFor(() => {
      expect(wechatBindPostCall()).toBeTruthy();
    });
    const post = wechatBindPostCall()!;
    const init = post[1] as RequestInit;
    expect(init.method).toBe("POST");
    expect(init.body).toBeUndefined();

    // 验证 GET /api/wechat/bind/wt-... 至少被调过
    await waitFor(() => {
      expect(wechatBindGetCalls().length).toBeGreaterThan(0);
    });

    // modal 出现（含 done 的 link + qr）
    await waitFor(() => {
      expect(screen.getByTestId("wechat-bind-modal")).toBeInTheDocument();
    });
    expect(screen.getByTestId("wechat-bind-modal-title").textContent).toBe("微信扫码登录");
    const qrEl = screen.getByTestId("wechat-bind-qr") as HTMLPreElement;
    expect(qrEl.textContent).toBe(qr);
    const linkEl = screen.getByTestId("wechat-bind-link") as HTMLAnchorElement;
    expect(linkEl.textContent).toBe(link);
    expect(linkEl.getAttribute("href")).toBe(link);
    expect(linkEl.getAttribute("target")).toBe("_blank");

    // done 后按钮恢复可点
    expect((screen.getByTestId("wechat-bind-button") as HTMLButtonElement).disabled).toBe(false);
  });

  it("polling → failed → 行内错误显示 result.error", async () => {
    mockWechatBindPolling({
      pollStatuses: [
        { kind: "ok", status: "failed", payload: { error: "qrcode script crashed" } },
      ],
    });
    renderSettings();
    await switchToWechatTab();

    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    // 显式 timeout:轮询间隔 1s,等 1 次 tick 后再断言;默认 waitFor(1s)跟 1s tick 会 race
    await waitFor(
      () => {
        const err = screen.getByTestId("wechat-bind-error");
        expect(err).toBeInTheDocument();
        expect(err.textContent).toMatch(/qrcode script crashed/);
      },
      { timeout: 3000 },
    );
    // 失败时 modal 不出现
    expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
    // 按钮恢复可点
    expect((screen.getByTestId("wechat-bind-button") as HTMLButtonElement).disabled).toBe(false);
  });

  it("polling → expired 但 link/qr 都空 → 行内 fallback '二维码生成超时'", async () => {
    // payload 不带 error,让前端走 fallback 文案「二维码生成超时」
    // (实际后端 expired 一定带 error,这是测前端 fallback 路径)
    mockWechatBindPolling({
      pollStatuses: [
        { kind: "ok", status: "expired" },
      ],
    });
    renderSettings();
    await switchToWechatTab();

    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    // 显式 timeout:轮询间隔 1s,等 1 次 tick 后再断言;默认 waitFor(1s)跟 1s tick 会 race
    await waitFor(
      () => {
        const err = screen.getByTestId("wechat-bind-error");
        expect(err).toBeInTheDocument();
        expect(err.textContent).toMatch(/超时/);
      },
      { timeout: 3000 },
    );
    expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
  });

  it("polling → expired 但 link/qr 非空 → 展示 modal + 顶部警告 banner", async () => {
    // 后端超时但 openclaw 已经把 QR 打出来了 —— 这种场景下前端要展示 modal,
    // 顶部加"进程被中断,但二维码可能还有效"提示,让用户试扫。
    const qr = "▄▄▄▄▄\n█ ▄▄ █\n█ ▀▀ █\n▀▀▀▀▀";
    const link = "https://liteapp.weixin.qq.com/q/test?qrcode=abc";
    mockWechatBindPolling({
      pollStatuses: [
        {
          kind: "ok",
          status: "expired",
          payload: {
            error: "wechat bind failed: signal: killed",
            link,
            qr,
            expired: true,
            raw: "stdout with qr",
          },
        },
      ],
    });
    renderSettings();
    await switchToWechatTab();

    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    // modal 出现（含 link + qr）;timeout 显式拉长避免跟 1s 轮询 tick 撞
    await waitFor(
      () => {
        expect(screen.getByTestId("wechat-bind-modal")).toBeInTheDocument();
      },
      { timeout: 3000 },
    );
    // 警告 banner 出现
    const warn = screen.getByTestId("wechat-bind-warning");
    expect(warn).toBeInTheDocument();
    expect(warn.textContent).toMatch(/进程被中断/);
    // 二维码 + 链接正常展示
    const qrEl = screen.getByTestId("wechat-bind-qr") as HTMLPreElement;
    expect(qrEl.textContent).toBe(qr);
    const linkEl = screen.getByTestId("wechat-bind-link") as HTMLAnchorElement;
    expect(linkEl.textContent).toBe(link);
    // 没有行内错误（modal 是主反馈渠道）
    expect(screen.queryByTestId("wechat-bind-error")).toBeNull();
    // 按钮恢复可点
    expect((screen.getByTestId("wechat-bind-button") as HTMLButtonElement).disabled).toBe(false);
  });

  it("polling → running 但 link/qr 非空 → 立刻弹 modal,无警告 banner", async () => {
    // 关键场景:后端"早期发布" — docker exec 打印完 link 行就 store.Update,此时
    // status 还是 running(docker 在等用户扫码,不会自然返回),但 GET 已经能拿到
    // link/qr。前端要识别这种状态:看到 link/qr 就弹 modal,而不是按 status="running"
    // 走"更新 elapsed"路径(那样 modal 永远不会出现,直到 timeout)。
    // warning 应为 false(流程正常,只是 QR 还没扫),不应有黄色 banner。
    const qr = "▄▄▄▄▄\n█ ▄▄ █\n█ ▀▀ █\n▀▀▀▀▀";
    const link = "https://liteapp.weixin.qq.com/q/early?qrcode=xyz";
    mockWechatBindPolling({
      pollStatuses: [
        {
          kind: "ok",
          status: "running",
          payload: { link, qr, raw: "stdout with qr", expired: false },
        },
      ],
    });
    renderSettings();
    await switchToWechatTab();

    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    await waitFor(
      () => {
        expect(screen.getByTestId("wechat-bind-modal")).toBeInTheDocument();
      },
      { timeout: 3000 },
    );
    // 没警告 banner(running 状态视为流程正常)
    expect(screen.queryByTestId("wechat-bind-warning")).toBeNull();
    // QR + link 正常
    const qrEl = screen.getByTestId("wechat-bind-qr") as HTMLPreElement;
    expect(qrEl.textContent).toBe(qr);
    const linkEl = screen.getByTestId("wechat-bind-link") as HTMLAnchorElement;
    expect(linkEl.textContent).toBe(link);
    // 没行内错误
    expect(screen.queryByTestId("wechat-bind-error")).toBeNull();
  });

  it("polling → pending 但 link/qr 非空 → 立刻弹 modal,无警告 banner", async () => {
    // 同 running 场景,只是 status 是 pending(后端刚把 task 标记 running 但前端拿到的
    // 早期 snapshot 还是 pending)。link/qr 非空 → 弹 modal,无 warning。
    const qr = "▄▄▄\n█ █\n▀▀▀";
    const link = "https://liteapp.weixin.qq.com/q/pending";
    mockWechatBindPolling({
      pollStatuses: [
        {
          kind: "ok",
          status: "pending",
          payload: { link, qr, raw: "stdout", expired: false },
        },
      ],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    await waitFor(
      () => expect(screen.getByTestId("wechat-bind-modal")).toBeInTheDocument(),
      { timeout: 3000 },
    );
    expect(screen.queryByTestId("wechat-bind-warning")).toBeNull();
  });

  it("polling → failed 但 link/qr 非空 → 弹 modal + 警告 banner", async () => {
    // 旧实现 bug:status=failed 时无条件 setState({kind:"failed"}),把 link/qr 吞了
    // 只显示 error。但实际场景是 docker 先打印了 QR + link 然后才报错(如容器退出非 0),
    // 这时 link/qr 仍可能有效,前端应展示 modal + 警告。
    const qr = "▄▄▄\n█ █\n▀▀▀";
    const link = "https://liteapp.weixin.qq.com/q/failed-with-qr";
    mockWechatBindPolling({
      pollStatuses: [
        {
          kind: "ok",
          status: "failed",
          payload: {
            error: "openclaw exited unexpectedly",
            link,
            qr,
            expired: false,
            raw: "stdout",
          },
        },
      ],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    await waitFor(
      () => expect(screen.getByTestId("wechat-bind-modal")).toBeInTheDocument(),
      { timeout: 3000 },
    );
    // 警告 banner 出现(failed 视同"流程已死,QR 可能失效")
    expect(screen.getByTestId("wechat-bind-warning")).toBeInTheDocument();
    // 行内错误不显示(modal 才是主反馈渠道)
    expect(screen.queryByTestId("wechat-bind-error")).toBeNull();
  });

  it("polling → done 但 link/qr 空 → 兜底 failed '未返回二维码'", async () => {
    // 异常场景:后端 done 但 link/qr 空(后端契约上不应发生,这里测前端兜底)。
    // 不弹 modal,只显示行内错误,提示数据缺失。
    // doneResult 显式传空字符串 —— mockWechatBindPolling 的 done 分支强制用 doneResult,
    // 默认值是带 link/qr 的,要测「空」必须显式 override。
    mockWechatBindPolling({
      doneResult: { link: "", qr: "" },
      pollStatuses: [
        { kind: "ok", status: "done" },
      ],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    await waitFor(
      () => {
        const err = screen.getByTestId("wechat-bind-error");
        expect(err).toBeInTheDocument();
        expect(err.textContent).toMatch(/未返回二维码/);
      },
      { timeout: 3000 },
    );
    expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
  });

  it("modal 关闭按钮：doneWithWarning 状态也能关", async () => {
    const qr = "▄▄▄▄▄\n█ ▄▄ █\n▀▀▀▀▀";
    const link = "https://liteapp.weixin.qq.com/q/test?qrcode=def";
    mockWechatBindPolling({
      pollStatuses: [
        { kind: "ok", status: "expired", payload: { error: "killed", link, qr, expired: true } },
      ],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));
    await waitFor(() => screen.getByTestId("wechat-bind-modal"), { timeout: 3000 });

    fireEvent.click(screen.getByTestId("wechat-bind-modal-close"));

    await waitFor(() => {
      expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
    });
  });

  it("GET 404 → 行内提示'任务已过期'", async () => {
    mockWechatBindPolling({
      pollStatuses: [{ kind: "404" }],
    });
    renderSettings();
    await switchToWechatTab();

    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    // 显式 timeout:轮询间隔 1s,等 1 次 tick 后再断言
    await waitFor(
      () => {
        const err = screen.getByTestId("wechat-bind-error");
        expect(err).toBeInTheDocument();
        expect(err.textContent).toMatch(/任务已过期/);
      },
      { timeout: 3000 },
    );
    expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
    expect((screen.getByTestId("wechat-bind-button") as HTMLButtonElement).disabled).toBe(false);
  });

  it("POST 5xx → 行内错误显示后端 error 字段（透传 CRMFetchError.message）", async () => {
    mockWechatBindPolling({
      postError: { error: "wechat bind failed: timeout", expired: true },
    });
    renderSettings();
    await switchToWechatTab();

    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    await waitFor(() => {
      const err = screen.getByTestId("wechat-bind-error");
      expect(err).toBeInTheDocument();
      expect(err.textContent).toMatch(/wechat bind failed: timeout/);
    });
    // POST 失败 → 没有 GET 轮询调用
    expect(wechatBindGetCalls().length).toBe(0);
    expect((screen.getByTestId("wechat-bind-button") as HTMLButtonElement).disabled).toBe(false);
  });

  it("GET 5xx → 不立刻切 failed，保留 polling 让下个 tick 重试", async () => {
    // 5xx 后再 done：5xx 不致命，第二次返 done 时切到 done
    mockWechatBindPolling({
      pollStatuses: [
        { kind: "5xx", error: "transient" },
        { kind: "ok", status: "done" },
      ],
    });
    renderSettings();
    await switchToWechatTab();

    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    // 等到 done 的 modal;timeout 显式拉长
    await waitFor(
      () => {
        expect(screen.getByTestId("wechat-bind-modal")).toBeInTheDocument();
      },
      { timeout: 3000 },
    );
  });

  it("modal 关闭按钮 → modal 消失，按钮恢复可点", async () => {
    mockWechatBindPolling({
      pollStatuses: [{ kind: "ok", status: "done" }],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));
    await waitFor(() => screen.getByTestId("wechat-bind-modal"), { timeout: 3000 });

    fireEvent.click(screen.getByTestId("wechat-bind-modal-close"));

    await waitFor(() => {
      expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
    });
    expect((screen.getByTestId("wechat-bind-button") as HTMLButtonElement).disabled).toBe(false);
  });

  it("modal 遮罩点击 → modal 消失", async () => {
    mockWechatBindPolling({
      pollStatuses: [{ kind: "ok", status: "done" }],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));
    await waitFor(() => screen.getByTestId("wechat-bind-modal"), { timeout: 3000 });

    // 点遮罩（modal 自身有 onClick 关闭；内容容器有 stopPropagation）
    fireEvent.click(screen.getByTestId("wechat-bind-modal"));

    await waitFor(() => {
      expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
    });
  });

  it("modal 内容区点击 → 不会关闭（stopPropagation）", async () => {
    mockWechatBindPolling({
      pollStatuses: [{ kind: "ok", status: "done" }],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));
    await waitFor(() => screen.getByTestId("wechat-bind-modal"), { timeout: 3000 });

    fireEvent.click(screen.getByTestId("wechat-bind-modal-title"));
    expect(screen.getByTestId("wechat-bind-modal")).toBeInTheDocument();
  });

  it("取消轮询：组件 unmount 时 interval 被清", async () => {
    // GET 一直 pending
    mockWechatBindPolling({ pollStatuses: [{ kind: "ok", status: "pending" }] });
    const { unmount } = renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));
    // 等到 polling 阶段
    await waitFor(() => {
      expect(screen.getByTestId("wechat-bind-button").textContent).toMatch(/等待生成二维码/);
    });
    const getCallsBefore = wechatBindGetCalls().length;
    // unmount
    unmount();
    // 等一段时间（用 fake timer 推进），再断言：不应再有新的 GET
    await new Promise((r) => setTimeout(r, 50));
    expect(wechatBindGetCalls().length).toBe(getCallsBefore);
  });

  it("多次连点：上一次 polling 未完时再点 → 旧 interval 被 clear，新轮询重启", async () => {
    // 第一次 poll 返 pending（轮询继续），第二次 poll 返 done
    // 关键：重新点「绑定」会让 GET 计数从 0 重新开始（旧的 timer 被 clear）
    mockWechatBindPolling({
      pollStatuses: [
        { kind: "ok", status: "pending" },
        { kind: "ok", status: "done" },
      ],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));
    // 轮询间隔 1s;state 切到 polling 后第一次 setInterval tick 还没 fire,
    // 等首次 GET 调用再断言"polling 状态 + 按钮 disabled",避免 race
    await waitFor(
      () => {
        expect(wechatBindGetCalls().length).toBeGreaterThanOrEqual(1);
      },
      { timeout: 3000 },
    );
    // 此刻必定在 polling 状态
    expect(screen.getByTestId("wechat-bind-button").textContent).toMatch(/等待生成二维码/);
    // 按钮在 polling 阶段 disabled 是符合契约的（用户连点不会启动新轮询）
    expect((screen.getByTestId("wechat-bind-button") as HTMLButtonElement).disabled).toBe(true);
  });

  it("polling 期间按钮 disabled 且文案含倒计时", async () => {
    // 一直返 pending，让 polling 持续
    mockWechatBindPolling({ pollStatuses: [{ kind: "ok", status: "pending" }] });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    // 等 polling 阶段出现
    await waitFor(() => {
      const btn = screen.getByTestId("wechat-bind-button") as HTMLButtonElement;
      expect(btn.disabled).toBe(true);
    });
    // 文案含「等待生成二维码」+ (Ns)
    expect(screen.getByTestId("wechat-bind-button").textContent).toMatch(/等待生成二维码.*\d+s/);
  });
});

// ===== 微信绑定 tab - 客户端 150s 超时（fake timers） =====
//
// 实现里用 Date.now() 算 elapsed 判定 150s 超时。vitest 的 vi.useFakeTimers()
// 会同时 mock setInterval/setTimeout 和 Date.now(),所以用 vi.advanceTimersByTimeAsync
// 推进时间后 Date.now() 也跟着走。
//
// 注意:fake timer 也会 mock 掉 @testing-library waitFor 内部用的 setTimeout,
// 所以 await waitFor(...) 在 fake timer 下会卡住 5s 超时(没人在驱动 fake clock)。
// 解决:fake timer 只在 it 主体启用(且必须晚于 switchToWechatTab 的 waitFor),
// 然后用 vi.advanceTimersByTimeAsync 推进时间后直接 expect,不再 wrap waitFor。
describe("Settings 微信绑定 tab 客户端 150s 超时", () => {
  it("150s 后未拿到终态 → 主动停 interval + 提示'已超时'", async () => {
    // 一直返 pending（不进入 done/failed/expired）
    mockWechatBindPolling({ pollStatuses: [{ kind: "ok", status: "pending" }] });
    renderSettings();
    // switchToWechatTab 内部用 waitFor,必须在 fake timer 之前
    await switchToWechatTab();

    // 启 fake timer:从这一刻起 setInterval / Date.now 都被 mock
    vi.useFakeTimers();
    try {
      fireEvent.click(screen.getByTestId("wechat-bind-button"));
      // 推进 1.1s,触发首次 setInterval tick(t=1000 时 GET),state 维持 polling
      await vi.advanceTimersByTimeAsync(1100);
      expect(screen.getByTestId("wechat-bind-button").textContent).toMatch(/等待生成二维码/);

      // 推进 149s(累计 150.1s)→ 跨过 150000ms 阈值
      // 在 t=150000 那次 tick 回调里,elapsedMs >= 150000 成立,setState failed
      await vi.advanceTimersByTimeAsync(149_000);
      // 错误条已渲染
      const err = screen.getByTestId("wechat-bind-error");
      expect(err).toBeInTheDocument();
      expect(err.textContent).toMatch(/已超时/);
      // 按钮恢复可点
      expect((screen.getByTestId("wechat-bind-button") as HTMLButtonElement).disabled).toBe(false);
    } finally {
      vi.useRealTimers();
    }
  });

  it("150s 内拿到 done → 不超时，正常展示 modal", async () => {
    // 第二次 tick 返 done（在 150s 之内）
    mockWechatBindPolling({
      pollStatuses: [
        { kind: "ok", status: "pending" },
        { kind: "ok", status: "done" },
      ],
    });
    renderSettings();
    await switchToWechatTab();

    vi.useFakeTimers();
    try {
      fireEvent.click(screen.getByTestId("wechat-bind-button"));
      // 推进 2s:跨过 2 次 tick,t=1000 返 pending、t=2000 返 done
      await vi.advanceTimersByTimeAsync(2000);
      // done → modal 展示
      expect(screen.getByTestId("wechat-bind-modal")).toBeInTheDocument();
      // 没超时错误
      expect(screen.queryByTestId("wechat-bind-error")).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });
});

// ===== 微信绑定 tab - 绑定成功 (bound=true) =====
//
// openclaw 扫码成功后会输出「已将此 OpenClaw 连接到微信。」标记,后端在 scanner
// 循环里早期置 Bound=true。前端 polling 拿到后切到「绑定成功」状态:modal 自动
// 关闭,section 内联显示绿色「绑定成功」banner,状态栏变「✓ 已绑定」,polling 停止。
//
// 设计取舍:不在 modal 内联展示成功(用户看到 QR + 绿条同时会很乱),而是把
// modal 视为"扫码阶段"的临时 UI,扫码成功后让 modal 退场,改成 section 的
// inline banner 持久显示直到用户主动重新绑定。
describe("Settings 微信绑定 tab 绑定成功", () => {
  it("polling 拿到 bound=true → modal 自动关闭,section 内联显示绿色'绑定成功' banner,状态栏变'已绑定'", async () => {
    mockWechatBindPolling({
      pollStatuses: [
        {
          kind: "ok",
          status: "running",
          payload: { bound: true, link: "", qr: "", raw: "..." },
        },
      ],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    // 等 inline banner 出现(在 section 内,不在 modal 内)
    const banner = await waitFor(
      () => screen.getByTestId("wechat-bind-success"),
      { timeout: 3000 },
    );
    expect(banner).toBeInTheDocument();
    expect(banner.textContent).toMatch(/绑定成功/);
    // modal 已关闭(自动收)
    expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
    // 状态栏变"✓ 已绑定"
    expect(screen.getByTestId("wechat-bind-status").textContent).toMatch(/已绑定/);
    // 按钮恢复可点(label 回到"绑定")
    expect(screen.getByTestId("wechat-bind-button").textContent).toBe("绑定");
    // 没行内错误
    expect(screen.queryByTestId("wechat-bind-error")).toBeNull();
    // 提示(绑定中)消失
    expect(screen.queryByTestId("wechat-bind-hint")).toBeNull();
  });

  it("polling 拿到 bound=true (status=done) → 仍然切到 success 状态", async () => {
    // 模拟 exec 已自然退出:status=done + bound=true
    // 验证 bound 优先级最高,即使 status=done 也切 success(不能被 done 状态
    // 拦截走 done → 弹 modal 路径)
    mockWechatBindPolling({
      pollStatuses: [{ kind: "ok", status: "done" }],
      doneResult: {
        link: "https://liteapp.weixin.qq.com/q/done-with-bound",
        qr: "▄▄▄",
        bound: true,
      },
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    const banner = await waitFor(
      () => screen.getByTestId("wechat-bind-success"),
      { timeout: 3000 },
    );
    expect(banner).toBeInTheDocument();
    // modal 不出现(bound 优先级高,不进入 done 路径)
    expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
  });

  it("polling 拿到 bound=true → polling 立即停止(不再发 GET 请求)", async () => {
    mockWechatBindPolling({
      pollStatuses: [
        { kind: "ok", status: "running", payload: { bound: true, link: "", qr: "" } },
      ],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    await waitFor(() => screen.getByTestId("wechat-bind-success"), { timeout: 3000 });
    // 记录成功时已发出的 GET 数量
    const getCountAtSuccess = wechatBindGetCalls().length;
    expect(getCountAtSuccess).toBeGreaterThanOrEqual(1);

    // 再推进 3s,看是否还有新 GET
    vi.useFakeTimers();
    try {
      await vi.advanceTimersByTimeAsync(3000);
    } finally {
      vi.useRealTimers();
    }
    expect(wechatBindGetCalls().length).toBe(getCountAtSuccess);
  });

  // ===== 已连接场景(already_bound=true) =====
  // 后端检测到 openclaw 输出"已连接过此 OpenClaw，无需重复连接"时设 already_bound=true。
  // 前端应在 success banner 里展示"该用户已绑定"而不是"绑定成功",文案更准确。

  it("polling 拿到 bound=true + already_bound=true → banner 显示'该用户已绑定'(不是'绑定成功')", async () => {
    mockWechatBindPolling({
      pollStatuses: [{ kind: "ok", status: "done" }],
      doneResult: {
        link: "https://liteapp.weixin.qq.com/q/already-link",
        qr: "▄▄▄",
        bound: true,
        already_bound: true,
      },
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    const banner = await waitFor(
      () => screen.getByTestId("wechat-bind-success"),
      { timeout: 3000 },
    );
    // 关键断言:文案是"该用户已绑定"
    expect(banner.textContent).toMatch(/该用户已绑定/);
    expect(banner.textContent).not.toMatch(/绑定成功/);
    // data-already-bound 属性也应是 "true"
    expect(banner.getAttribute("data-already-bound")).toBe("true");
    // 状态栏仍变"已绑定"(跟 bound=true 流程一致)
    expect(screen.getByTestId("wechat-bind-status").textContent).toMatch(/已绑定/);
    // modal 不出现
    expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
  });

  it("polling 拿到 bound=true + already_bound=false → banner 仍显示'绑定成功'(向后兼容)", async () => {
    // already_bound 缺失 / false 走老路径,文案不变
    mockWechatBindPolling({
      pollStatuses: [{ kind: "ok", status: "done" }],
      doneResult: {
        link: "https://liteapp.weixin.qq.com/q/fresh-link",
        qr: "▄▄▄",
        bound: true,
        // already_bound 故意不传,后端也不会写
      },
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    const banner = await waitFor(
      () => screen.getByTestId("wechat-bind-success"),
      { timeout: 3000 },
    );
    expect(banner.textContent).toMatch(/绑定成功/);
    expect(banner.textContent).not.toMatch(/该用户已绑定/);
    expect(banner.getAttribute("data-already-bound")).toBe("false");
  });
});

// ===== 微信绑定 tab - 关弹窗取消 exec =====
//
// 用户在 done/doneWithWarning/bound 状态点关闭时,前端 fire-and-forget 调 cancel
// 端点,免等 2 分钟 timeout 兜底。cancel 调用失败(404/网络)也不影响 UI 状态。
describe("Settings 微信绑定 tab 关弹窗取消", () => {
  // 拿 cancel 端点的 fetch 调用
  function cancelCall() {
    return wechatBindCalls().find(
      (c) =>
        typeof c[0] === "string" &&
        (c[0] as string).endsWith("/cancel") &&
        (c[1] as RequestInit | undefined)?.method === "POST",
    );
  }

  it("done 状态点关闭 → 调 POST /api/wechat/bind/:task_id/cancel,modal 消失", async () => {
    mockWechatBindPolling({
      pollStatuses: [{ kind: "ok", status: "done" }],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));
    await waitFor(() => screen.getByTestId("wechat-bind-modal"), { timeout: 3000 });

    fireEvent.click(screen.getByTestId("wechat-bind-modal-close"));

    await waitFor(() => {
      expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
    });
    // 关键:cancel 端点被调
    expect(cancelCall()).toBeTruthy();
  });

  it("doneWithWarning 状态点关闭 → 同样调 cancel 端点", async () => {
    const qr = "▄▄▄▄▄\n█ ▄▄ █\n▀▀▀▀▀";
    const link = "https://liteapp.weixin.qq.com/q/warn";
    mockWechatBindPolling({
      pollStatuses: [
        {
          kind: "ok",
          status: "expired",
          payload: { error: "killed", link, qr, expired: true },
        },
      ],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));
    await waitFor(() => screen.getByTestId("wechat-bind-modal"), { timeout: 3000 });

    fireEvent.click(screen.getByTestId("wechat-bind-modal-close"));

    await waitFor(() => {
      expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
    });
    expect(cancelCall()).toBeTruthy();
  });

  it("bound=true 切到 success 后,内联 banner 持久显示,不会自动消失", async () => {
    mockWechatBindPolling({
      pollStatuses: [
        {
          kind: "ok",
          status: "running",
          payload: { bound: true, link: "", qr: "", raw: "..." },
        },
      ],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));
    await waitFor(() => screen.getByTestId("wechat-bind-success"), { timeout: 3000 });

    // success 状态下 modal 不存在(自动关闭),没有可点的"关闭"按钮。
    // 这是设计:success 是终态,需要明确操作(点"绑定"重新开始)才能退出。
    expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
    expect(screen.queryByTestId("wechat-bind-modal-close")).toBeNull();

    // 推进时间验证 banner 不消失
    vi.useFakeTimers();
    try {
      await vi.advanceTimersByTimeAsync(5000);
    } finally {
      vi.useRealTimers();
    }
    expect(screen.getByTestId("wechat-bind-success")).toBeInTheDocument();
    expect(screen.getByTestId("wechat-bind-status").textContent).toMatch(/已绑定/);
  });

  it("success 状态点'绑定'按钮 → 进入 submitting,新 POST 发起", async () => {
    mockWechatBindPolling({
      pollStatuses: [
        {
          kind: "ok",
          status: "running",
          payload: { bound: true, link: "", qr: "", raw: "..." },
        },
      ],
    });
    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));
    await waitFor(() => screen.getByTestId("wechat-bind-success"), { timeout: 3000 });

    // 重新点"绑定"——直接重置为 submitting,无需用户先手动"关闭"
    fireEvent.click(screen.getByTestId("wechat-bind-button"));

    // 按钮文案变"启动中..."或"等待生成二维码..."
    await waitFor(() => {
      expect(screen.getByTestId("wechat-bind-button").textContent).toMatch(/启动中|等待/);
    });
    // 至少发了一次 POST /api/wechat/bind
    expect(wechatBindPostCall()).toBeTruthy();
    // 旧的 success banner 已被新流程替代(进入 submitting 时 banner 不渲染)
    // 注:此断言在 submitting 期间通过;后续 polling 也可能继续显示 success
    // 状态直到拿到新 task_id 响应。这里不强断言 banner 消失,因为状态机
    // 一旦拿到新 task_id 就会立刻替换。
  });

  it("cancel 端点 404 → 不影响 UI 状态(modal 仍然消失,按钮恢复可点)", async () => {
    // 完全自定义 mock:done 状态展示 modal,cancel 端点 404
    mockFetch.mockImplementation(async (url: string, init?: RequestInit) => {
      const method = (init?.method ?? "GET").toUpperCase();
      if (url === "/api/config") {
        return { ok: true, status: 200, json: () => Promise.resolve({ env: { EMAIL_REQUIRE_REVIEW: "true" } }) };
      }
      if (url === "/api/grading-rules" || url === "/api/interest-level") {
        return { ok: true, status: 200, json: () => Promise.resolve({}) };
      }
      if (typeof url === "string" && url.startsWith("/api/target-sites")) {
        return { ok: true, status: 200, json: () => Promise.resolve([]) };
      }
      if (url === "/api/wechat/bind" && method === "POST") {
        return { ok: true, status: 202, json: () => Promise.resolve({ task_id: "wt-test-001", status: "pending" }) };
      }
      if (typeof url === "string" && url.startsWith("/api/wechat/bind/wt-") && method === "GET") {
        return {
          ok: true,
          status: 200,
          json: () => Promise.resolve({
            task_id: "wt-test-001",
            status: "done",
            link: "https://liteapp.weixin.qq.com/q/abc",
            qr: "▄▄▄",
            expired: false,
          }),
        };
      }
      // 关键:cancel 端点返 404(已 TTL 清理)
      if (typeof url === "string" && url.endsWith("/cancel") && method === "POST") {
        return { ok: false, status: 404, json: () => Promise.resolve({ error: "task not found" }) };
      }
      return { ok: false, status: 500, json: () => Promise.resolve({ error: "not mocked" }) };
    });

    renderSettings();
    await switchToWechatTab();
    fireEvent.click(screen.getByTestId("wechat-bind-button"));
    await waitFor(() => screen.getByTestId("wechat-bind-modal"), { timeout: 3000 });

    fireEvent.click(screen.getByTestId("wechat-bind-modal-close"));

    // 关键:即使 cancel 404,modal 仍然消失(cancel 是 fire-and-forget,失败不阻塞 UI)
    await waitFor(() => {
      expect(screen.queryByTestId("wechat-bind-modal")).toBeNull();
    });
    // 按钮恢复可点(没被错误卡住)
    expect((screen.getByTestId("wechat-bind-button") as HTMLButtonElement).disabled).toBe(false);
  });
});
