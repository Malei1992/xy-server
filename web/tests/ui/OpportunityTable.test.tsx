import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { OpportunityTable } from "@/ui/components/OpportunityTable";
import { OPPORTUNITY_STATUS_OPTIONS } from "@/query/types";
import type { Opportunity } from "@/query/types";

const O1: Opportunity = {
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

const O2: Opportunity = {
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

describe("OpportunityTable", () => {
  it("renders '暂无公开信息' when opportunities is empty", () => {
    render(
      <OpportunityTable
        opportunities={[]}
        onCustomerClick={vi.fn()}
        statusOptions={OPPORTUNITY_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    expect(screen.getByText("暂无公开信息")).toBeInTheDocument();
  });

  it("renders 7 column headers (no 操作 column)", () => {
    render(
      <OpportunityTable
        opportunities={[O1]}
        onCustomerClick={vi.fn()}
        statusOptions={OPPORTUNITY_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    const table = screen.getByTestId("opportunities-table");
    const headers = table.querySelectorAll("th");
    expect(headers).toHaveLength(7);
    const titles = Array.from(headers).map((h) => h.textContent);
    expect(titles).toEqual([
      "名称", "详情", "信息来源", "来源类型", "状态", "客户名称", "说明",
    ]);
  });

  it("renders opportunity fields with correct values", () => {
    render(
      <OpportunityTable
        opportunities={[O1]}
        onCustomerClick={vi.fn()}
        statusOptions={OPPORTUNITY_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    expect(screen.getByText("泰国正大集团拟新建食品加工厂")).toBeInTheDocument();
    expect(screen.getByText("占地约 200 亩，预计投资 5 亿美元")).toBeInTheDocument();
    expect(screen.getByText("Siam Cement")).toBeInTheDocument();
    expect(screen.getByText("与张三跟进重叠")).toBeInTheDocument();
  });

  it("renders source_url as a clickable link with target=_blank", () => {
    render(
      <OpportunityTable
        opportunities={[O1]}
        onCustomerClick={vi.fn()}
        statusOptions={OPPORTUNITY_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    const link = screen.getByRole("link", { name: "https://example.com/news/123" });
    expect(link).toHaveAttribute("href", "https://example.com/news/123");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("shows '—' for missing source_url when not provided", () => {
    render(
      <OpportunityTable
        opportunities={[O2]}
        onCustomerClick={vi.fn()}
        statusOptions={OPPORTUNITY_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    const cell = screen.getByTestId("source-url-OPP-2");
    expect(cell.textContent).toBe("—");
  });

  it("shows '—' for missing customer name when customer_id is empty", () => {
    render(
      <OpportunityTable
        opportunities={[O2]}
        onCustomerClick={vi.fn()}
        statusOptions={OPPORTUNITY_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    const cell = screen.getByTestId("customer-OPP-2");
    expect(cell.textContent).toBe("—");
  });

  it("status column renders an InlineStatusSelect per row", () => {
    render(
      <OpportunityTable
        opportunities={[O1, O2]}
        onCustomerClick={vi.fn()}
        statusOptions={OPPORTUNITY_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    const selects = screen.getAllByTestId("status-select");
    expect(selects).toHaveLength(2);
    expect((selects[0] as HTMLSelectElement).value).toBe("待评估");
    expect((selects[1] as HTMLSelectElement).value).toBe("跟进中");
  });

  it("changing status select calls onStatusChange(id, newStatus)", () => {
    const onStatusChange = vi.fn().mockResolvedValue(undefined);
    render(
      <OpportunityTable
        opportunities={[O1]}
        onCustomerClick={vi.fn()}
        statusOptions={OPPORTUNITY_STATUS_OPTIONS}
        onStatusChange={onStatusChange}
      />,
    );
    fireEvent.change(screen.getByTestId("status-select"), { target: { value: "已转化" } });
    expect(onStatusChange).toHaveBeenCalledWith("OPP-1", "已转化");
  });

  it("calls onCustomerClick with customer_id when customer name button is clicked", () => {
    const handler = vi.fn();
    render(
      <OpportunityTable
        opportunities={[O1]}
        onCustomerClick={handler}
        statusOptions={OPPORTUNITY_STATUS_OPTIONS}
        onStatusChange={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByText("Siam Cement"));
    expect(handler).toHaveBeenCalledWith("CUST-1");
  });
});