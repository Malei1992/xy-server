import type { Project } from "@/query/types";
import { formatDateTime, formatValue } from "../format";

// 商机信息列表
// 列：项目名称 / 客户名称 / 意向等级 / 跟进状态 / 邮箱 / 负责人 / 备注说明 / 更新时间
// - table-layout: fixed + 等宽列
// - 数据列超长截断省略，title 属性展示完整内容供悬浮查看
// - 客户名称列可点击，触发 onCustomerClick(customer_id) 跳转客户详情
// - 客户名称为空（customer_id 找不到客户）时显示为不可点击的 "—"
export function ProjectTable({
  projects, onCustomerClick,
}: {
  projects: Project[];
  onCustomerClick: (customerId: string) => void;
}) {
  if (projects.length === 0) {
    return <div style={{ padding: 24, color: "var(--text-muted)" }}>暂无商机</div>;
  }
  return (
    <table style={{ tableLayout: "fixed", width: "100%" }} data-testid="projects-table">
      <thead>
        <tr>
          <th>项目名称</th>
          <th>客户名称</th>
          <th>意向等级</th>
          <th>跟进状态</th>
          <th>邮箱</th>
          <th>负责人</th>
          <th>备注说明</th>
          <th>更新时间</th>
        </tr>
      </thead>
      <tbody>
        {projects.map((p) => {
          const customerName = p.customer_name;
          const hasCustomer = Boolean(p.customer_id);
          return (
            <tr key={p.id} data-testid={`row-${p.id}`}>
              <td title={p.project_name} style={truncatedCell}>{p.project_name}</td>
              <td
                title={customerName}
                style={truncatedCell}
                data-testid={`customer-${p.id}`}
              >
                {hasCustomer && customerName ? (
                  <button
                    type="button"
                    onClick={() => onCustomerClick(p.customer_id)}
                    style={customerLinkStyle}
                  >
                    {customerName}
                  </button>
                ) : (
                  <span style={{ color: "var(--text-muted)" }}>—</span>
                )}
              </td>
              <td title={p.intent_level} style={truncatedCell}>{formatValue(p.intent_level)}</td>
              <td title={p.status} style={truncatedCell}>{p.status}</td>
              <td title={p.customer_email} style={truncatedCell}>{formatValue(p.customer_email)}</td>
              <td title={p.assigned_to ?? ""} style={truncatedCell}>{formatValue(p.assigned_to)}</td>
              <td title={p.notes ?? ""} style={truncatedCell}>{formatValue(p.notes)}</td>
              <td title={p.updated_at} style={truncatedCell}>{formatDateTime(p.updated_at)}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

// 单元格通用样式：超长截断省略（配合 table-layout: fixed + 等宽列使用）
const truncatedCell: React.CSSProperties = {
  overflow: "hidden",
  textOverflow: "ellipsis",
  whiteSpace: "nowrap",
};

// 客户名称按钮：去掉默认 button 样式，保留主色和下划线，提示可点击
const customerLinkStyle: React.CSSProperties = {
  background: "none",
  border: "none",
  padding: 0,
  color: "var(--primary)",
  cursor: "pointer",
  textDecoration: "underline",
  font: "inherit",
  maxWidth: "100%",
  overflow: "hidden",
  textOverflow: "ellipsis",
  whiteSpace: "nowrap",
  display: "block",
};
