import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { Users } from "@/ui/pages/Settings/Users";

// mock query/users 整个模块
const { mockListUsers, mockCreateUser, mockChangePassword } = vi.hoisted(() => ({
  mockListUsers: vi.fn(),
  mockCreateUser: vi.fn(),
  mockChangePassword: vi.fn(),
}));
vi.mock("@/query/users", () => ({
  listUsers: mockListUsers,
  createUser: mockCreateUser,
  changePassword: mockChangePassword,
}));

beforeEach(() => {
  mockListUsers.mockReset();
  mockCreateUser.mockReset();
  mockChangePassword.mockReset();
  localStorage.clear();
});

function renderUsers(initialPath = "/settings/users") {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/settings/users" element={<Users />} />
        <Route path="/login" element={<div>LOGIN_PAGE</div>} />
        <Route path="/settings" element={<div>SETTINGS_PAGE</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("Settings/Users - 列表渲染", () => {
  it("进入页面立即调 listUsers,展示返回的账号行 + 顶部按钮", async () => {
    mockListUsers.mockResolvedValue([{ account: "admin" }, { account: "alice" }]);
    renderUsers();
    await waitFor(() => {
      expect(mockListUsers).toHaveBeenCalledTimes(1);
    });
    expect(await screen.findByTestId("user-row-admin")).toHaveTextContent("admin");
    expect(screen.getByTestId("user-row-alice")).toHaveTextContent("alice");
    // 顶部按钮都在
    expect(screen.getByTestId("add-user-button")).toBeInTheDocument();
    expect(screen.getByTestId("logout-button")).toBeInTheDocument();
  });

  it("listUsers 失败:展示错误 + 不渲染表格", async () => {
    mockListUsers.mockRejectedValue(new Error("boom"));
    renderUsers();
    await waitFor(() => {
      expect(screen.getByTestId("users-error")).toHaveTextContent("boom");
    });
    // 没有 row
    expect(screen.queryByTestId(/^user-row-/)).not.toBeInTheDocument();
  });

  it("空列表:显示空状态文案", async () => {
    mockListUsers.mockResolvedValue([]);
    renderUsers();
    await waitFor(() => {
      expect(screen.getByTestId("users-empty")).toBeInTheDocument();
    });
  });
});

describe("Settings/Users - 退出登录", () => {
  it("点退出:清 localStorage + 跳 /login", async () => {
    localStorage.setItem("crm_logged_in", "true");
    localStorage.setItem("crm_account", "admin");
    mockListUsers.mockResolvedValue([{ account: "admin" }]);
    renderUsers();
    await screen.findByTestId("user-row-admin");

    fireEvent.click(screen.getByTestId("logout-button"));

    await waitFor(() => {
      expect(localStorage.getItem("crm_logged_in")).toBeNull();
      expect(localStorage.getItem("crm_account")).toBeNull();
      expect(screen.getByText("LOGIN_PAGE")).toBeInTheDocument();
    });
  });
});

describe("Settings/Users - 新增用户弹窗", () => {
  it("点新增:弹窗出现,提交后调 createUser,成功后列表刷新且弹窗关闭", async () => {
    mockListUsers.mockResolvedValue([{ account: "admin" }]);
    mockCreateUser.mockResolvedValue({ ok: true, account: "alice" });
    // 创建后再列一次返回带 alice
    mockListUsers.mockResolvedValueOnce([{ account: "admin" }])
      .mockResolvedValueOnce([{ account: "admin" }, { account: "alice" }]);
    renderUsers();
    await screen.findByTestId("user-row-admin");

    fireEvent.click(screen.getByTestId("add-user-button"));
    // 弹窗出现
    expect(screen.getByTestId("add-user-modal")).toBeInTheDocument();
    // 初始提交按钮 disabled(账号密码都空)
    expect(screen.getByTestId("add-user-submit")).toBeDisabled();

    fireEvent.change(screen.getByTestId("add-user-account"), { target: { value: "alice" } });
    fireEvent.change(screen.getByTestId("add-user-password"), { target: { value: "pw123" } });
    expect(screen.getByTestId("add-user-submit")).not.toBeDisabled();

    fireEvent.click(screen.getByTestId("add-user-submit"));

    await waitFor(() => {
      expect(mockCreateUser).toHaveBeenCalledWith({ account: "alice", password: "pw123" });
    });
    // 弹窗关闭
    await waitFor(() => {
      expect(screen.queryByTestId("add-user-modal")).not.toBeInTheDocument();
    });
    // 列表刷新后能看到新行
    expect(await screen.findByTestId("user-row-alice")).toBeInTheDocument();
  });

  it("点取消:弹窗关闭,不发请求", async () => {
    mockListUsers.mockResolvedValue([]);
    renderUsers();
    await waitFor(() => screen.getByTestId("users-empty"));

    fireEvent.click(screen.getByTestId("add-user-button"));
    expect(screen.getByTestId("add-user-modal")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("add-user-cancel"));
    expect(screen.queryByTestId("add-user-modal")).not.toBeInTheDocument();
    expect(mockCreateUser).not.toHaveBeenCalled();
  });

  it("createUser 失败:弹窗内展示错误,弹窗不关", async () => {
    mockListUsers.mockResolvedValue([]);
    mockCreateUser.mockRejectedValue(new Error("账号已存在"));
    renderUsers();
    await waitFor(() => screen.getByTestId("users-empty"));

    fireEvent.click(screen.getByTestId("add-user-button"));
    fireEvent.change(screen.getByTestId("add-user-account"), { target: { value: "alice" } });
    fireEvent.change(screen.getByTestId("add-user-password"), { target: { value: "pw123" } });
    fireEvent.click(screen.getByTestId("add-user-submit"));

    await waitFor(() => {
      expect(screen.getByTestId("add-user-error")).toHaveTextContent("账号已存在");
    });
    // 弹窗还在
    expect(screen.getByTestId("add-user-modal")).toBeInTheDocument();
  });
});

describe("Settings/Users - 修改密码弹窗", () => {
  it("点某行的修改密码:弹窗出现,新密码两次一致 + 旧密码非空 → 调 changePassword", async () => {
    mockListUsers.mockResolvedValue([{ account: "admin" }]);
    mockChangePassword.mockResolvedValue({ ok: true });
    renderUsers();
    await screen.findByTestId("user-row-admin");

    fireEvent.click(screen.getByTestId("change-pw-button-admin"));
    expect(screen.getByTestId("change-pw-modal")).toBeInTheDocument();
    // 初始 submit disabled(三栏都空)
    expect(screen.getByTestId("change-pw-submit")).toBeDisabled();

    fireEvent.change(screen.getByTestId("change-pw-old"), { target: { value: "old" } });
    // 新密码和确认密码不一致 → 仍 disabled
    fireEvent.change(screen.getByTestId("change-pw-new"), { target: { value: "new1" } });
    fireEvent.change(screen.getByTestId("change-pw-confirm"), { target: { value: "newX" } });
    expect(screen.getByTestId("change-pw-submit")).toBeDisabled();
    // 不一致错误展示
    expect(screen.getByTestId("change-pw-mismatch")).toBeInTheDocument();

    // 改成一致
    fireEvent.change(screen.getByTestId("change-pw-confirm"), { target: { value: "new1" } });
    expect(screen.getByTestId("change-pw-submit")).not.toBeDisabled();

    fireEvent.click(screen.getByTestId("change-pw-submit"));

    await waitFor(() => {
      expect(mockChangePassword).toHaveBeenCalledWith("admin", {
        oldPassword: "old",
        newPassword: "new1",
        confirmNewPassword: "new1",
      });
    });
    // 成功后弹窗关闭
    await waitFor(() => {
      expect(screen.queryByTestId("change-pw-modal")).not.toBeInTheDocument();
    });
  });

  it("changePassword 失败(旧密码错):弹窗内展示错误,弹窗不关", async () => {
    mockListUsers.mockResolvedValue([{ account: "admin" }]);
    mockChangePassword.mockRejectedValue(new Error("旧密码错误"));
    renderUsers();
    await screen.findByTestId("user-row-admin");

    fireEvent.click(screen.getByTestId("change-pw-button-admin"));
    fireEvent.change(screen.getByTestId("change-pw-old"), { target: { value: "wrong" } });
    fireEvent.change(screen.getByTestId("change-pw-new"), { target: { value: "new1" } });
    fireEvent.change(screen.getByTestId("change-pw-confirm"), { target: { value: "new1" } });
    fireEvent.click(screen.getByTestId("change-pw-submit"));

    await waitFor(() => {
      expect(screen.getByTestId("change-pw-error")).toHaveTextContent("旧密码错误");
    });
    expect(screen.getByTestId("change-pw-modal")).toBeInTheDocument();
  });

  it("点取消:弹窗关闭,不发请求", async () => {
    mockListUsers.mockResolvedValue([{ account: "admin" }]);
    renderUsers();
    await screen.findByTestId("user-row-admin");

    fireEvent.click(screen.getByTestId("change-pw-button-admin"));
    expect(screen.getByTestId("change-pw-modal")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("change-pw-cancel"));
    expect(screen.queryByTestId("change-pw-modal")).not.toBeInTheDocument();
    expect(mockChangePassword).not.toHaveBeenCalled();
  });
});
