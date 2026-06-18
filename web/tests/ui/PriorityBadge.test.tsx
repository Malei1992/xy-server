import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { PriorityBadge } from "@/ui/components/PriorityBadge";

describe("PriorityBadge", () => {
  it("renders P0 with red background", () => {
    render(<PriorityBadge priority="P0" />);
    const badge = screen.getByTestId("priority-P0");
    expect(badge.textContent).toBe("P0");
    expect(badge.style.background).toContain("220, 38, 38"); // #dc2626 → rgb(220, 38, 38)
    expect(badge.style.borderRadius).toBe("50%");
  });

  it("renders P1 with yellow background", () => {
    render(<PriorityBadge priority="P1" />);
    const badge = screen.getByTestId("priority-P1");
    expect(badge.textContent).toBe("P1");
    expect(badge.style.background).toContain("250, 204, 21"); // #facc15
  });

  it("renders P2 with green background", () => {
    render(<PriorityBadge priority="P2" />);
    const badge = screen.getByTestId("priority-P2");
    expect(badge.textContent).toBe("P2");
    expect(badge.style.background).toContain("22, 163, 74"); // #16a34a
  });

  it("renders P3 with gray background", () => {
    render(<PriorityBadge priority="P3" />);
    const badge = screen.getByTestId("priority-P3");
    expect(badge.textContent).toBe("P3");
    expect(badge.style.background).toContain("156, 163, 175"); // #9ca3af
  });

  it("renders as a circle (borderRadius 50%)", () => {
    render(<PriorityBadge priority="P0" />);
    const badge = screen.getByTestId("priority-P0");
    expect(badge.style.borderRadius).toBe("50%");
    expect(badge.style.width).toBe("22px");
    expect(badge.style.height).toBe("22px");
  });

  it("has accessible label with priority", () => {
    render(<PriorityBadge priority="P0" />);
    expect(screen.getByLabelText("任务等级 P0")).toBeInTheDocument();
  });
});
