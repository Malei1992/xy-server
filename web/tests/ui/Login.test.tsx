import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { Login } from "@/ui/pages/Login";
import { CRMFetchError } from "@/query/loader";

// mock query/users 整个模块,让 Login 不真发 fetch
const { mockLogin } = vi.hoisted(() => ({ mockLogin: vi.fn() }));
vi.mock("@/query/users", () => ({
  login: mockLogin,
}));

beforeEach(() => {
  mockLogin.mockReset();
  localStorage.clear();
});

function renderLogin(initialPath = "/login") {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/" element={<div>HOME</div>} />
        <Route path="/customers" element={<div>CUSTOMERS</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("Login", () => {
  it("渲染:账号输入 + 密码输入(默认 type=password) + 登录按钮", () => {
    renderLogin();
    // 账号用普通 input
    expect(screen.getByTestId("account-input")).toBeInTheDocument();
    // 密码用 PasswordInput,默认 testid
    const pwd = screen.getByTestId("password-input") as HTMLInputElement;
    expect(pwd.type).toBe("password");
    // 登录按钮
    expect(screen.getByTestId("login-submit")).toBeInTheDocument();
  });

  it("账号/密码都为空时:登录按钮 disabled", () => {
    renderLogin();
    const btn = screen.getByTestId("login-submit") as HTMLButtonElement;
    expect(btn).toBeDisabled();
  });

  it("只填账号:按钮仍 disabled", () => {
    renderLogin();
    fireEvent.change(screen.getByTestId("account-input"), { target: { value: "admin" } });
    expect(screen.getByTestId("login-submit")).toBeDisabled();
  });

  it("只填密码:按钮仍 disabled", () => {
    renderLogin();
    fireEvent.change(screen.getByTestId("password-input"), { target: { value: "x" } });
    expect(screen.getByTestId("login-submit")).toBeDisabled();
  });

  it("两个都填:按钮 enabled", () => {
    renderLogin();
    fireEvent.change(screen.getByTestId("account-input"), { target: { value: "admin" } });
    fireEvent.change(screen.getByTestId("password-input"), { target: { value: "x" } });
    expect(screen.getByTestId("login-submit")).not.toBeDisabled();
  });

  it("成功登录:写 localStorage + 跳到 /", async () => {
    mockLogin.mockResolvedValue({ ok: true, account: "admin" });
    renderLogin();
    fireEvent.change(screen.getByTestId("account-input"), { target: { value: "admin" } });
    fireEvent.change(screen.getByTestId("password-input"), { target: { value: "admin123" } });
    fireEvent.click(screen.getByTestId("login-submit"));

    await waitFor(() => {
      expect(mockLogin).toHaveBeenCalledWith({ account: "admin", password: "admin123" });
    });
    await waitFor(() => {
      expect(localStorage.getItem("crm_logged_in")).toBe("true");
      expect(localStorage.getItem("crm_account")).toBe("admin");
    });
    await waitFor(() => {
      expect(screen.getByText("HOME")).toBeInTheDocument();
    });
  });

  it("401 (账号或密码错):展示后端 error 文案,按钮恢复可点", async () => {
    mockLogin.mockRejectedValue(
      new CRMFetchError("/api/login", 401, "账号或密码错误"),
    );
    renderLogin();
    fireEvent.change(screen.getByTestId("account-input"), { target: { value: "admin" } });
    fireEvent.change(screen.getByTestId("password-input"), { target: { value: "wrong" } });
    fireEvent.click(screen.getByTestId("login-submit"));

    await waitFor(() => {
      expect(screen.getByTestId("login-error")).toHaveTextContent("账号或密码错误");
    });
    // 不会跳走
    expect(screen.queryByText("HOME")).not.toBeInTheDocument();
    // 按钮恢复可点
    expect(screen.getByTestId("login-submit")).not.toBeDisabled();
  });

  it("404 (账号不存在):展示后端 error 文案", async () => {
    mockLogin.mockRejectedValue(
      new CRMFetchError("/api/login", 404, "账号不存在"),
    );
    renderLogin();
    fireEvent.change(screen.getByTestId("account-input"), { target: { value: "nobody" } });
    fireEvent.change(screen.getByTestId("password-input"), { target: { value: "x" } });
    fireEvent.click(screen.getByTestId("login-submit"));

    await waitFor(() => {
      expect(screen.getByTestId("login-error")).toHaveTextContent("账号不存在");
    });
  });
});
