import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { TaskList } from "@/ui/pages/TaskList";

const { mockList } = vi.hoisted(() => ({ mockList: vi.fn() }));
vi.mock("@/query", () => ({
  CRMQuery: vi.fn().mockImplementation(() => ({
    listTasks: mockList,
  })),
}));

const T1 = {
  id: "TASK-1",
  created_at: "2026-06-15T10:00:00Z",
  updated_at: "2026-06-16T10:00:00Z",
  type: "compliance_blocked",
  priority: "P1",
  status: "pending",
  title: "合规文件缺失：泰国 BOI",
  description: "客户缺少投资促进委员会证明",
  customer_id: "CUST-1",
  customer_name: "Siam Cement",
  assigned_to: "张三",
};
const T2 = {
  id: "TASK-2",
  created_at: "2026-06-15T11:00:00Z",
  updated_at: "2026-06-16T11:00:00Z",
  type: "anomaly_alert",
  priority: "P0",
  status: "resolved",
  title: "异常告警",
  customer_id: "",
  customer_name: "",
  assigned_to: "",
};

describe("TaskList", () => {
  it("renders 代办任务 heading and tasks from query", async () => {
    mockList.mockResolvedValue([T1, T2]);
    render(<MemoryRouter><TaskList /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByText("代办任务")).toBeInTheDocument();
      expect(screen.getByText("合规文件缺失：泰国 BOI")).toBeInTheDocument();
    });
    expect(screen.getByText("Siam Cement")).toBeInTheDocument();
    // T2.title="异常告警" 与 T2.type="anomaly_alert"→"异常告警" 都会出现 → 用 getAllByText
    expect(screen.getAllByText("异常告警").length).toBeGreaterThanOrEqual(1);
  });

  it("renders empty state when no tasks", async () => {
    mockList.mockResolvedValue([]);
    render(<MemoryRouter><TaskList /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByText("代办任务")).toBeInTheDocument();
    });
    expect(screen.getByText("暂无代办任务")).toBeInTheDocument();
  });

  it("renders error state when query fails", async () => {
    mockList.mockRejectedValue(new Error("HTTP 500 boom"));
    render(<MemoryRouter><TaskList /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByText(/加载失败/)).toBeInTheDocument();
    });
    expect(screen.getByText(/HTTP 500 boom/)).toBeInTheDocument();
  });

  it("shows '—' for missing customer name when customer_id is empty", async () => {
    mockList.mockResolvedValue([T2]);
    render(<MemoryRouter><TaskList /></MemoryRouter>);
    await waitFor(() => screen.getAllByText("异常告警")[0]);
    const cell = screen.getByTestId("customer-TASK-2");
    expect(cell.textContent).toBe("—");
  });

  it("filters tasks by title via search input", async () => {
    mockList.mockResolvedValue([T1, T2]);
    render(<MemoryRouter><TaskList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));

    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "合规" } });
    await waitFor(() => {
      expect(screen.getByText("合规文件缺失：泰国 BOI")).toBeInTheDocument();
      expect(screen.queryByText("异常告警")).not.toBeInTheDocument();
    });
  });

  it("filters tasks by customer name via search input", async () => {
    mockList.mockResolvedValue([T1, T2]);
    render(<MemoryRouter><TaskList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));

    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "siam" } });
    await waitFor(() => {
      expect(screen.getByText("合规文件缺失：泰国 BOI")).toBeInTheDocument();
      expect(screen.queryByText("异常告警")).not.toBeInTheDocument();
    });
  });

  it("filters tasks by assigned_to via search input", async () => {
    mockList.mockResolvedValue([T1, T2]);
    render(<MemoryRouter><TaskList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));

    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "张三" } });
    await waitFor(() => {
      expect(screen.getByText("合规文件缺失：泰国 BOI")).toBeInTheDocument();
      expect(screen.queryByText("异常告警")).not.toBeInTheDocument();
    });
  });

  it("filters tasks by type via search input", async () => {
    mockList.mockResolvedValue([T1, T2]);
    render(<MemoryRouter><TaskList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));

    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "anomaly_alert" } });
    await waitFor(() => {
      expect(screen.queryByText("合规文件缺失：泰国 BOI")).not.toBeInTheDocument();
      // T2.title 与 T2.type→"异常告警" 同时出现，验证至少一个
      expect(screen.getAllByText("异常告警").length).toBeGreaterThanOrEqual(1);
    });
  });

  it("clicking customer name does not error in MemoryRouter", async () => {
    mockList.mockResolvedValue([T1]);
    render(<MemoryRouter><TaskList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));
    fireEvent.click(screen.getByText("Siam Cement"));
  });

  it("filters tasks by status dropdown", async () => {
    mockList.mockResolvedValue([T1, T2]);
    render(<MemoryRouter><TaskList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));

    // 默认显示全部
    expect(screen.getByText("合规文件缺失：泰国 BOI")).toBeInTheDocument();
    expect(screen.getAllByText("异常告警").length).toBeGreaterThanOrEqual(1);

    // 筛选 pending → 只显示 T1
    fireEvent.change(screen.getByTestId("filter-status"), { target: { value: "pending" } });
    await waitFor(() => {
      expect(screen.getByText("合规文件缺失：泰国 BOI")).toBeInTheDocument();
      expect(screen.queryByText("异常告警")).not.toBeInTheDocument();
    });

    // 筛选 resolved → 只显示 T2
    fireEvent.change(screen.getByTestId("filter-status"), { target: { value: "resolved" } });
    await waitFor(() => {
      expect(screen.queryByText("合规文件缺失：泰国 BOI")).not.toBeInTheDocument();
      expect(screen.getAllByText("异常告警").length).toBeGreaterThanOrEqual(1);
    });

    // 切回全部 → 都显示
    fireEvent.change(screen.getByTestId("filter-status"), { target: { value: "" } });
    await waitFor(() => {
      expect(screen.getByText("合规文件缺失：泰国 BOI")).toBeInTheDocument();
    });
  });

  it("filters tasks by priority dropdown", async () => {
    mockList.mockResolvedValue([T1, T2]);
    render(<MemoryRouter><TaskList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));

    // 筛选 P1 → 只显示 T1
    fireEvent.change(screen.getByTestId("filter-priority"), { target: { value: "P1" } });
    await waitFor(() => {
      expect(screen.getByText("合规文件缺失：泰国 BOI")).toBeInTheDocument();
      expect(screen.queryByText("异常告警")).not.toBeInTheDocument();
    });

    // 筛选 P0 → 只显示 T2
    fireEvent.change(screen.getByTestId("filter-priority"), { target: { value: "P0" } });
    await waitFor(() => {
      expect(screen.queryByText("合规文件缺失：泰国 BOI")).not.toBeInTheDocument();
      expect(screen.getAllByText("异常告警").length).toBeGreaterThanOrEqual(1);
    });

    // 切回全部 → 都显示
    fireEvent.change(screen.getByTestId("filter-priority"), { target: { value: "" } });
    await waitFor(() => {
      expect(screen.getByText("合规文件缺失：泰国 BOI")).toBeInTheDocument();
    });
  });

  it("combines status + priority filter with text search", async () => {
    const T3 = {
      id: "TASK-3",
      created_at: "2026-06-15T12:00:00Z",
      updated_at: "2026-06-16T12:00:00Z",
      type: "llm_failure",
      priority: "P1",
      status: "pending",
      title: "LLM 调用失败",
      customer_id: "CUST-2",
      customer_name: "Bangkok Bank",
      assigned_to: "李四",
    };
    mockList.mockResolvedValue([T1, T2, T3]);
    render(<MemoryRouter><TaskList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Siam Cement"));

    // 筛选 status=pending + priority=P1
    fireEvent.change(screen.getByTestId("filter-status"), { target: { value: "pending" } });
    fireEvent.change(screen.getByTestId("filter-priority"), { target: { value: "P1" } });
    await waitFor(() => {
      // T1 (pending,P1) 和 T3 (pending,P1) 应显示，T2 (resolved,P0) 不显示
      expect(screen.getByText("合规文件缺失：泰国 BOI")).toBeInTheDocument();
      expect(screen.getByText("LLM 调用失败")).toBeInTheDocument();
      expect(screen.queryByText("异常告警")).not.toBeInTheDocument();
    });

    // 再加文本搜索
    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "LLM" } });
    await waitFor(() => {
      expect(screen.queryByText("合规文件缺失：泰国 BOI")).not.toBeInTheDocument();
      expect(screen.getByText("LLM 调用失败")).toBeInTheDocument();
    });
  });
});
