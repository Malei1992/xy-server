import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { CustomerTable } from "@/ui/components/CustomerTable";
import type { Customer } from "@/query/types";

const fullCustomer: Customer = {
  id: "C1", created_at: "", updated_at: "",
  basic: { name: "A公司", country: "泰国", industry: "X", contacts: ["a@x.com", "b@x.com"], phones: ["+66 1", "+66 2"] },
  engagement: { status: "active" },
  prospecting: { grade: "A", source_extracted_at: "2026-06-01T03:25:42Z" },
  timeline: [],
};

const emptyContactsCustomer: Customer = {
  id: "C2", created_at: "", updated_at: "",
  basic: { name: "B公司", country: "越南", industry: "Y", contacts: "", phones: "" },
  engagement: { status: "dormant" },
  timeline: [],
};

// jsdom 不提供 navigator.clipboard，测试前补上 writeText 桩
const writeText = vi.fn().mockResolvedValue(undefined);
beforeEach(() => {
  writeText.mockClear();
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText },
    writable: true,
    configurable: true,
  });
});

describe("CustomerTable", () => {
  it("renders customer name, country, grade, contacts, phones, source_extracted_at", () => {
    render(<CustomerTable customers={[fullCustomer]} onDetail={() => {}} />);
    expect(screen.getByText("A公司")).toBeInTheDocument();
    expect(screen.getByText("泰国")).toBeInTheDocument();
    expect(screen.getByText("A")).toBeInTheDocument(); // grade chip
    expect(screen.getByText("a@x.com，b@x.com")).toBeInTheDocument();
    expect(screen.getByText("+66 1，+66 2")).toBeInTheDocument();
  });

  it("shows '无' when contacts/phones are empty string", () => {
    render(<CustomerTable customers={[emptyContactsCustomer]} onDetail={() => {}} />);
    const cells = screen.getAllByText("无");
    expect(cells.length).toBeGreaterThanOrEqual(2);
  });

  it("renders empty state when no customers", () => {
    render(<CustomerTable customers={[]} onDetail={() => {}} />);
    expect(screen.getByText(/暂无客户/)).toBeInTheDocument();
  });

  it("calls onDetail when 详情 button clicked", () => {
    const onDetail = vi.fn();
    render(<CustomerTable customers={[fullCustomer]} onDetail={onDetail} />);
    fireEvent.click(screen.getByTestId("detail-C1"));
    expect(onDetail).toHaveBeenCalledWith("C1");
  });

  it("does not call onDetail when row body clicked (only button triggers)", () => {
    const onDetail = vi.fn();
    render(<CustomerTable customers={[fullCustomer]} onDetail={onDetail} />);
    fireEvent.click(screen.getByText("A公司"));
    expect(onDetail).not.toHaveBeenCalled();
  });

  it("uses fixed table layout and exposes full text via title for hover tooltip", () => {
    render(<CustomerTable customers={[fullCustomer]} onDetail={() => {}} />);
    const table = screen.getByRole("table");
    expect((table as HTMLTableElement).style.tableLayout).toBe("fixed");
    // 数据单元格的 title 等于其文本，方便 hover 时看完整内容
    const nameCell = screen.getByText("A公司").closest("td") as HTMLTableCellElement;
    expect(nameCell.title).toBe("A公司");
    const contactsCell = screen.getByText("a@x.com，b@x.com").closest("td") as HTMLTableCellElement;
    expect(contactsCell.title).toBe("a@x.com，b@x.com");
  });

  it("copy button writes name to clipboard and shows ✅", async () => {
    render(<CustomerTable customers={[fullCustomer]} onDetail={() => {}} />);
    const copyBtn = screen.getByTestId("copy-btn");
    expect(copyBtn.getAttribute("data-copied")).toBe("false");
    fireEvent.click(copyBtn);
    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith("A公司");
    });
    await waitFor(() => {
      expect(screen.getByTestId("copy-btn").getAttribute("data-copied")).toBe("true");
    });
    expect(screen.getByText("✅")).toBeInTheDocument();
  });
});
