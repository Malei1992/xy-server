import type { Customer } from "@/query/types";
import { GradeChip } from "./GradeChip";
import { CopyButton } from "./CopyButton";
import { formatList, formatValue, formatDateTime } from "../format";

// 客户信息列表
// 列：客户名称 / 所属国家 / 潜力等级 / 邮箱 / 电话 / 获取时间 / 操作
// - table-layout: fixed + 7 列等宽（最后一列固定 100px）
// - 数据列超长截断省略，title 属性展示完整内容供悬浮查看
// - 客户名称列右侧带复制按钮，点击后短暂显示 ✅
// - 每行末尾的"详情"按钮触发 onDetail(id)
export function CustomerTable({
  customers, onDetail,
}: { customers: Customer[]; onDetail: (id: string) => void }) {
  if (customers.length === 0) {
    return <div style={{ padding: 24, color: "var(--text-muted)" }}>暂无客户</div>;
  }
  return (
    <table style={{ tableLayout: "fixed", width: "100%" }}>
      <thead>
        <tr>
          <th>客户名称</th>
          <th>所属国家</th>
          <th>潜力等级</th>
          <th>邮箱</th>
          <th>电话</th>
          <th>获取时间</th>
          <th style={{ width: 100 }}>操作</th>
        </tr>
      </thead>
      <tbody>
        {customers.map((c) => {
          const name = c.basic.name;
          const country = c.basic.country;
          const gradeNode = c.prospecting?.grade ? <GradeChip grade={c.prospecting.grade} /> : formatValue(undefined);
          const contacts = formatList(c.basic.contacts);
          const phones = formatList(c.basic.phones);
          const extractedAt = formatDateTime(c.prospecting?.source_extracted_at);
          return (
            <tr key={c.id} data-testid={`row-${c.id}`}>
              <td title={name} style={truncatedCell}>
                <div style={nameCellLayout}>
                  <span
                    title={name}
                    style={{
                      flex: 1, minWidth: 0,
                      overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
                    }}
                  >{name}</span>
                  {name && <CopyButton text={name} />}
                </div>
              </td>
              <td title={country} style={truncatedCell}>{country}</td>
              <td style={truncatedCell}>{gradeNode}</td>
              <td title={contacts} style={truncatedCell}>{contacts}</td>
              <td title={phones} style={truncatedCell}>{phones}</td>
              <td title={extractedAt} style={truncatedCell}>{extractedAt}</td>
              <td>
                <button
                  onClick={() => onDetail(c.id)}
                  data-testid={`detail-${c.id}`}
                >详情</button>
              </td>
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

// 名称列：左侧名称 + 右侧复制按钮，整体 flex 布局让按钮固定宽度、名称按需截断
const nameCellLayout: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: 6,
  minWidth: 0,
};
