import { NavLink, Outlet } from "react-router-dom";

// 全局布局：顶栏（星月 logo） + 左侧导航 + 主内容区
export function Layout() {
  return (
    <div style={{
      display: "grid",
      gridTemplateRows: "56px 1fr",
      gridTemplateColumns: "200px 1fr",
      height: "100vh",
    }}>
      <header style={{
        gridColumn: "1 / 3",
        display: "flex",
        alignItems: "center",
        padding: "0 24px",
        borderBottom: "1px solid var(--border)",
        background: "white",
      }}>
        <Logo />
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
    </nav>
  );
}
