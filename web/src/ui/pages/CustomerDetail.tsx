import { useEffect, useState, useMemo, useCallback, type ReactNode } from "react";
import { useParams, Link } from "react-router-dom";
import { CRMQuery } from "@/query";
import type { Customer, ComplianceRisk, Analysis } from "@/query/types";
import { StatusBadge } from "../components/StatusBadge";
import { GradeChip } from "../components/GradeChip";
import { RiskBadge } from "../components/RiskBadge";
import { Timeline } from "../components/Timeline";
import { formatValue, formatDateTime } from "../format";

const q = new CRMQuery();

export function CustomerDetail() {
  const { id } = useParams<{ id: string }>();
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setCustomer(null);
    setError(null);
    q.getCustomer(id)
      .then(setCustomer)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)));
  }, [id]);

  // 保存回调必须放在所有 hooks 之后、early return 之前
  const handleSaveContacts = useCallback(async (value: string | string[]) => {
    await q.patchCustomer(id!, { contacts: value });
    setCustomer((prev) => {
      if (!prev) return prev;
      return { ...prev, basic: { ...prev.basic, contacts: value } };
    });
  }, [id]);

  const handleSavePhones = useCallback(async (value: string | string[]) => {
    await q.patchCustomer(id!, { phones: value });
    setCustomer((prev) => {
      if (!prev) return prev;
      return { ...prev, basic: { ...prev.basic, phones: value } };
    });
  }, [id]);

  if (error) {
    return (
      <div style={{ padding: 24, color: "#991b1b" }}>
        加载失败：{error}
        <div style={{ marginTop: 12 }}>
          <Link to="/customers" style={{ color: "var(--primary)", fontSize: 13 }}>← 返回客户列表</Link>
        </div>
      </div>
    );
  }
  if (!customer) return <div style={{ padding: 24 }}>加载中...</div>;

  const c = customer;
  const b = c.basic;
  const p = c.prospecting;
  const e = c.engagement;

  return (
    <div style={{ padding: 24, display: "grid", gap: 16, maxWidth: 960 }}>
      <div>
        <Link to="/customers" style={{ color: "var(--primary)", fontSize: 13 }}>← 返回客户列表</Link>
      </div>

      {/* 头部 */}
      <header style={{
        background: "white", border: "1px solid var(--border)",
        borderRadius: 8, padding: 16,
      }}>
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <h2 style={{ fontSize: 20 }}>{b.name}</h2>
          {e?.status && <StatusBadge status={e.status} />}
          {p?.grade && <GradeChip grade={p.grade} />}
        </div>
      </header>

      {/* basic */}
      <Section title="客户基本信息">
        <FieldRow label="客户名称" value={formatValue(b.name)} />
        <FieldRow label="所属国家" value={formatValue(b.country)} />
        <FieldRow label="行业" value={formatValue(b.industry)} />
        <FieldRow label="规模" value={formatValue(b.scale)} />
        <EditableFieldRow
          label="联系人"
          rawValue={b.contacts}
          onSave={handleSaveContacts}
          editBtnTestId="edit-contacts-btn"
          saveBtnTestId="save-contacts-btn"
          cancelBtnTestId="cancel-contacts-btn"
          inputTestId="input-contacts"
        />
        <EditableFieldRow
          label="电话"
          rawValue={b.phones}
          onSave={handleSavePhones}
          editBtnTestId="edit-phones-btn"
          saveBtnTestId="save-phones-btn"
          cancelBtnTestId="cancel-phones-btn"
          inputTestId="input-phones"
        />
      </Section>

      {/* prospecting */}
      <Section title="爬取阶段信息">
        <FieldRow label="数据来源 URL" value={formatValue(p?.source_url)} />
        <FieldRow label="来源网站" value={formatValue(p?.source_site)} />
        <FieldRow label="海外投资历史" value={formatValue(p?.investment_history)} />
        <FieldRow label="土地交易" value={formatValue(p?.land_deal)} />
        <FieldRow label="潜力等级" value={p?.grade ?? "无"} />
        <FieldRow label="整体风险" value={
          p?.overall_risk
            ? <RiskBadge risk={p.overall_risk} />
            : "无"
        } />
        <FieldRow label="园区适配" value={formatValue(p?.park_fit_rating)} />
        <FieldRow label="需求强度" value={formatValue(p?.demand_strength)} />
        <FieldRow label="禁止行业" value={formatValue(p?.prohibited_industries)} />
        <FieldRow label="初筛依据" value={formatValue(p?.screening_reasons)} />
        <FieldRow label="数据获取时间" value={formatDateTime(p?.source_extracted_at)} />
        <FieldRow label="合规分析" value={<ComplianceView risk={p?.compliance_risk} />} />
      </Section>

      {/* engagement */}
      <Section title="跟进与意向">
        <FieldRow label="状态" value={e?.status ?? "无"} />
        <FieldRow label="意向等级" value={formatValue(e?.intent_level)} />
        <FieldRow label="关键问题" value={formatValue(e?.key_questions)} />
        <FieldRow label="顾虑点" value={formatValue(e?.concerns)} />
        <FieldRow label="最近邮件 ID" value={formatValue(e?.last_email_id)} />
        <FieldRow label="负责员工" value={formatValue(e?.assigned_to)} />
        <FieldRow label="分析结果" value={<AnalysisView analysis={e?.analysis} />} />
      </Section>

      {/* 时间线 */}
      <Section title="时间线">
        <Timeline events={[...c.timeline].sort((a, b) => (a.at < b.at ? 1 : -1))} />
      </Section>
    </div>
  );
}

