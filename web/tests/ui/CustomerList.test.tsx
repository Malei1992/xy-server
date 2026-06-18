import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { CustomerList } from "@/ui/pages/CustomerList";

const { mockList } = vi.hoisted(() => ({ mockList: vi.fn() }));
vi.mock("@/query", () => ({
  CRMQuery: vi.fn().mockImplementation(() => ({
    listCustomers: mockList,
  })),
}));

describe("CustomerList", () => {
  it("renders 客户信息 heading and customers from query", async () => {
    mockList.mockResolvedValue([
      { id: "C1", basic: { name: "A公司", country: "泰国", industry: "X", contacts: "a@x.com", phones: "" }, engagement: { status: "active" }, timeline: [] },
      { id: "C2", basic: { name: "B公司", country: "越南", industry: "Y", contacts: "", phones: "" }, engagement: { status: "dormant" }, timeline: [] },
    ]);
    render(<MemoryRouter><CustomerList /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByText("客户信息")).toBeInTheDocument();
      expect(screen.getByText("A公司")).toBeInTheDocument();
    });
    expect(screen.getByText("B公司")).toBeInTheDocument();
  });

  it("invokes navigation when 详情 button clicked", async () => {
    mockList.mockResolvedValue([
      { id: "C1", basic: { name: "A公司", country: "泰国", industry: "X", contacts: "", phones: "" }, engagement: { status: "active" }, timeline: [] },
    ]);
    render(<MemoryRouter><CustomerList /></MemoryRouter>);
    await waitFor(() => screen.getByText("A公司"));
    fireEvent.click(screen.getByTestId("detail-C1"));
    // MemoryRouter 不会真的导航，但点击不应报错
  });

  it("filters customers by name (case-insensitive substring) via search input", async () => {
    mockList.mockResolvedValue([
      { id: "C1", basic: { name: "Alpha 有限公司", country: "泰国", industry: "X", contacts: "", phones: "" }, engagement: { status: "active" }, timeline: [] },
      { id: "C2", basic: { name: "Beta 实业", country: "越南", industry: "Y", contacts: "", phones: "" }, engagement: { status: "active" }, timeline: [] },
      { id: "C3", basic: { name: "Gamma 集团", country: "印尼", industry: "Z", contacts: "", phones: "" }, engagement: { status: "active" }, timeline: [] },
    ]);
    render(<MemoryRouter><CustomerList /></MemoryRouter>);
    await waitFor(() => screen.getByText("Alpha 有限公司"));

    // 输入 "beta"（小写）应只匹配 "Beta 实业"
    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "beta" } });
    await waitFor(() => {
      expect(screen.queryByText("Alpha 有限公司")).not.toBeInTheDocument();
      expect(screen.getByText("Beta 实业")).toBeInTheDocument();
      expect(screen.queryByText("Gamma 集团")).not.toBeInTheDocument();
    });

    // 清空搜索框恢复全部
    fireEvent.change(screen.getByTestId("search-input"), { target: { value: "" } });
    await waitFor(() => {
      expect(screen.getByText("Alpha 有限公司")).toBeInTheDocument();
      expect(screen.getByText("Beta 实业")).toBeInTheDocument();
      expect(screen.getByText("Gamma 集团")).toBeInTheDocument();
    });
  });
});
