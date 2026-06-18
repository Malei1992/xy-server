import type { Task } from "@/query/types";
import {
  formatDateTime, formatValue,
  formatTaskType, formatTaskStatus,
} from "../format";
import { PriorityBadge } from "./PriorityBadge";

// 代办任务列表
// 列：任务标题 / 任务类型 / 任务等级 / 任务状态 / 客户名称 / 详细说明 /
//     负责人 / 解决时间 / 解决说明
// - table-layout: fixed + 等宽列；超长截断省略，title 属性展示完整内容
// - 客户名称列：customer_id 存在 + customer_name 非空 → 可点击按钮跳转；
//               否则显示 "—"
// - 任务等级列：圆形徽章（PriorityBadge），P0 红 / P1 黄 / P2 绿 / P3 灰
// - 任务类型 / 任务状态：cell 展示中文字面（formatTaskType / formatTaskStatus），
//                       title 属性展示原始 enum 方便对照
// - 空字段（undefined / ""）一律显示 "无"（由 formatValue / formatTaskType/Status 处理）
export function TaskTable({
  tasks, onCustomerClick,
}: {
  tasks: Task[];
  onCustomerClick: (customerId: string) => void;
}) {
  if (tasks.length === 0) {
    return <div style={{ padding: 24, color: "var(--text-muted)" }}>暂无代办任务</div>;
  }
  return (
    <table style={{ tableLayout: "fixed", width: "100%" }} data-testid="tasks-table">
      <thead>
        <tr>
          <th>任务标题</th>
          <th>任务类型</th>
          <th>任务等级</th>
          <th>任务状态</th>
          <th>客户名称</th>
          <th>详细说明</th>
          <th>负责人</th>
          <th>解决时间</th>
          <th>解决说明</th>
        </tr>
      </thead>
      <tbody>
        {tasks.map((t) => {
          const customerName = t.customer_name;
          const hasCustomer = Boolean(t.customer_id);
          return (
            <tr key={t.id} data-testid={`row-${t.id}`}>
              <td title={t.title} style={truncatedCell}>{t.title}</td>
              <td title={t.type} style={truncatedCell}>{formatTaskType(t.type)}</td>
              <td style={truncatedCell}><PriorityBadge priority={t.priority} /></td>
              <td title={t.status} style={truncatedCell}>{formatTaskStatus(t.status)}</td>
              <td
                title={customerName}
                style={truncatedCell}
                data-testid={`customer-${t.id}`}
              >
                {hasCustomer && customerName ? (
                  <button
                    type="button"
                    onClick={() => onCustomerClick(t.customer_id!)}
                    style={customerLinkStyle}
                  >
                    {customerName}
                  </button>
                ) : (
                  <span style={{ color: "var(--text-muted)" }}>—</span>
                )}
              </td>
              <td title={t.description ?? ""} style={truncatedCell}>{formatValue(t.description)}</td>
              <td title={t.assigned_to ?? ""} style={truncatedCell}>{formatValue(t.assigned_to)}</td>
              <td title={t.resolved_at ?? ""} style={truncatedCell}>{formatDateTime(t.resolved_at)}</td>
              <td title={t.resolution ?? ""} style={truncatedCell}>{formatValue(t.resolution)}</td>
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