function ComplianceView({ risk }: { risk?: ComplianceRisk }) {
  if (!risk) return "无";
  const dims: { key: keyof ComplianceRisk; label: string }[] = [
    { key: "foreign_investment", label: "外资准入" },
    { key: "tax", label: "税务政策" },
    { key: "labor", label: "劳工法规" },
    { key: "forex", label: "外汇管制" },
    { key: "industry_barrier", label: "行业壁垒" },
  ];
  return (
    <div style={{ display: "grid", gap: 4 }}>
      {dims.map(({ key, label }) => {
        const d = risk[key];
        if (!d) return (
          <div key={key} style={{ display: "grid", gridTemplateColumns: "100px 1fr", gap: 8, fontSize: 13 }}>
            <span style={{ color: "var(--text-muted)" }}>{label}</span>
            <span>无</span>
          </div>
        );
        return (
          <div key={key} style={{ display: "grid", gridTemplateColumns: "100px 80px 1fr", gap: 8, fontSize: 13 }}>
            <span style={{ color: "var(--text-muted)" }}>{label}</span>
            <span style={{ fontWeight: 600 }}>{d.rating || d.level || "无"}</span>
            <span style={{ color: "var(--text)" }}>{d.detail || d.note || "无"}</span>
          </div>
        );
      })}
    </div>
  );
}

function AnalysisView({ analysis }: { analysis?: Analysis }) {
  if (!analysis) return "无";
  const lines: string[] = [];
  if (analysis.profile?.notes) lines.push(`综合判断：${analysis.profile.notes}`);
  if (analysis.recommended_action) lines.push(`推荐行动：${analysis.recommended_action}`);
  if (analysis.urgency_level) lines.push(`紧急程度：${analysis.urgency_level}`);
  if (analysis.analyzed_at) lines.push(`分析时间：${formatDateTime(analysis.analyzed_at)}`);
  if (lines.length === 0) return "无";
  return (
    <div style={{ display: "grid", gap: 4, fontSize: 13 }}>
      {lines.map((l, i) => <div key={i}>{l}</div>)}
    </div>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section style={{
      background: "white", border: "1px solid var(--border)",
      borderRadius: 8, padding: 16,
    }}>
      <h3 style={{ fontSize: 15, marginBottom: 12, color: "var(--primary)" }}>{title}</h3>
      {children}
    </section>
  );
}

function FieldRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div style={{
      display: "grid",
      gridTemplateColumns: "140px 1fr",
      gap: 8,
      padding: "8px 0",
      fontSize: 14,
      borderBottom: "1px solid #f3f4f6",
    }}>
      <span style={{ color: "var(--text-muted)" }}>{label}</span>
      <span>{value}</span>
    </div>
  );
}

// 将 rawValue 规范化成 string[]（undefined / "" → []；string → [v]；[] → []）
function toItems(raw: string | string[] | undefined): string[] {
  if (raw === undefined || raw === "") return [];
  if (Array.isArray(raw)) return raw.filter(Boolean);
  return [raw];
}

