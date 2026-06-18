import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StatusBadge } from "../../src/ui/components/StatusBadge";

describe("StatusBadge", () => {
  it.each([
    ["active", "活跃"],
    ["dormant", "休眠"],
    ["closed", "关闭"],
    ["invalid", "无效"],
    ["needs_manual_review", "待人工"],
  ] as const)("renders %s as %s", (status, label) => {
    render(<StatusBadge status={status} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });
});
