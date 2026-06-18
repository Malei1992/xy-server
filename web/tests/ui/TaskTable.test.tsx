import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TaskTable } from "@/ui/components/TaskTable";
import type { Task } from "@/query/types";

const T1: Task = {
  id: "TASK-1",
  created_at: "2026-06-15T10:00:00Z",
  updated_at: "2026-06-16T10:00:00Z",
  source: "ProspectorAgent",
  type: "compliance_blocked",
  priority: "P1",
  status: "pending",
  title: "合规文件缺失：泰国 BOI",
  description: "客户缺少投资促进委员会证明，请尽快补充",
  customer_id: "CUST-1",
  customer_name: "Siam Cement Group",
  email_id: "MAIL-1",
  assigned_to: "张三",
  resolution: "已补充 BOI 证书",
  resolved_at: "2026-06-16T11:00:00Z",
};

const T2: Task = {
  id: "TASK-2",
  created_at: "2026-06-15T11:00:00Z",
  updated_at: "2026-06-16T11:00:00Z",
  type: "anomaly_alert",
  priority: "P0",
  status: "resolved",
  title: "异常告警：重复注册",
  customer_id: "",
  customer_name: "",
};

describe("TaskTable", () => {
  it("renders '暂无代办任务' when tasks is empty", () => {
    render(<TaskTable tasks={[]} onCustomerClick={vi.fn()} />);
    expect(screen.getByText("暂无代办任务")).toBeInTheDocument();
  });

  it("renders all 9 column headers", () => {
    render(<TaskTable tasks={[T1]} onCustomerClick={vi.fn()} />);
    const table = screen.getByTestId("tasks-table");
    const headers = table.querySelectorAll("th");
    expect(headers).toHaveLength(9);
    const titles = Array.from(headers).map((h) => h.textContent);
    expect(titles).toEqual([
      "任务标题", "任务类型", "任务等级", "任务状态", "客户名称",
      "详细说明", "负责人", "解决时间", "解决说明",
    ]);
  });

  it("renders task fields with correct values", () => {
    render(<TaskTable tasks={[T1]} onCustomerClick={vi.fn()} />);
    expect(screen.getByText("合规文件缺失：泰国 BOI")).toBeInTheDocument();
    // type / status 走中文映射
    expect(screen.getByText("合规阻断")).toBeInTheDocument();
    expect(screen.getByText("待处理")).toBeInTheDocument();
    expect(screen.getByText("Siam Cement Group")).toBeInTheDocument();
    expect(screen.getByText("客户缺少投资促进委员会证明，请尽快补充")).toBeInTheDocument();
    expect(screen.getByText("张三")).toBeInTheDocument();
    expect(screen.getByText("已补充 BOI 证书")).toBeInTheDocument();
  });

  it("renders Chinese labels for all 4 task statuses", () => {
    render(<TaskTable tasks={[
      { ...T1, id: "TASK-pending", status: "pending" },
      { ...T1, id: "TASK-progress", status: "in_progress" },
      { ...T1, id: "TASK-resolved", status: "resolved" },
      { ...T1, id: "TASK-dismissed", status: "dismissed" },
    ]} onCustomerClick={vi.fn()} />);
    expect(screen.getByText("待处理")).toBeInTheDocument();
    expect(screen.getByText("处理中")).toBeInTheDocument();
    expect(screen.getByText("已解决")).toBeInTheDocument();
    expect(screen.getByText("已驳回")).toBeInTheDocument();
  });

  it("renders Chinese labels for the common 3 task types (sample)", () => {
    render(<TaskTable tasks={[
      { ...T1, id: "TASK-compliance", type: "compliance_blocked" },
      { ...T1, id: "TASK-anomaly", type: "anomaly_alert" },
      { ...T1, id: "TASK-low", type: "low_confidence" },
    ]} onCustomerClick={vi.fn()} />);
    expect(screen.getByText("合规阻断")).toBeInTheDocument();
    expect(screen.getByText("异常告警")).toBeInTheDocument();
    expect(screen.getByText("低置信度")).toBeInTheDocument();
  });

  it("keeps raw enum in title attribute for type and status (hover tooltip)", () => {
    render(<TaskTable tasks={[T1]} onCustomerClick={vi.fn()} />);
    const row = screen.getByTestId("row-TASK-1");
    const cells = row.querySelectorAll("td");
    // 1=任务类型, 3=任务状态
    expect(cells[1].getAttribute("title")).toBe("compliance_blocked");
    expect(cells[3].getAttribute("title")).toBe("pending");
  });

  it("renders priority as a circular badge", () => {
    render(<TaskTable tasks={[T1]} onCustomerClick={vi.fn()} />);
    const badge = screen.getByTestId("priority-P1");
    expect(badge).toBeInTheDocument();
    expect(badge.style.borderRadius).toBe("50%");
  });

  it("renders different priorities for different tasks", () => {
    render(<TaskTable tasks={[T1, T2]} onCustomerClick={vi.fn()} />);
    expect(screen.getByTestId("priority-P1")).toBeInTheDocument();
    expect(screen.getByTestId("priority-P0")).toBeInTheDocument();
  });

  it("renders '—' for missing customer name when customer_id is empty", () => {
    render(<TaskTable tasks={[T2]} onCustomerClick={vi.fn()} />);
    const cell = screen.getByTestId("customer-TASK-2");
    expect(cell.textContent).toBe("—");
  });

  it("renders '无' for missing optional fields (description, assigned_to, resolved_at, resolution)", () => {
    render(<TaskTable tasks={[T2]} onCustomerClick={vi.fn()} />);
    const row = screen.getByTestId("row-TASK-2");
    const cells = row.querySelectorAll("td");
    // 6=详细说明, 7=负责人, 8=解决时间, 9=解决说明
    expect(cells[5].textContent).toBe("无");
    expect(cells[6].textContent).toBe("无");
    expect(cells[7].textContent).toBe("无");
    expect(cells[8].textContent).toBe("无");
  });

  it("calls onCustomerClick with customer_id when customer name button is clicked", () => {
    const handler = vi.fn();
    render(<TaskTable tasks={[T1]} onCustomerClick={handler} />);
    fireEvent.click(screen.getByText("Siam Cement Group"));
    expect(handler).toHaveBeenCalledWith("CUST-1");
  });

  it("formats resolved_at as zh-CN localized string when present", () => {
    render(<TaskTable tasks={[T1]} onCustomerClick={vi.fn()} />);
    const row = screen.getByTestId("row-TASK-1");
    const cells = row.querySelectorAll("td");
    const resolvedCell = cells[7]; // 解决时间列
    expect(resolvedCell.textContent).not.toBe("无");
    expect(resolvedCell.textContent).toBeTruthy();
  });
});