// 可编辑字段行组件：用芯片（chip）方式编辑多值字段（联系人/电话）。
// 展示态：值用逗号拼接显示 +「编辑」按钮
// 编辑态：每个值是一个可删除的芯片，底部有添加输入框；按回车或失焦添加新值；
//         Backspace 在空输入时删除最后一个芯片
function EditableFieldRow({
  label,
  rawValue,
  onSave,
  editBtnTestId,
  saveBtnTestId,
  cancelBtnTestId,
  inputTestId,
}: {
  label: string;
  rawValue: string | string[] | undefined;
  onSave: (value: string | string[]) => Promise<void>;
  editBtnTestId: string;
  saveBtnTestId: string;
  cancelBtnTestId: string;
  inputTestId: string;
}) {
  const [editing, setEditing] = useState(false);
  const [items, setItems] = useState<string[]>([]);
  const [addValue, setAddValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const displayValue = useMemo(() => {
    const arr = toItems(rawValue);
    if (arr.length === 0) return "无";
    return arr.join("，");
  }, [rawValue]);

  const enterEdit = () => {
    setItems(toItems(rawValue));
    setAddValue("");
    setError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setError(null);
  };

  const commitAdd = () => {
    // 支持粘贴逗号分隔的多个值，批量分裂
    const parts = addValue.split(/[,，]/).map((s) => s.trim()).filter(Boolean);
    if (parts.length === 0) return;
    setItems((prev) => [...prev, ...parts]);
    setAddValue("");
  };

  const removeItem = (index: number) => {
    setItems((prev) => prev.filter((_, i) => i !== index));
  };

  const handleAddKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      commitAdd();
    } else if (e.key === "Backspace" && addValue === "" && items.length > 0) {
      removeItem(items.length - 1);
    }
  };

  const handleSave = async () => {
    const result = items.length === 1 ? items[0] : items;
    setSaving(true);
    setError(null);
    try {
      await onSave(result);
      setEditing(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  const hasChanged = useMemo(() => {
    const orig = toItems(rawValue);
    if (orig.length !== items.length) return true;
    return orig.some((v, i) => v !== items[i]);
  }, [items, rawValue]);

  const chipInputId = `${inputTestId}-add`;

  const rowStyle: React.CSSProperties = {
    display: "grid",
    gridTemplateColumns: "140px 1fr",
    gap: 8,
    padding: "8px 0",
    fontSize: 14,
    borderBottom: "1px solid #f3f4f6",
    alignItems: items.length > 2 ? "flex-start" : "center",
  };
  const labelStyle: React.CSSProperties = { color: "var(--text-muted)", paddingTop: 4 };

  if (editing) {
    return (
      <div style={rowStyle}>
        <span style={labelStyle}>{label}</span>
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {/* 芯片列表 */}
          <div style={{ display: "flex", flexWrap: "wrap", gap: 6, alignItems: "center" }}>
            {items.map((item, i) => (
              <span
                key={`${item}-${i}`}
                data-testid={`chip-${inputTestId}-${i}`}
                style={chipStyle}
              >
                {item}
                <button
                  data-testid={`chip-remove-${inputTestId}-${i}`}
                  onClick={() => removeItem(i)}
                  disabled={saving}
                  style={chipRemoveStyle}
                  aria-label={`删除 ${item}`}
                >
                  ×
                </button>
              </span>
            ))}
          </div>

          {/* 添加输入框 */}
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <input
              data-testid={chipInputId}
              value={addValue}
              onChange={(e) => setAddValue(e.target.value)}
              onKeyDown={handleAddKeyDown}
              onBlur={commitAdd}
              placeholder={items.length === 0 ? `输入${label}后按回车添加` : "继续添加，按回车确认"}
              disabled={saving}
              style={{
                flex: 1, padding: "4px 8px", fontSize: 14,
                border: "1px solid var(--border)", borderRadius: 4,
              }}
            />
            <button
              data-testid={saveBtnTestId}
              disabled={!hasChanged || saving}
              onClick={handleSave}
              style={{
                padding: "4px 12px", fontSize: 13, cursor: "pointer",
                background: "var(--primary)", color: "white",
                border: "none", borderRadius: 4, opacity: !hasChanged || saving ? 0.5 : 1,
                whiteSpace: "nowrap",
              }}
            >
              {saving ? "保存中..." : "保存"}
            </button>
            <button
              data-testid={cancelBtnTestId}
              onClick={cancelEdit}
              disabled={saving}
              style={{
                padding: "4px 12px", fontSize: 13, cursor: "pointer",
                background: "transparent", color: "var(--text-muted)",
                border: "1px solid var(--border)", borderRadius: 4,
                whiteSpace: "nowrap",
              }}
            >
              取消
            </button>
          </div>
          {error && <span style={{ color: "#991b1b", fontSize: 13 }}>{error}</span>}
        </div>
      </div>
    );
  }

  return (
    <div style={rowStyle}>
      <span style={labelStyle}>{label}</span>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span style={{ flex: 1 }}>{displayValue}</span>
        <button
          data-testid={editBtnTestId}
          onClick={enterEdit}
          style={{
            padding: "2px 8px", fontSize: 12, cursor: "pointer",
            background: "transparent", color: "var(--primary)",
            border: "1px solid var(--border)", borderRadius: 4,
          }}
        >
          编辑
        </button>
      </div>
    </div>
  );
}

const chipStyle: React.CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 4,
  padding: "2px 8px",
  fontSize: 13,
  background: "#eef2ff",
  color: "var(--primary)",
  borderRadius: 12,
  border: "1px solid #c7d2fe",
};

const chipRemoveStyle: React.CSSProperties = {
  background: "none",
  border: "none",
  cursor: "pointer",
  fontSize: 15,
  lineHeight: 1,
  padding: 0,
  color: "inherit",
  opacity: 0.6,
};
