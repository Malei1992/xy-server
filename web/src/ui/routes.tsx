import { Routes, Route, Navigate, useLocation } from "react-router-dom";
import { Layout } from "./components/Layout";
import { Workbench } from "./pages/Workbench";
import { CustomerList } from "./pages/CustomerList";
import { CustomerDetail } from "./pages/CustomerDetail";
import { ProjectList } from "./pages/ProjectList";
import { TaskList } from "./pages/TaskList";
import { OpportunityList } from "./pages/OpportunityList";
import { Settings } from "./pages/Settings";
import { Users } from "./pages/Users";
import { Login } from "./pages/Login";
import { NotFound } from "./pages/NotFound";
import { isLoggedIn } from "./auth";

// RequireAuth 路由 gate:未登录时跳 /login(replace)
// 用 useLocation 读当前路径,Navigate 在未登录时一键重定向。
function RequireAuth({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  if (!isLoggedIn()) {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }
  return <>{children}</>;
}

export function AppRoutes() {
  return (
    <Routes>
      {/* /login 免 gate */}
      <Route path="/login" element={<Login />} />

      {/* 顶层 Workbench 不在侧边栏,自己独立页(也要求登录) */}
      <Route
        path="/workbench"
        element={
          <RequireAuth>
            <Workbench />
          </RequireAuth>
        }
      />

      {/* 主框架:Layout 内所有页面要求登录 */}
      <Route
        path="/"
        element={
          <RequireAuth>
            <Layout />
          </RequireAuth>
        }
      >
        <Route index element={<Navigate to="/customers" replace />} />
        <Route path="customers" element={<CustomerList />} />
        <Route path="customers/:id" element={<CustomerDetail />} />
        <Route path="projects" element={<ProjectList />} />
        <Route path="tasks" element={<TaskList />} />
        <Route path="opportunities" element={<OpportunityList />} />
        <Route path="settings" element={<Settings />} />
        <Route path="users" element={<Users />} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  );
}
