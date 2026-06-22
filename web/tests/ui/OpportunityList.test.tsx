import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { OpportunityList } from "@/ui/pages/OpportunityList";

const { mockList, mockUpdateStatus } = vi.hoisted(() => ({
  mockList: vi.fn(),
  mockUpdateStatus: vi.fn(),
}));
vi.mock("@/query", () => ({
  CRMQuery: vi.fn().mockImplementation(() => ({
    listOpportunities: mockList,
    updateOpportunityStatus: mockUpdateStatus,
  })),
}));

const O1 = {
  id: "OPP-1",
  created_at: "2026-06-15T10:00:00Z",
  updated_at: "2026-06-16T10:00:00Z",
  opportunity_name: "泰国正大集团拟新建食品加工厂",
  customer_id: "CUST-1",
  customer_name: "Siam Cement",
  opportunity_info: "占地约 200 亩，预计投资 5 亿美元",
  source_url: "https://example.com/news/123",
  source_type: "新闻搜索",
  status: "待评估",
  notes: "与张三跟进重叠",
};
const O2 = {
  id: "OPP-2",
  created_at: "2026-06-15T11:00:00Z",
  updated_at: "2026-06-16T11:00:00Z",
  opportunity_name: "曼谷厂房租赁招标",
  customer_id: "",
  customer_name: "",
  opportunity_info: "",
  source_url: "",
  source_type: "招标公告",
  status: "跟进中",
  notes: "",
};

describe("OpportunityList", () => {
  it("renders 公开信息 heading and opportunities from query", async () => {
    mockList.mockResolvedValue([O1, O2]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByText("公开信息")).toBeInTheDocument();
      expect(screen.getByText("泰国正大集团拟新建食品加工厂")).toBeInTheDocument();
    });
    expect(screen.getByText("Siam Cement")).toBeInTheDocument();
    expect(screen.getByText("曼谷厂房租赁招标")).toBeInTheDocument();
  });

  it("renders empty state when no opportunities", async () => {
    mockList.mockResolvedValue([]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByText("公开信息")).toBeInTheDocument();
    });
    expect(screen.getByText("暂无公开信息")).toBeInTheDocument();
  });

  it("renders error state when query fails", async () => {
    mockList.mockRejectedValue(new Error("HTTP 500 boom"));
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByText(/加载失败/)).toBeInTheDocument();
    });
    expect(screen.getByText(/HTTP 500 boom/)).toBeInTheDocument();
  });

  it("renders source_url as a clickable link with target=_blank", async () => {
    mockList.mockResolvedValue([O1]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => screen.getByText("泰国正大集团拟新建食品加工厂"));
    const link = screen.getByRole("link", { name: "https://example.com/news/123" });
    expect(link).toHaveAttribute("href", "https://example.com/news/123");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("shows '—' for missing source_url when not provided", async () => {
    mockList.mockResolvedValue([O2]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => screen.getByText("曼谷厂房租赁招标"));
    const cell = screen.getByTestId("source-url-OPP-2");
    expect(cell.textContent).toBe("—");
  });

  it("renders Chinese source_type and status", async () => {
    mockList.mockResolvedValue([O1]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => screen.getByText("泰国正大集团拟新建食品加工厂"));
    expect(screen.getByText("新闻搜索")).toBeInTheDocument();
    expect(screen.getByText("待评估")).toBeInTheDocument();
  });

  it("shows '—' for missing customer name when customer_id is empty", async () => {
    mockList.mockResolvedValue([O2]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => screen.getByText("曼谷厂房租赁招标"));
    const cell = screen.getByTestId("customer-OPP-2");
    expect(cell.textContent).toBe("—");
  });

  it("shows '无' for missing notes / opportunity_info / source_url", async () => {
    mockList.mockResolvedValue([O2]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => screen.getByText("曼谷厂房租赁招标"));
    const row = screen.getByTestId("row-OPP-2");
    const noCount = row.textContent?.split("无").length ?? 0;
    // notes + opportunity_info 缺失 → 出现 2 次 "无"
    expect(noCount - 1).toBeGreaterThanOrEqual(2);
  });

  it("filters opportunities by opportunity_name via search input", async () => {
    mockList.mockResolvedValue([O1, O2]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => screen.getByText("泰国正大集团拟新建食品加工厂"));

    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "曼谷" } });
    await waitFor(() => {
      expect(screen.queryByText("泰国正大集团拟新建食品加工厂")).not.toBeInTheDocument();
      expect(screen.getByText("曼谷厂房租赁招标")).toBeInTheDocument();
    });
  });

  it("filters opportunities by customer_name via search input", async () => {
    mockList.mockResolvedValue([O1, O2]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));

    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "siam" } });
    await waitFor(() => {
      expect(screen.getByText("泰国正大集团拟新建食品加工厂")).toBeInTheDocument();
      expect(screen.queryByText("曼谷厂房租赁招标")).not.toBeInTheDocument();
    });
  });

  it("filters opportunities by source_type via search input", async () => {
    mockList.mockResolvedValue([O1, O2]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => screen.getByText("泰国正大集团拟新建食品加工厂"));

    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "招标" } });
    await waitFor(() => {
      expect(screen.queryByText("泰国正大集团拟新建食品加工厂")).not.toBeInTheDocument();
      expect(screen.getByText("曼谷厂房租赁招标")).toBeInTheDocument();
    });
  });

  it("clicking customer name does not error in MemoryRouter", async () => {
    mockList.mockResolvedValue([O1]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));
    fireEvent.click(screen.getByText("Siam Cement"));
  });
});

describe("OpportunityList 内联状态修改", () => {
  beforeEach(() => {
    mockUpdateStatus.mockReset();
    mockList.mockReset();
  });

  it("status 列渲染 InlineStatusSelect(<select> per row)", async () => {
    mockList.mockResolvedValue([O1, O2]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => screen.getByText("泰国正大集团拟新建食品加工厂"));
    const selects = screen.getAllByTestId("status-select") as HTMLSelectElement[];
    expect(selects).toHaveLength(2);
    expect(selects[0].value).toBe("待评估");
    expect(selects[1].value).toBe("跟进中");
  });

  it("改 status select → 调 updateOpportunityStatus + 列表该行 status 更新", async () => {
    mockUpdateStatus.mockResolvedValue({ ok: true, status: "跟进中" });
    mockList.mockResolvedValue([O1]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => screen.getByText("泰国正大集团拟新建食品加工厂"));
    fireEvent.change(screen.getByTestId("status-select"), { target: { value: "跟进中" } });
    await waitFor(() => {
      expect(mockUpdateStatus).toHaveBeenCalledWith("OPP-1", "跟进中");
    });
    await waitFor(() => {
      expect((screen.getByTestId("status-select") as HTMLSelectElement).value).toBe("跟进中");
    });
  });

  it("失败时 select 还原原值 + InlineStatusSelect 自身展示错误", async () => {
    mockUpdateStatus.mockRejectedValue(new Error("HTTP 404 opportunity not found"));
    mockList.mockResolvedValue([O1]);
    render(<MemoryRouter><OpportunityList /></MemoryRouter>);
    await waitFor(() => screen.getByText("泰国正大集团拟新建食品加工厂"));
    fireEvent.change(screen.getByTestId("status-select"), { target: { value: "已转化" } });
    await waitFor(() => {
      expect(screen.getByTestId("status-select-error")).toHaveTextContent(/opportunity not found/);
    });
    expect((screen.getByTestId("status-select") as HTMLSelectElement).value).toBe("待评估");
  });
});