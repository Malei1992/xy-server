import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Workbench } from "@/ui/pages/Workbench";

vi.mock("@/query", () => ({
  CRMQuery: vi.fn().mockImplementation(() => ({
    getIndex: vi.fn().mockResolvedValue({
      customers: ["C1", "C2", "C3"],
      by_status: { active: ["C1", "C3"], dormant: ["C2"] },
      by_intent: { unknown: ["C1"], "": ["C2"], A: ["C3"] },
      by_country: { 泰国: ["C1", "C2", "C3"] },
    }),
    listCustomers: vi.fn().mockResolvedValue([
      { id: "C1", basic: { name: "A", country: "泰国", industry: "X", contacts: "", phones: "" }, engagement: { status: "active" }, timeline: [{ event: "created", by: "P", at: "2026-06-01T00:00:00Z" }] },
      { id: "C2", basic: { name: "B", country: "泰国", industry: "Y", contacts: "", phones: "" }, engagement: { status: "dormant" }, timeline: [] },
      { id: "C3", basic: { name: "C", country: "泰国", industry: "Z", contacts: "", phones: "" }, engagement: { status: "active" }, timeline: [] },
    ]),
  })),
}));

describe("Workbench", () => {
  it("renders summary cards with counts", async () => {
    render(<MemoryRouter><Workbench /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByText("3")).toBeInTheDocument(); // 客户总数
    });
    expect(screen.getByText(/活跃/)).toBeInTheDocument();
  });
});
