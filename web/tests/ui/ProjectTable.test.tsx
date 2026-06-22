import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ProjectTable } from "@/ui/components/ProjectTable";
import { PROJECT_STATUS_OPTIONS } from "@/query/types";
import type { Project } from "@/query/types";

const P1: Project = {
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

const P2: Project = {
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

describe("ProjectTable", () => {
  it("renders '暂无商机' when projects is empty", () => {
    render(
      <ProjectTable
        projects={[]}
        onCustomerClick={vi.fn()}
        statusOptions={PROJECT_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    expect(screen.getByText("暂无商机")).toBeInTheDocument();
  });

  it("renders 8 column headers (no 操作 column)", () => {
    render(
      <ProjectTable
        projects={[P1]}
        onCustomerClick={vi.fn()}
        statusOptions={PROJECT_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    const table = screen.getByTestId("projects-table");
    const headers = table.querySelectorAll("th");
    expect(headers).toHaveLength(8);
    const titles = Array.from(headers).map((h) => h.textContent);
    expect(titles).toEqual([
      "项目名称", "客户名称", "意向等级", "跟进状态", "邮箱",
      "负责人", "备注说明", "更新时间",
    ]);
  });

  it("renders project fields with correct values", () => {
    render(
      <ProjectTable
        projects={[P1]}
        onCustomerClick={vi.fn()}
        statusOptions={PROJECT_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    expect(screen.getByText("华为泰国数据中心")).toBeInTheDocument();
    expect(screen.getByText("Siam Cement")).toBeInTheDocument();
    expect(screen.getByText("A")).toBeInTheDocument();
    expect(screen.getByText("客户对价格敏感")).toBeInTheDocument();
  });

  it("status column renders an InlineStatusSelect per row", () => {
    render(
      <ProjectTable
        projects={[P1, P2]}
        onCustomerClick={vi.fn()}
        statusOptions={PROJECT_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    const selects = screen.getAllByTestId("status-select");
    expect(selects).toHaveLength(2);
    expect((selects[0] as HTMLSelectElement).value).toBe("谈判中");
    expect((selects[1] as HTMLSelectElement).value).toBe("跟进中");
  });

  it("status cell preserves raw enum in title attribute (hover tooltip)", () => {
    render(
      <ProjectTable
        projects={[P1]}
        onCustomerClick={vi.fn()}
        statusOptions={PROJECT_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    const row = screen.getByTestId("row-PRJ-1");
    const cells = row.querySelectorAll("td");
    // status 列 = 第 4 列(index 3)
    expect(cells[3].getAttribute("title")).toBe("谈判中");
  });

  it("shows '—' for missing customer name when customer_id is empty", () => {
    render(
      <ProjectTable
        projects={[P2]}
        onCustomerClick={vi.fn()}
        statusOptions={PROJECT_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    const cell = screen.getByTestId("customer-PRJ-2");
    expect(cell.textContent).toBe("—");
  });

  it("calls onCustomerClick with customer_id when customer name button is clicked", () => {
    const handler = vi.fn();
    render(
      <ProjectTable
        projects={[P1]}
        onCustomerClick={handler}
        statusOptions={PROJECT_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByText("Siam Cement"));
    expect(handler).toHaveBeenCalledWith("CUST-1");
  });

  it("changing status select calls onStatusChange(id, newStatus)", async () => {
    const onStatusChange = vi.fn().mockResolvedValue(undefined);
    render(
      <ProjectTable
        projects={[P1]}
        onCustomerClick={vi.fn()}
        statusOptions={PROJECT_STATUS_OPTIONS}
        onStatusChange={onStatusChange}
      />,
    );
    fireEvent.change(screen.getByTestId("status-select"), { target: { value: "签约中" } });
    expect(onStatusChange).toHaveBeenCalledWith("PRJ-1", "签约中");
  });

  it("formats updated_at as zh-CN localized string when present", () => {
    render(
      <ProjectTable
        projects={[P1]}
        onCustomerClick={vi.fn()}
        statusOptions={PROJECT_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    const row = screen.getByTestId("row-PRJ-1");
    const cells = row.querySelectorAll("td");
    const updatedCell = cells[cells.length - 1];
    expect(updatedCell.textContent).not.toBe("无");
    expect(updatedCell.textContent).toBeTruthy();
  });
});