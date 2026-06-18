import { Routes, Route, Navigate } from "react-router-dom";
import { Layout } from "./components/Layout";
import { Workbench } from "./pages/Workbench";
import { CustomerList } from "./pages/CustomerList";
import { CustomerDetail } from "./pages/CustomerDetail";
import { ProjectList } from "./pages/ProjectList";
import { TaskList } from "./pages/TaskList";
import { OpportunityList } from "./pages/OpportunityList";
import { Settings } from "./pages/Settings";
import { NotFound } from "./pages/NotFound";

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Navigate to="/customers" replace />} />
        <Route path="customers" element={<CustomerList />} />
        <Route path="customers/:id" element={<CustomerDetail />} />
        <Route path="projects" element={<ProjectList />} />
        <Route path="tasks" element={<TaskList />} />
        <Route path="opportunities" element={<OpportunityList />} />
        <Route path="settings" element={<Settings />} />
        <Route path="*" element={<NotFound />} />
      </Route>
      {/* Workbench 保留为顶层路由,侧边栏无入口 */}
      <Route path="/workbench" element={<Workbench />} />
    </Routes>
  );
}
