import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { CustomerDetail } from "@/ui/pages/CustomerDetail";

const { mockGetCustomer, mockPatchCustomer } = vi.hoisted(() => ({
  mockGetCustomer: vi.fn(),
  mockPatchCustomer: vi.fn(),
}));
vi.mock("@/query", () => ({
  CRMQuery: vi.fn().mockImplementation(() => ({
    getCustomer: mockGetCustomer,
    patchCustomer: mockPatchCustomer,
  })),
}));

function makeCustomer(overrides?: Record<string, unknown>) {
  return {
    id: "C1",
    basic: { name: "测试公司", country: "泰国", industry: "X", contacts: "a@x.com", phones: "+66" },
    engagement: { status: "active", intent_level: "A" },
    prospecting: { grade: "A", overall_risk: "低" },
    timeline: [],
    ...overrides,
  };
}

describe("CustomerDetail", () => {
  it("renders customer basic info and timeline", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer({
      timeline: [
        { event: "created", by: "P", at: "2026-06-01T00:00:00Z" },
        { event: "screening", by: "P", at: "2026-06-02T00:00:00Z" },
      ],
    }));

    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getAllByText("测试公司").length).toBeGreaterThan(0);
    });
    expect(screen.getByText("created")).toBeInTheDocument();
    expect(screen.getByText("screening")).toBeInTheDocument();
  });

  it("shows error when customer fetch fails", async () => {
    mockGetCustomer.mockRejectedValue(new Error("HTTP 404 Not Found"));
    render(
      <MemoryRouter initialEntries={["/customers/missing"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getByText(/加载失败/)).toBeInTheDocument();
    });
  });

  it("shows '无' for schema fields that are missing in data", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer({
      basic: { name: "空字段公司", country: "泰国", industry: "X", contacts: "", phones: "" },
    }));

    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getAllByText("空字段公司").length).toBeGreaterThan(0));
    const noneCells = screen.getAllByText("无");
    expect(noneCells.length).toBeGreaterThan(5);
  });

  it("renders section titles for basic/prospecting/engagement", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer({
      basic: { name: "测试", country: "泰国", industry: "X", contacts: "", phones: "" },
    }));
    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => screen.getByText("客户基本信息"));
    expect(screen.getByText("爬取阶段信息")).toBeInTheDocument();
    expect(screen.getByText("跟进与意向")).toBeInTheDocument();
    expect(screen.getByText("时间线")).toBeInTheDocument();
  });

  // ===== 编辑功能测试（芯片输入模式）=====

  it("clicking edit button shows chip add input and save/cancel buttons", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer());
    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => screen.getByText("客户基本信息"));

    fireEvent.click(screen.getByTestId("edit-contacts-btn"));

    // 添加输入框 + 保存/取消按钮
    expect(screen.getByTestId("input-contacts-add")).toBeInTheDocument();
    expect(screen.getByTestId("save-contacts-btn")).toBeInTheDocument();
    expect(screen.getByTestId("cancel-contacts-btn")).toBeInTheDocument();
  });

  it("shows existing value as a chip in edit mode", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer({
      basic: { name: "测试公司", country: "泰国", industry: "X", contacts: "old@x.com", phones: "+66" },
    }));
    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => screen.getByText("客户基本信息"));

    fireEvent.click(screen.getByTestId("edit-contacts-btn"));

    // 已有值应显示为芯片
    expect(screen.getByTestId("chip-input-contacts-0")).toBeInTheDocument();
    expect(screen.getByTestId("chip-input-contacts-0").textContent).toContain("old@x.com");
  });

  it("adds a chip by typing and pressing Enter", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer({
      basic: { name: "测试公司", country: "泰国", industry: "X", contacts: "", phones: "" },
    }));
    mockPatchCustomer.mockResolvedValue({} as never);

    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => screen.getByText("客户基本信息"));

    fireEvent.click(screen.getByTestId("edit-contacts-btn"));

    // 在添加输入框中输入并回车
    const input = screen.getByTestId("input-contacts-add") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "new@x.com" } });
    fireEvent.keyDown(input, { key: "Enter" });

    // 应出现 chip
    expect(screen.getByTestId("chip-input-contacts-0")).toBeInTheDocument();
    expect(screen.getByTestId("chip-input-contacts-0").textContent).toContain("new@x.com");

    // 输入框应清空
    expect((screen.getByTestId("input-contacts-add") as HTMLInputElement).value).toBe("");
  });

  it("pasting comma-separated values splits into multiple chips", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer({
      basic: { name: "测试公司", country: "泰国", industry: "X", contacts: "", phones: "" },
    }));
    mockPatchCustomer.mockResolvedValue({} as never);

    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => screen.getByText("客户基本信息"));

    fireEvent.click(screen.getByTestId("edit-contacts-btn"));

    const input = screen.getByTestId("input-contacts-add") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "a@x.com, b@x.com" } });
    // 失焦触发 commitAdd
    fireEvent.blur(input);

    expect(screen.getByTestId("chip-input-contacts-0")).toBeInTheDocument();
    expect(screen.getByTestId("chip-input-contacts-1")).toBeInTheDocument();
  });

  it("removes a chip when clicking X button", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer({
      basic: { name: "测试公司", country: "泰国", industry: "X", contacts: ["a@x.com", "b@x.com"], phones: "" },
    }));
    mockPatchCustomer.mockResolvedValue({} as never);

    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => screen.getByText("客户基本信息"));

    fireEvent.click(screen.getByTestId("edit-contacts-btn"));

    // 应有 2 个 chip
    expect(screen.getByTestId("chip-input-contacts-0")).toBeInTheDocument();
    expect(screen.getByTestId("chip-input-contacts-1")).toBeInTheDocument();

    // 点击第二个 chip 的 X
    fireEvent.click(screen.getByTestId("chip-remove-input-contacts-1"));

    // 只剩下 1 个 chip
    expect(screen.getByTestId("chip-input-contacts-0")).toBeInTheDocument();
    expect(screen.queryByTestId("chip-input-contacts-1")).not.toBeInTheDocument();
  });

  it("saves chips correctly: single item as string, multiple as array", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer({
      basic: { name: "测试公司", country: "泰国", industry: "X", contacts: "", phones: "" },
    }));
    mockPatchCustomer.mockResolvedValue({} as never);

    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => screen.getByText("客户基本信息"));

    // 编辑联系人：添加单个值
    fireEvent.click(screen.getByTestId("edit-contacts-btn"));
    const input = screen.getByTestId("input-contacts-add") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "single@x.com" } });
    fireEvent.keyDown(input, { key: "Enter" });
    fireEvent.click(screen.getByTestId("save-contacts-btn"));
    await waitFor(() => {
      expect(mockPatchCustomer).toHaveBeenCalledWith("C1", { contacts: "single@x.com" });
    });

    mockPatchCustomer.mockClear();

    // 再次编辑：先删掉旧 chip，再添加多个新值
    fireEvent.click(screen.getByTestId("edit-contacts-btn"));
    fireEvent.click(screen.getByTestId("chip-remove-input-contacts-0")); // 删掉 "single@x.com"
    fireEvent.change(screen.getByTestId("input-contacts-add"), { target: { value: "a@x.com" } });
    fireEvent.keyDown(screen.getByTestId("input-contacts-add"), { key: "Enter" });
    fireEvent.change(screen.getByTestId("input-contacts-add"), { target: { value: "b@x.com" } });
    fireEvent.keyDown(screen.getByTestId("input-contacts-add"), { key: "Enter" });
    fireEvent.click(screen.getByTestId("save-contacts-btn"));
    await waitFor(() => {
      expect(mockPatchCustomer).toHaveBeenCalledWith("C1", { contacts: ["a@x.com", "b@x.com"] });
    });
  });

  it("edits phones and saves successfully using chip input", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer({
      basic: { name: "测试公司", country: "泰国", industry: "X", contacts: "a@x.com", phones: "+66" },
    }));
    mockPatchCustomer.mockResolvedValue({} as never);

    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => screen.getByText("客户基本信息"));

    fireEvent.click(screen.getByTestId("edit-phones-btn"));

    // 移除现有 chip，添加新的
    fireEvent.click(screen.getByTestId("chip-remove-input-phones-0"));
    const input = screen.getByTestId("input-phones-add") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "+86123456789" } });
    fireEvent.keyDown(input, { key: "Enter" });

    fireEvent.click(screen.getByTestId("save-phones-btn"));

    await waitFor(() => {
      expect(mockPatchCustomer).toHaveBeenCalledWith("C1", { phones: "+86123456789" });
    });
  });

  it("cancel restores original value and does not call API", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer({
      basic: { name: "测试公司", country: "泰国", industry: "X", contacts: "original@x.com", phones: "" },
    }));
    mockPatchCustomer.mockClear();

    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => screen.getByText("客户基本信息"));
    expect(screen.getByText("original@x.com")).toBeInTheDocument();

    // 编辑并添加
    fireEvent.click(screen.getByTestId("edit-contacts-btn"));
    const input = screen.getByTestId("input-contacts-add") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "changed@x.com" } });
    fireEvent.keyDown(input, { key: "Enter" });

    // 取消
    fireEvent.click(screen.getByTestId("cancel-contacts-btn"));

    // 原始值恢复
    expect(screen.getByText("original@x.com")).toBeInTheDocument();
    expect(mockPatchCustomer).not.toHaveBeenCalled();
  });

  it("shows error message when save fails using chip input", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer({
      basic: { name: "测试公司", country: "泰国", industry: "X", contacts: "old@x.com", phones: "" },
    }));
    mockPatchCustomer.mockRejectedValue(new Error("服务器内部错误"));

    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => screen.getByText("客户基本信息"));

    fireEvent.click(screen.getByTestId("edit-contacts-btn"));

    // 修改（添加一个 chip）
    const input = screen.getByTestId("input-contacts-add") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "new@x.com" } });
    fireEvent.keyDown(input, { key: "Enter" });

    fireEvent.click(screen.getByTestId("save-contacts-btn"));

    await waitFor(() => {
      expect(screen.getByText("服务器内部错误")).toBeInTheDocument();
    });
  });

  it("Backspace on empty add input removes last chip", async () => {
    mockGetCustomer.mockResolvedValue(makeCustomer({
      basic: { name: "测试公司", country: "泰国", industry: "X", contacts: ["a@x.com", "b@x.com"], phones: "" },
    }));
    mockPatchCustomer.mockResolvedValue({} as never);

    render(
      <MemoryRouter initialEntries={["/customers/C1"]}>
        <Routes>
          <Route path="/customers/:id" element={<CustomerDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => screen.getByText("客户基本信息"));

    fireEvent.click(screen.getByTestId("edit-contacts-btn"));

    // 2 个 chips
    expect(screen.getByTestId("chip-input-contacts-0")).toBeInTheDocument();
    expect(screen.getByTestId("chip-input-contacts-1")).toBeInTheDocument();

    // 在空输入框按 Backspace
    const input = screen.getByTestId("input-contacts-add") as HTMLInputElement;
    fireEvent.keyDown(input, { key: "Backspace" });

    // 最后一个 chip 被删除
    expect(screen.getByTestId("chip-input-contacts-0")).toBeInTheDocument();
    expect(screen.queryByTestId("chip-input-contacts-1")).not.toBeInTheDocument();
  });
});
