import { describe, it, expect, vi, afterEach } from "vitest";
import { useState } from "react";
import { act, render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { InlineStatusSelect } from "@/ui/components/InlineStatusSelect";

// 状态选项样例(泛型 T = string)
const OPTIONS = [
  { value: "active", label: "活跃" },
  { value: "pending", label: "待处理" },
  { value: "closed", label: "关闭" },
] as const;

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("InlineStatusSelect", () => {
  it("renders a select with the current value", () => {
    render(
      <InlineStatusSelect
        value="active"
        options={OPTIONS}
        onChange={vi.fn()}
      />,
    );
    const select = screen.getByTestId("status-select") as HTMLSelectElement;
    expect(select).toBeInTheDocument();
    expect(select.value).toBe("active");
  });

  it("renders all options as <option> elements", () => {
    render(
      <InlineStatusSelect
        value="active"
        options={OPTIONS}
        onChange={vi.fn()}
      />,
    );
    const select = screen.getByTestId("status-select");
    expect(select.querySelectorAll("option")).toHaveLength(3);
    expect(screen.getByRole("option", { name: "活跃" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "待处理" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "关闭" })).toBeInTheDocument();
  });

  it("changing the select calls onChange with the new value", async () => {
    const onChange = vi.fn().mockResolvedValue(undefined);
    render(
      <InlineStatusSelect
        value="active"
        options={OPTIONS}
        onChange={onChange}
      />,
    );
    await act(async () => {
      fireEvent.change(screen.getByTestId("status-select"), { target: { value: "pending" } });
    });
    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith("pending");
    });
  });

  it("changing to the same value does NOT call onChange", () => {
    const onChange = vi.fn().mockResolvedValue(undefined);
    render(
      <InlineStatusSelect
        value="active"
        options={OPTIONS}
        onChange={onChange}
      />,
    );
    fireEvent.change(screen.getByTestId("status-select"), { target: { value: "active" } });
    expect(onChange).not.toHaveBeenCalled();
  });

  it("on successful change: select shows new value, no error, no saving", async () => {
    const onChange = vi.fn().mockResolvedValue(undefined);
    // 用受控 wrapper:onChange 成功后改外部 value,模拟 list page 调完 updateXxxStatus 后 setState
    function Wrapper() {
      const [v, setV] = useState<typeof OPTIONS[number]["value"]>("active");
      return (
        <InlineStatusSelect
          value={v}
          options={OPTIONS}
          onChange={async (nv) => {
            await onChange(nv);
            setV(nv);
          }}
        />
      );
    }
    render(<Wrapper />);
    await act(async () => {
      fireEvent.change(screen.getByTestId("status-select"), { target: { value: "pending" } });
    });
    await waitFor(() => {
      const select = screen.getByTestId("status-select") as HTMLSelectElement;
      expect(select.value).toBe("pending");
      expect(select).not.toBeDisabled();
      expect(screen.queryByTestId("status-select-error")).not.toBeInTheDocument();
    });
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("on failed change: select reverts to original value and shows error", async () => {
    const onChange = vi.fn().mockRejectedValue(new Error("HTTP 400 状态非法"));
    render(
      <InlineStatusSelect
        value="active"
        options={OPTIONS}
        onChange={onChange}
      />,
    );
    await act(async () => {
      fireEvent.change(screen.getByTestId("status-select"), { target: { value: "pending" } });
    });
    await waitFor(() => {
      expect(screen.getByTestId("status-select-error")).toHaveTextContent(/状态非法/);
    });
    // select 回到原值
    const select = screen.getByTestId("status-select") as HTMLSelectElement;
    expect(select.value).toBe("active");
    expect(select).not.toBeDisabled();
  });

  it("error auto-dismisses after 5 seconds (vi.useFakeTimers + advanceTimersByTime)", async () => {
    const onChange = vi.fn().mockRejectedValue(new Error("网络错误"));
    // 一开始就 fakeTimers,这样组件内部 setTimeout 走 fake
    vi.useFakeTimers();
    render(
      <InlineStatusSelect
        value="active"
        options={OPTIONS}
        onChange={onChange}
      />,
    );
    await act(async () => {
      fireEvent.change(screen.getByTestId("status-select"), { target: { value: "pending" } });
      // 让 onChange 抛的 promise + setError 走完
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(screen.getByTestId("status-select-error")).toBeInTheDocument();

    // 推进 5s,触发 setError(null) 的 setTimeout 回调
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });
    expect(screen.queryByTestId("status-select-error")).not.toBeInTheDocument();
  });

  it("during in-flight save: select is disabled", async () => {
    let resolveSave: () => void = () => undefined;
    const onChange = vi.fn().mockImplementation(
      () => new Promise<void>((resolve) => { resolveSave = resolve; }),
    );
    render(
      <InlineStatusSelect
        value="active"
        options={OPTIONS}
        onChange={onChange}
      />,
    );
    await act(async () => {
      fireEvent.change(screen.getByTestId("status-select"), { target: { value: "pending" } });
    });
    await waitFor(() => {
      const select = screen.getByTestId("status-select") as HTMLSelectElement;
      expect(select).toBeDisabled();
    });
    await act(async () => { resolveSave(); });
  });

  it("during in-flight save: select shows the pending value (what user picked)", async () => {
    let resolveSave: () => void = () => undefined;
    const onChange = vi.fn().mockImplementation(
      () => new Promise<void>((resolve) => { resolveSave = resolve; }),
    );
    render(
      <InlineStatusSelect
        value="active"
        options={OPTIONS}
        onChange={onChange}
      />,
    );
    await act(async () => {
      fireEvent.change(screen.getByTestId("status-select"), { target: { value: "pending" } });
    });
    await waitFor(() => {
      const select = screen.getByTestId("status-select") as HTMLSelectElement;
      expect(select.value).toBe("pending");
      expect(select).toBeDisabled();
    });
    await act(async () => { resolveSave(); });
  });

  it("concurrent change during in-flight save is ignored", async () => {
    let resolveSave: () => void = () => undefined;
    const onChange = vi.fn().mockImplementation(
      () => new Promise<void>((resolve) => { resolveSave = resolve; }),
    );
    render(
      <InlineStatusSelect
        value="active"
        options={OPTIONS}
        onChange={onChange}
      />,
    );
    // 第一次 change:in-flight
    await act(async () => {
      fireEvent.change(screen.getByTestId("status-select"), { target: { value: "pending" } });
    });
    await waitFor(() => {
      expect(screen.getByTestId("status-select")).toBeDisabled();
    });
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith("pending");

    // 第二次 change:disabled 状态下,浏览器通常不会发 change 事件,InlineStatusSelect
    // 内部用 saving state 守门,所以也不会调 onChange
    await act(async () => {
      fireEvent.change(screen.getByTestId("status-select"), { target: { value: "closed" } });
    });
    // 仍只调用 1 次,saving 守门生效
    expect(onChange).toHaveBeenCalledTimes(1);

    await act(async () => { resolveSave(); });
  });

  it("uses Error.message when onChange rejects with a non-Error throwable", async () => {
    const onChange = vi.fn().mockRejectedValue("纯字符串错误");
    render(
      <InlineStatusSelect
        value="active"
        options={OPTIONS}
        onChange={onChange}
      />,
    );
    await act(async () => {
      fireEvent.change(screen.getByTestId("status-select"), { target: { value: "pending" } });
    });
    await waitFor(() => {
      expect(screen.getByTestId("status-select-error")).toHaveTextContent(/纯字符串错误/);
    });
  });

  it("changing value clears previous error", async () => {
    const onChange = vi.fn()
      .mockRejectedValueOnce(new Error("第一次失败"))
      .mockResolvedValueOnce(undefined);
    render(
      <InlineStatusSelect
        value="active"
        options={OPTIONS}
        onChange={onChange}
      />,
    );
    await act(async () => {
      fireEvent.change(screen.getByTestId("status-select"), { target: { value: "pending" } });
    });
    await waitFor(() => {
      expect(screen.getByTestId("status-select-error")).toHaveTextContent(/第一次失败/);
    });
    // 改值(此时 saving=false,因为第一次已经结束)
    await act(async () => {
      fireEvent.change(screen.getByTestId("status-select"), { target: { value: "closed" } });
    });
    await waitFor(() => {
      expect(screen.queryByTestId("status-select-error")).not.toBeInTheDocument();
    });
    expect(onChange).toHaveBeenCalledTimes(2);
    expect(onChange).toHaveBeenLastCalledWith("closed");
  });

});