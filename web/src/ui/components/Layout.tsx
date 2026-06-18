import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { clearLogin, getLoggedInAccount } from "@/ui/auth";

// 全局布局：顶栏（星月 logo + 退出登录）+ 左侧导航 + 主内容区
export function Layout() {
  return (
    <div style={{
      display: "grid",
      gridTemplateRows: "56px 1fr",
      gridTemplateColumns: "200px 1fr",
      height: "100vh",
    }}>
      <header
        data-testid="app-header"
        style={{
          gridColumn: "1 / 3",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "0 24px",
          borderBottom: "1px solid var(--border)",
          background: "white",
        }}
      >
        <Logo />
        <HeaderRight />
      </header>
      <aside style={{
        borderRight: "1px solid var(--border)",
        background: "white",
        padding: "16px 0",
      }}>
        <Sidebar />
      </aside>
      <main style={{ overflow: "auto" }}>
        <Outlet />
      </main>
    </div>
  );
}

// header 右侧:当前登录账号 + 退出登录按钮
// - 退出登录:clearLogin() 清 localStorage + useNavigate 跳 /login(replace)
// - 用 react-router 的 navigate 替代 window.location,避免整页刷新
function HeaderRight() {
  const navigate = useNavigate();
  const account = getLoggedInAccount();

  function handleLogout() {
    clearLogin();
    navigate("/login", { replace: true });
  }

  return (
    <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
      {account && (
        <span style={{ fontSize: 13, color: "var(--text-muted)" }}>{account}</span>
      )}
      <button
        type="button"
        data-testid="logout-button"
        onClick={handleLogout}
        className="btn-secondary"
        style={{ padding: "6px 14px", fontSize: 13 }}
      >
        退出登录
      </button>
    </div>
  );
}

function Logo() {
  return (
    <span style={{
      fontSize: 22,
      fontWeight: 700,
      color: "var(--primary)",
      letterSpacing: 2,
    }}>星月</span>
  );
}

function Sidebar() {
  const linkStyle = ({ isActive }: { isActive: boolean }) => ({
    display: "block",
    padding: "10px 24px",
    color: isActive ? "var(--primary)" : "var(--text)",
    background: isActive ? "var(--primary-soft)" : "transparent",
    borderLeft: isActive ? "3px solid var(--primary)" : "3px solid transparent",
    fontSize: 14,
  });

  return (
    <nav>
      <NavLink to="/projects" style={linkStyle}>商机信息</NavLink>
      <NavLink to="/customers" style={linkStyle}>客户信息</NavLink>
      <NavLink to="/tasks" style={linkStyle}>代办任务</NavLink>
      <NavLink to="/opportunities" style={linkStyle}>公开信息</NavLink>
      <NavLink to="/settings" style={linkStyle}>系统设置</NavLink>
      <NavLink to="/users" style={linkStyle}>用户管理</NavLink>
    </nav>
  );
}
