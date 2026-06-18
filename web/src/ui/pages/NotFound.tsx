import { Link } from "react-router-dom";

// 404 页面：路由不匹配时显示
export function NotFound() {
  return (
    <div style={{ padding: 48, textAlign: "center" }}>
      <h2 style={{ fontSize: 24, marginBottom: 8 }}>404</h2>
      <p style={{ color: "#6b7280", marginBottom: 16 }}>页面不存在</p>
      <Link to="/workbench" style={{ color: "#2563eb" }}>返回代办</Link>
    </div>
  );
}
