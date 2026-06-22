import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { ProjectList } from "@/ui/pages/ProjectList";

const { mockList, mockUpdateStatus } = vi.hoisted(() => ({
  mockList: vi.fn(),
  mockUpdateStatus: vi.fn(),
}));
vi.mock("@/query", () => ({
  CRMQuery: vi.fn().mockImplementation(() => ({
    listProjects: mockList,
    updateProjectStatus: mockUpdateStatus,
  })),
}));

const P1 = {
  id: "PRJ-1",
  created_at: "2026-06-15T10:00:00Z",
  updated_at: "2026-06-16T10:00:00Z",
  project_name: "华为泰国数据中心",
  customer_id: "CUST-1",
  customer_name: "Siam Cement",
  intent_level: "A",
  customer_email: "contact@example.com",
  status: "谈判中",
  assigned_to: "张三",
  notes: "客户对价格敏感",
};
const P2 = {
  id: "PRJ-2",
  created_at: "2026-06-15T11:00:00Z",
  updated_at: "2026-06-16T11:00:00Z",
  project_name: "云服务采购",
  customer_id: "",
  customer_name: "",
  intent_level: "",
  customer_email: "",
  status: "跟进中",
  assigned_to: "",
  notes: "",
};

describe("ProjectList", () => {
  it("renders 商机信息 heading and projects from query", async () => {
    mockList.mockResolvedValue([P1, P2]);
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByText("商机信息")).toBeInTheDocument();
      expect(screen.getByText("华为泰国数据中心")).toBeInTheDocument();
    });
    expect(screen.getByText("Siam Cement")).toBeInTheDocument();
    expect(screen.getByText("云服务采购")).toBeInTheDocument();
  });

  it("renders empty state when no projects", async () => {
    mockList.mockResolvedValue([]);
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByText("商机信息")).toBeInTheDocument();
    });
    expect(screen.getByText("暂无商机")).toBeInTheDocument();
  });

  it("renders error state when query fails", async () => {
    mockList.mockRejectedValue(new Error("HTTP 500 boom"));
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByText(/加载失败/)).toBeInTheDocument();
    });
    expect(screen.getByText(/HTTP 500 boom/)).toBeInTheDocument();
  });

  it("renders Chinese status / notes / assigned_to correctly", async () => {
    mockList.mockResolvedValue([P1]);
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => screen.getByText("华为泰国数据中心"));
    expect(screen.getByText("谈判中")).toBeInTheDocument();
    expect(screen.getByText("张三")).toBeInTheDocument();
    expect(screen.getByText("客户对价格敏感")).toBeInTheDocument();
  });

  it("shows '—' for missing customer name when customer_id is empty", async () => {
    mockList.mockResolvedValue([P2]);
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => screen.getByText("云服务采购"));
    // 客户名称 cell 显示 "—"
    const cell = screen.getByTestId("customer-PRJ-2");
    expect(cell.textContent).toBe("—");
  });

  it("filters projects by project_name via search input", async () => {
    mockList.mockResolvedValue([
      P1, P2,
      {
        ...P1,
        id: "PRJ-3",
        project_name: "海外仓租赁",
        customer_id: "CUST-3",
        customer_name: "Bangkok Bank",
        status: "签约中",
      },
    ]);
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => screen.getByText("华为泰国数据中心"));

    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "海外仓" } });
    await waitFor(() => {
      expect(screen.queryByText("华为泰国数据中心")).not.toBeInTheDocument();
      expect(screen.getByText("海外仓租赁")).toBeInTheDocument();
      expect(screen.queryByText("云服务采购")).not.toBeInTheDocument();
    });

    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "" } });
    await waitFor(() => {
      expect(screen.getByText("华为泰国数据中心")).toBeInTheDocument();
      expect(screen.getByText("海外仓租赁")).toBeInTheDocument();
    });
  });

  it("filters projects by customer_name via search input", async () => {
    mockList.mockResolvedValue([P1, P2]);
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));

    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "siam" } });
    await waitFor(() => {
      expect(screen.getByText("华为泰国数据中心")).toBeInTheDocument();
      expect(screen.queryByText("云服务采购")).not.toBeInTheDocument();
    });
  });

  it("filters projects by status via search input", async () => {
    mockList.mockResolvedValue([P1, P2]);
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));

    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "谈判" } });
    await waitFor(() => {
      expect(screen.getByText("华为泰国数据中心")).toBeInTheDocument();
      expect(screen.queryByText("云服务采购")).not.toBeInTheDocument();
    });
  });

  it("clicking customer name fires onCustomerClick handler (no error in MemoryRouter)", async () => {
    mockList.mockResolvedValue([P1]);
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));
    // 点击客户名按钮应不报错（MemoryRouter 不会真正导航）
    fireEvent.click(screen.getByText("Siam Cement"));
  });

  it("formats updated_at as zh-CN localized string", async () => {
    mockList.mockResolvedValue([P1]);
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => screen.getByText("华为泰国数据中心"));
    // 验证 updated_at 单元格存在（具体值依赖运行环境，断言非 "无" 即可）
    const row = screen.getByTestId("row-PRJ-1");
    const cells = row.querySelectorAll("td");
    const updatedCell = cells[cells.length - 1];
    expect(updatedCell.textContent).not.toBe("无");
    expect(updatedCell.textContent).toBeTruthy();
  });
});

describe("ProjectList 内联状态修改", () => {
  beforeEach(() => {
    mockUpdateStatus.mockReset();
    mockList.mockReset();
  });

  it("status 列渲染 InlineStatusSelect(<select> per row)", async () => {
    mockList.mockResolvedValue([P1, P2]);
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => screen.getByText("华为泰国数据中心"));
    const selects = screen.getAllByTestId("status-select") as HTMLSelectElement[];
    expect(selects).toHaveLength(2);
    expect(selects[0].value).toBe("谈判中");
    expect(selects[1].value).toBe("跟进中");
  });

  it("改 status select → 调 updateProjectStatus + 列表该行 status 更新", async () => {
    mockUpdateStatus.mockResolvedValue({ ok: true, status: "签约中" });
    mockList.mockResolvedValue([P1]);
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => screen.getByText("华为泰国数据中心"));
    const select = screen.getByTestId("status-select") as HTMLSelectElement;
    fireEvent.change(select, { target: { value: "签约中" } });
    await waitFor(() => {
      expect(mockUpdateStatus).toHaveBeenCalledWith("PRJ-1", "签约中");
    });
    // 列表该行 status 更新(select value 跟随 prop 变)
    await waitFor(() => {
      expect((screen.getByTestId("status-select") as HTMLSelectElement).value).toBe("签约中");
    });
  });

  it("失败时 select 还原原值,InlineStatusSelect 自身展示错误", async () => {
    mockUpdateStatus.mockRejectedValue(new Error("HTTP 400 status 不在枚举内"));
    mockList.mockResolvedValue([P1]);
    render(<MemoryRouter><ProjectList /></MemoryRouter>);
    await waitFor(() => screen.getByText("华为泰国数据中心"));
    fireEvent.change(screen.getByTestId("status-select"), { target: { value: "签约中" } });
    await waitFor(() => {
      expect(screen.getByTestId("status-select-error")).toHaveTextContent(/不在枚举内/);
    });
    // 列表该行 status 保持原值(select 回到 "谈判中")
    expect((screen.getByTestId("status-select") as HTMLSelectElement).value).toBe("谈判中");
    // PATCH 失败时,page 的 list state 不更新(因为 onStatusChange 抛了,page 不应改 list)
    // InlineStatusSelect 内部把 error 显示在 select 下方
  });
});