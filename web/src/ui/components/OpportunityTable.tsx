import type { Opportunity, OpportunityStatus } from "@/query/types";
import { formatValue } from "../format";
import { InlineStatusSelect } from "./InlineStatusSelect";

// 公开信息列表
// 列：名称 / 详情 / 信息来源 / 来源类型 / 状态 / 客户名称 / 说明
// - table-layout: fixed + 等宽列；超长截断省略，title 属性展示完整内容
// - 信息来源列：source_url 非空 → 渲染为新页打开的 <a target="_blank" rel="noopener noreferrer">；
//               否则显示 "—"
// - 客户名称列：customer_id 存在 + customer_name 非空 → 可点击按钮跳转；
//               否则显示 "—"
// - 名称 / 详情 / 说明 缺失值（undefined / ""）显示 "无"（由 formatValue 处理）
// - 状态列：内联 <select>(InlineStatusSelect) 直接修改,onChange 立即 PATCH
export function OpportunityTable({
  opportunities, onCustomerClick, statusOptions, onStatusChange,
}: {
  opportunities: Opportunity[];
  onCustomerClick: (customerId: string) => void;
  statusOptions: ReadonlyArray<{ value: OpportunityStatus; label: string }>;
  onStatusChange: (id: string, newStatus: OpportunityStatus) => Promise<void>;
}) {
  if (opportunities.length === 0) {
    return <div style={{ padding: 24, color: "var(--text-muted)" }}>暂无公开信息</div>;
  }
  return (
    <table style={{ tableLayout: "fixed", width: "100%" }} data-testid="opportunities-table">
      <thead>
        <tr>
          <th>名称</th>
          <th>详情</th>
          <th>信息来源</th>
          <th>来源类型</th>
          <th>状态</th>
          <th>客户名称</th>
          <th>说明</th>
        </tr>
      </thead>
      <tbody>
        {opportunities.map((o) => {
          const customerName = o.customer_name;
          const hasCustomer = Boolean(o.customer_id);
          const hasUrl = Boolean(o.source_url);
          return (
            <tr key={o.id} data-testid={`row-${o.id}`}>
              <td title={o.opportunity_name} style={truncatedCell}>{o.opportunity_name}</td>
              <td title={o.opportunity_info ?? ""} style={truncatedCell}>{formatValue(o.opportunity_info)}</td>
              <td
                title={o.source_url ?? ""}
                style={truncatedCell}
                data-testid={`source-url-${o.id}`}
              >
                {hasUrl ? (
                  <a
                    href={o.source_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    style={sourceLinkStyle}
                  >
                    {o.source_url}
                  </a>
                ) : (
                  <span style={{ color: "var(--text-muted)" }}>—</span>
                )}
              </td>
              <td title={o.source_type} style={truncatedCell}>{o.source_type}</td>
              <td title={o.status} style={truncatedCell}>
                <InlineStatusSelect
                  value={o.status}
                  options={statusOptions}
                  onChange={async (newStatus) => {
                    await onStatusChange(o.id, newStatus);
                  }}
                />
              </td>
              <td
                title={customerName}
                style={truncatedCell}
                data-testid={`customer-${o.id}`}
              >
                {hasCustomer && customerName ? (
                  <button
                    type="button"
                    onClick={() => onCustomerClick(o.customer_id!)}
                    style={customerLinkStyle}
                  >
                    {customerName}
                  </button>
                ) : (
                  <span style={{ color: "var(--text-muted)" }}>—</span>
                )}
              </td>
              <td title={o.notes ?? ""} style={truncatedCell}>{formatValue(o.notes)}</td>
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

// 信息来源链接：去掉下划线，主色，超长截断
const sourceLinkStyle: React.CSSProperties = {
  color: "var(--primary)",
  textDecoration: "none",
  maxWidth: "100%",
  overflow: "hidden",
  textOverflow: "ellipsis",
  whiteSpace: "nowrap",
  display: "block",
};