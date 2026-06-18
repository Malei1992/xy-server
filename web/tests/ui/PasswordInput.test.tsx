import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { PasswordInput } from "@/ui/components/PasswordInput";

// PasswordInput 行为契约:
//   - 默认 type="password"(隐藏)
//   - 右侧眼睛按钮(toggle)点击 → type 切到 "text"(显示)
//   - 再点一次 → 切回 "password"(隐藏)
//   - 受控:value / onChange,初始 value 显示在 input 上
//   - data-testid: input 是 "password-input",toggle 按钮是 "password-toggle"
//   - placeholder / autoComplete / 外部 "data-testid" 透传(注意:props.data-testid 覆盖默认)

describe("PasswordInput", () => {
  it("默认渲染:type=password + 眼睛 toggle 按钮 + 透传 placeholder/autoComplete", () => {
    render(
      <PasswordInput
        value=""
        onChange={() => {}}
        placeholder="请输入密码"
        autoComplete="current-password"
      />,
    );
    const input = screen.getByTestId("password-input") as HTMLInputElement;
    expect(input.type).toBe("password");
    expect(input.placeholder).toBe("请输入密码");
    expect(input.autocomplete).toBe("current-password");
    expect(screen.getByTestId("password-toggle")).toBeInTheDocument();
  });

  it("受控:value 显示在 input 上,输入触发 onChange(且只触发一次)", () => {
    const onChange = vi.fn();
    render(<PasswordInput value="initial" onChange={onChange} />);
    const input = screen.getByTestId("password-input") as HTMLInputElement;
    expect(input.value).toBe("initial");
    fireEvent.change(input, { target: { value: "new" } });
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith("new");
  });

  it("点击 toggle:type 在 password 和 text 之间切换", () => {
    render(<PasswordInput value="x" onChange={() => {}} />);
    const input = screen.getByTestId("password-input") as HTMLInputElement;
    const toggle = screen.getByTestId("password-toggle");

    expect(input.type).toBe("password");
    fireEvent.click(toggle);
    expect(input.type).toBe("text");
    fireEvent.click(toggle);
    expect(input.type).toBe("password");
  });

  it("toggle 内部渲染 SVG(无 emoji 字符),aria-label 随状态切换", () => {
    render(<PasswordInput value="x" onChange={() => {}} />);
    const toggle = screen.getByTestId("password-toggle");
    // 初始(隐藏态):aria-label 是 "显示密码",且 toggle 内含一个 svg
    expect(toggle).toHaveAttribute("aria-label", "显示密码");
    expect(toggle.querySelector("svg")).toBeInTheDocument();
    // 不应该出现旧的 emoji 字符 👁 / 🙈
    expect(toggle.textContent).not.toMatch(/[👁🙈]/);

    fireEvent.click(toggle);
    // 切到显示态:aria-label 切到 "隐藏密码"
    expect(toggle).toHaveAttribute("aria-label", "隐藏密码");
    expect(toggle.querySelector("svg")).toBeInTheDocument();
    expect(toggle.textContent).not.toMatch(/[👁🙈]/);

    fireEvent.click(toggle);
    // 切回隐藏态
    expect(toggle).toHaveAttribute("aria-label", "显示密码");
  });

  it("外部传入 data-testid 覆盖默认的 password-input", () => {
    render(
      <PasswordInput
        value=""
        onChange={() => {}}
        data-testid="custom-input"
      />,
    );
    expect(screen.getByTestId("custom-input")).toBeInTheDocument();
    // 默认 testid 不应同时存在
    expect(screen.queryByTestId("password-input")).not.toBeInTheDocument();
    // 但 toggle 仍用默认 testid
    expect(screen.getByTestId("password-toggle")).toBeInTheDocument();
  });

  it("value 变化时 input 同步显示(标准受控语义)", () => {
    const { rerender } = render(<PasswordInput value="a" onChange={() => {}} />);
    expect((screen.getByTestId("password-input") as HTMLInputElement).value).toBe("a");
    rerender(<PasswordInput value="b" onChange={() => {}} />);
    expect((screen.getByTestId("password-input") as HTMLInputElement).value).toBe("b");
  });
});
