import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { Layout } from "@/ui/components/Layout";

beforeEach(() => {
  localStorage.clear();
});

function renderLayout(initialPath = "/customers") {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route element={<Layout />}>
          <Route path="customers" element={<div>CUSTOMERS_PAGE</div>} />
        </Route>
        <Route path="/login" element={<div>LOGIN_PAGE</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("Layout - header 退出登录按钮", () => {
  it("header 右侧渲染「退出登录」按钮", () => {
    renderLayout();
    const btn = screen.getByTestId("logout-button");
    expect(btn).toBeInTheDocument();
    expect(btn).toHaveTextContent("退出登录");
  });

  it("点击退出登录:清 localStorage + 跳 /login", () => {
    localStorage.setItem("crm_logged_in", "true");
    localStorage.setItem("crm_account", "admin");
    renderLayout();

    fireEvent.click(screen.getByTestId("logout-button"));

    // 真实 clearLogin 清掉两个 localStorage key
    expect(localStorage.getItem("crm_logged_in")).toBeNull();
    expect(localStorage.getItem("crm_account")).toBeNull();
    // useNavigate 跳转到 /login 路由
    expect(screen.getByText("LOGIN_PAGE")).toBeInTheDocument();
  });

  it("header 左侧仍展示 logo '星月'", () => {
    renderLayout();
    expect(screen.getByText("星月")).toBeInTheDocument();
  });

  it("sidebar 仍包含 6 个 NavLink(商机/客户/代办/公开/系统设置/用户管理)", () => {
    renderLayout();
    expect(screen.getByText("商机信息")).toBeInTheDocument();
    expect(screen.getByText("客户信息")).toBeInTheDocument();
    expect(screen.getByText("代办任务")).toBeInTheDocument();
    expect(screen.getByText("公开信息")).toBeInTheDocument();
    expect(screen.getByText("系统设置")).toBeInTheDocument();
    expect(screen.getByText("用户管理")).toBeInTheDocument();
  });

  it("header 同时显示当前登录账号(可选增强)", () => {
    localStorage.setItem("crm_account", "admin");
    renderLayout();
    // header 区域内展示当前账号 'admin'
    const header = screen.getByTestId("app-header");
    expect(header).toHaveTextContent("admin");
  });
});
