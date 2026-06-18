import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { Users } from "@/ui/pages/Users";

// mock query/users 整个模块
const { mockListUsers, mockCreateUser, mockChangePassword, mockDeleteUser } = vi.hoisted(() => ({
  mockListUsers: vi.fn(),
  mockCreateUser: vi.fn(),
  mockChangePassword: vi.fn(),
  mockDeleteUser: vi.fn(),
}));
vi.mock("@/query/users", () => ({
  listUsers: mockListUsers,
  createUser: mockCreateUser,
  changePassword: mockChangePassword,
  deleteUser: mockDeleteUser,
}));

beforeEach(() => {
  mockListUsers.mockReset();
  mockCreateUser.mockReset();
  mockChangePassword.mockReset();
  mockDeleteUser.mockReset();
  localStorage.clear();
});

function renderUsers(initialPath = "/users") {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/users" element={<Users />} />
        <Route path="/login" element={<div>LOGIN_PAGE</div>} />
        <Route path="/settings" element={<div>SETTINGS_PAGE</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("Users - 列表渲染", () => {
  it("进入页面立即调 listUsers,展示返回的账号行 + 顶部按钮", async () => {
    mockListUsers.mockResolvedValue([{ account: "admin" }, { account: "alice" }]);
    renderUsers();
    await waitFor(() => {
      expect(mockListUsers).toHaveBeenCalledTimes(1);
    });
    expect(await screen.findByTestId("user-row-admin")).toHaveTextContent("admin");
    expect(screen.getByTestId("user-row-alice")).toHaveTextContent("alice");
    // 顶部「新增用户」按钮在
    expect(screen.getByTestId("add-user-button")).toBeInTheDocument();
    // 退出登录按钮已迁移到 Layout,Users 页不再渲染
    expect(screen.queryByTestId("logout-button")).not.toBeInTheDocument();
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

describe("Users - 新增用户弹窗", () => {
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

describe("Users - 修改密码弹窗", () => {
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

describe("Users - 删除用户", () => {
  it("当前账号那行不渲染删除按钮(也不显示(我)标记以外的差异)", async () => {
    // 设置登录态为 admin
    localStorage.setItem("crm_logged_in", "true");
    localStorage.setItem("crm_account", "admin");
    mockListUsers.mockResolvedValue([{ account: "admin" }, { account: "alice" }]);
    renderUsers();
    await screen.findByTestId("user-row-admin");
    // admin 行没有删除按钮
    expect(screen.queryByTestId("delete-user-admin")).not.toBeInTheDocument();
    // alice 行有
    expect(screen.getByTestId("delete-user-alice")).toBeInTheDocument();
  });

  it("未登录态(getLoggedInAccount 返回 null)时所有行都渲染删除按钮", async () => {
    // 不设 localStorage
    mockListUsers.mockResolvedValue([{ account: "admin" }, { account: "alice" }]);
    renderUsers();
    await screen.findByTestId("user-row-admin");
    expect(screen.getByTestId("delete-user-admin")).toBeInTheDocument();
    expect(screen.getByTestId("delete-user-alice")).toBeInTheDocument();
  });

  it("点删除按钮 → 出现 inline 确认(确认/取消),点取消确认消失 + 不发请求", async () => {
    mockListUsers.mockResolvedValue([{ account: "admin" }, { account: "alice" }]);
    renderUsers();
    await screen.findByTestId("user-row-alice");

    fireEvent.click(screen.getByTestId("delete-user-alice"));
    // inline 确认 UI 出现
    expect(screen.getByTestId("delete-confirm-alice")).toBeInTheDocument();
    expect(screen.getByTestId("delete-cancel-alice")).toBeInTheDocument();
    // 确认期间,原删除按钮隐藏(被「确认」取代)
    expect(screen.queryByTestId("delete-user-alice")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("delete-cancel-alice"));
    // 确认 UI 消失,原删除按钮回来
    await waitFor(() => {
      expect(screen.queryByTestId("delete-confirm-alice")).not.toBeInTheDocument();
    });
    expect(screen.getByTestId("delete-user-alice")).toBeInTheDocument();
    // 没发 deleteUser 请求
    expect(mockDeleteUser).not.toHaveBeenCalled();
  });

  it("点确认 → 调 deleteUser + 列表刷新(成功后该行消失)", async () => {
    // 第一次 listUsers 返回 2 个;删除后第二次 listUsers 返回 1 个
    mockListUsers
      .mockResolvedValueOnce([{ account: "admin" }, { account: "alice" }])
      .mockResolvedValueOnce([{ account: "admin" }]);
    mockDeleteUser.mockResolvedValue(undefined);
    renderUsers();
    await screen.findByTestId("user-row-alice");

    fireEvent.click(screen.getByTestId("delete-user-alice"));
    fireEvent.click(screen.getByTestId("delete-confirm-alice"));

    await waitFor(() => {
      expect(mockDeleteUser).toHaveBeenCalledWith("alice");
    });
    // 列表刷新后 alice 行消失
    await waitFor(() => {
      expect(screen.queryByTestId("user-row-alice")).not.toBeInTheDocument();
    });
    expect(screen.getByTestId("user-row-admin")).toBeInTheDocument();
    // 只调了 2 次 listUsers(初次 + 刷新)
    expect(mockListUsers).toHaveBeenCalledTimes(2);
  });

  it("删除失败(mock 抛错)→ 行内错误显示,该行不消失,不发新的 listUsers", async () => {
    mockListUsers.mockResolvedValue([{ account: "admin" }, { account: "alice" }]);
    mockDeleteUser.mockRejectedValue(new Error("账号不存在"));
    renderUsers();
    await screen.findByTestId("user-row-alice");

    fireEvent.click(screen.getByTestId("delete-user-alice"));
    fireEvent.click(screen.getByTestId("delete-confirm-alice"));

    await waitFor(() => {
      expect(screen.getByTestId("delete-error-alice")).toHaveTextContent("账号不存在");
    });
    // 行还在
    expect(screen.getByTestId("user-row-alice")).toBeInTheDocument();
    // 确认 UI 已清除,可以重新点
    expect(screen.getByTestId("delete-user-alice")).toBeInTheDocument();
    // 只调了 1 次 listUsers(初次),失败没刷新
    expect(mockListUsers).toHaveBeenCalledTimes(1);
  });

  it("删除错误后,再点删除按钮(确认状态清空)→ 旧错误消失", async () => {
    mockListUsers.mockResolvedValue([{ account: "alice" }]);
    mockDeleteUser.mockRejectedValue(new Error("boom"));
    renderUsers();
    await screen.findByTestId("user-row-alice");

    fireEvent.click(screen.getByTestId("delete-user-alice"));
    fireEvent.click(screen.getByTestId("delete-confirm-alice"));
    await screen.findByTestId("delete-error-alice");

    // 再点删除按钮(应该清错误,重新进入确认)
    fireEvent.click(screen.getByTestId("delete-user-alice"));
    expect(screen.queryByTestId("delete-error-alice")).not.toBeInTheDocument();
    expect(screen.getByTestId("delete-confirm-alice")).toBeInTheDocument();
  });
});
