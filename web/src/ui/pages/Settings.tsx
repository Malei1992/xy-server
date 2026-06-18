import { useEffect, useRef, useState } from "react";
import { CRMQuery } from "@/query";
import type { Site, WechatBindPollResult } from "@/query/types";

// 系统设置：每次进入页面都从 /api/config 拉取最新 .env（已被 Go 后端解析成 JSON）。
// 后端读取根目录 .env，返回 {"env": {"KEY":"value",...}}。
// 切换路由（进入 /settings）→ useEffect 触发 → fetch → 更新 UI。
// 邮件配置 tab：所有字段常驻可编辑，底部统一保存/取消。
// 资料上传：multipart/form-data POST 到 /api/uploads/* 端点；
// 前端预校验文件后缀 + 大小，错误直接 setError 不发请求。
type Env = Record<string, string>;

// 系统设置页面有四个 tab：邮件配置 / 资料上传 / 公开数据源 / 微信绑定；默认 tab 是邮件配置
type TabKey = "email" | "upload" | "sources" | "wechat";

// 邮件配置 tab 内所有可编辑字段的 key 列表。
// 后端 PATCH /api/config 会按这些 key 写回 .env。
const EDITABLE_KEYS = [
  "SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD",
  "IMAP_HOST", "IMAP_PORT", "IMAP_USERNAME", "IMAP_PASSWORD",
  "EMAIL_REQUIRE_REVIEW", "REVIEWER_EMAIL",
] as const;
type EditableKey = (typeof EDITABLE_KEYS)[number];
type Draft = Record<EditableKey, string>;

// 客户价值等级标准：只允许 A/B/C（后端 /api/grading-rules 契约）
// 客户意向等级标准：S/A/B/C（后端 /api/interest-level 契约）
// 两个端点独立维护各自的等级集合，前端按各自 levelKeys 渲染。
const VALUE_LEVEL_KEYS = ["A", "B", "C"] as const;
const INTENT_LEVEL_KEYS = ["S", "A", "B", "C"] as const;
type Levels = Record<string, string>;

// 公开数据源 type 字段枚举：value 是后端存的英文代码，label 是 UI 显示的中文。
// 新增枚举时同步改前后端；下拉只展示这 2 个选项。
const SITE_TYPE_OPTIONS: ReadonlyArray<{ value: string; label: string }> = [
  { value: "crawl", label: "常规爬取" },
  { value: "download", label: "文件下载" },
];

// 公开数据源行内 5 个数据框的统一样式：4 个 input + 1 个 select 视觉完全对齐。
// 关键点：
//   - box-sizing: border-box 让 width 含 padding/border，5 列严格同宽
//   - 固定 height 28px（不用默认 line-height）消除 input/select 的浏览器差异
//   - select 用 appearance: none 去掉浏览器原生下拉箭头和 chrome 默认 padding，
//     并在右侧留 18px 空白用 ::after 背景画一个 ▾，避免挤压文字
//   - 4 个 input 样式一致：fontSize/padding/border/radius 全等
const cellInputStyle: React.CSSProperties = {
  boxSizing: "border-box",
  width: "100%",
  height: 28,
  padding: "0 6px",
  border: "1px solid var(--border)",
  borderRadius: 4,
  fontSize: 13,
  background: "white",
};
const cellSelectStyle: React.CSSProperties = {
  ...cellInputStyle,
  appearance: "none",
  WebkitAppearance: "none",
  MozAppearance: "none",
  // 右侧 18px 留给自绘 ▾ 箭头
  paddingRight: 22,
  // 自绘 ▾：用 base64 内联 SVG 当 background-image，箭头在右侧 6px 处
  backgroundImage:
    "url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 10 6'><path d='M0 0l5 6 5-6z' fill='%236b7280'/></svg>\")",
  backgroundRepeat: "no-repeat",
  backgroundPosition: "right 6px center",
  backgroundSize: "8px 5px",
  cursor: "pointer",
};

// 把后端 GET 返回的对象规范成 Levels（缺失 keys 填空串）
// keys 参数决定要规范出哪些 keys；不在 keys 里的多余字段被忽略
function normalizeLevels(raw: Record<string, string>, keys: readonly string[]): Levels {
  const norm: Levels = {};
  for (const k of keys) norm[k] = raw[k] ?? "";
  return norm;
}

// 构造一个初始 draft：所有 EDITABLE_KEYS 都用 initialConfig 的值（缺失则空串）
function buildDraft(initialConfig: Env): Draft {
  const d = {} as Draft;
  for (const k of EDITABLE_KEYS) {
    d[k] = initialConfig[k] ?? "";
  }
  return d;
}

export function Settings() {
  const [config, setConfig] = useState<Env | null>(null);
  const [valueLevels, setValueLevels] = useState<Levels | null>(null);
  const [interestLevels, setInterestLevels] = useState<Levels | null>(null);
  const [sites, setSites] = useState<Site[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tab, setTab] = useState<TabKey>("email");
  const [restarting, setRestarting] = useState(false);
  const [restartMsg, setRestartMsg] = useState<{ ok: boolean; text: string } | null>(null);
  // 单例 CRMQuery（patchConfig 用）
  const queryRef = useRef<CRMQuery | null>(null);
  if (!queryRef.current) queryRef.current = new CRMQuery();

  async function handleRestart() {
    setRestarting(true);
    setRestartMsg(null);
    try {
      await queryRef.current!.restart();
      setRestartMsg({ ok: true, text: "重启信号已发送，服务即将重新加载" });
    } catch (e) {
      setRestartMsg({ ok: false, text: e instanceof Error ? e.message : String(e) });
    } finally {
      setRestarting(false);
    }
  }

  // 重新拉取 /api/config（保存成功后刷新显示用）
  async function refresh(): Promise<void> {
    const r = await fetch("/api/config", { cache: "no-store" });
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    const body = (await r.json()) as { env: Env };
    setConfig(body.env);
  }

  // 重新拉取两个 level 标准（保存成功后刷新显示用）
  async function refreshValueLevels(): Promise<void> {
    const v = await queryRef.current!.getGradingRules();
    setValueLevels(normalizeLevels(v, VALUE_LEVEL_KEYS));
  }
  async function refreshInterestLevels(): Promise<void> {
    const v = await queryRef.current!.getInterestLevel();
    setInterestLevels(normalizeLevels(v, INTENT_LEVEL_KEYS));
  }
  // 重新拉取公开数据源
  async function refreshSites(): Promise<void> {
    const s = await queryRef.current!.getSites();
    setSites(s);
  }

  useEffect(() => {
    let cancelled = false;
    setConfig(null);
    setValueLevels(null);
    setInterestLevels(null);
    setSites(null);
    setError(null);

    // 三次拉取互相独立：任何一个失败都不阻塞其他；
    // 只有 /api/config 失败才显示全屏错误（其余失败由各自 section 处理）
    const loadConfig = fetch("/api/config", { cache: "no-store" })
      .then(async (r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json() as Promise<{ env: Env }>;
      })
      .then((body) => {
        if (cancelled) return;
        setConfig(body.env);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
      });

    const loadValueLevels = queryRef.current!.getGradingRules()
      .then((v) => {
        if (cancelled) return;
        setValueLevels(normalizeLevels(v, VALUE_LEVEL_KEYS));
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        // 不阻塞 config；值缺失时留 null，section 自己显示"加载中"
        console.error("加载客户价值等级标准失败:", e);
      });

    const loadInterestLevels = queryRef.current!.getInterestLevel()
      .then((v) => {
        if (cancelled) return;
        setInterestLevels(normalizeLevels(v, INTENT_LEVEL_KEYS));
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        console.error("加载客户意向等级标准失败:", e);
      });

    // 公开数据源：失败不阻塞其他 tab
    const loadSites = queryRef.current!.getSites()
      .then((s) => {
        if (cancelled) return;
        setSites(s);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        console.error("加载公开数据源失败:", e);
      });

    void Promise.allSettled([loadConfig, loadValueLevels, loadInterestLevels, loadSites]);
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) {
    return (
      <div style={{ padding: 24, color: "#991b1b" }}>
        加载失败：{error}
        <p style={{ fontSize: 12, color: "var(--text-muted)", marginTop: 8 }}>
          请确认 Go 后端（<code>server/</code>）已启动，
          dev 模式下由 vite proxy 转发 <code>/api/*</code> 到 <code>http://192.168.5.245:15373</code>
        </p>
      </div>
    );
  }
  if (!config) {
    return (
      <div style={{ padding: 24, color: "var(--text-muted)" }}>加载中...</div>
    );
  }

  return (
    <div style={{ padding: 24, maxWidth: 960 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 16 }}>
        <h2 style={{ fontSize: 18, margin: 0 }}>系统设置</h2>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          {restartMsg && (
            <span style={{ fontSize: 13, color: restartMsg.ok ? "#16a34a" : "#dc2626" }}>
              {restartMsg.text}
            </span>
          )}
          <button
            data-testid="restart-btn"
            onClick={handleRestart}
            disabled={restarting}
            style={{
              padding: "6px 16px",
              fontSize: 13,
              cursor: restarting ? "not-allowed" : "pointer",
              background: restarting ? "#9ca3af" : "#dc2626",
              color: "white",
              border: "none",
              borderRadius: 4,
              opacity: restarting ? 0.7 : 1,
              whiteSpace: "nowrap",
            }}
          >
            {restarting ? "重启中..." : "重启服务"}
          </button>
        </div>
      </div>
      <Tabs tab={tab} onTabChange={setTab} />
      {tab === "email" ? (
        <EmailConfigTab
          initialConfig={config}
          queryRef={queryRef}
          onConfigRefresh={refresh}
        />
      ) : tab === "upload" ? (
        <UploadTab
          queryRef={queryRef}
          valueLevels={valueLevels}
          interestLevels={interestLevels}
          onRefreshValueLevels={refreshValueLevels}
          onRefreshInterestLevels={refreshInterestLevels}
        />
      ) : tab === "sources" ? (
        <PublicSourcesTab
          queryRef={queryRef}
          sites={sites}
          onRefresh={refreshSites}
        />
      ) : (
        <WechatBindTab queryRef={queryRef} />
      )}
    </div>
  );
}

// 邮件配置 tab：所有字段常驻可编辑，底部统一保存/取消。
// - 内部维护 draft(每个 EDITABLE_KEYS 一个 string)
// - dirty 检测:draft 任何一项 != initialConfig 对应项 → 启用保存/取消
// - 保存:一次性 PATCH /api/config 带所有 EDITABLE_KEYS,成功后 setSavedAt + 父组件拉新
// - 取消:draft 还原到 initialConfig
function EmailConfigTab({
  initialConfig, queryRef, onConfigRefresh,
}: {
  initialConfig: Env;
  queryRef: React.MutableRefObject<CRMQuery | null>;
  onConfigRefresh: () => Promise<void>;
}) {
  const [draft, setDraft] = useState<Draft>(() => buildDraft(initialConfig));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [savedAt, setSavedAt] = useState<number | null>(null);

  // initialConfig 变化（保存后父组件 refresh）→ 重新同步 draft
  useEffect(() => {
    setDraft(buildDraft(initialConfig));
  }, [initialConfig]);

  // dirty 检测：与 initialConfig 任一 key 不等即视为脏
  const isDirty = EDITABLE_KEYS.some(
    (k) => (draft[k] ?? "") !== (initialConfig[k] ?? ""),
  );

  function setField(k: EditableKey, v: string) {
    setDraft((d) => ({ ...d, [k]: v }));
    // 改动后清掉"已保存"提示，避免误导
    setSavedAt(null);
  }

  async function save() {
    setBusy(true);
    setError(null);
    try {
      const updates: Record<string, string> = { ...draft };
      await queryRef.current!.patchConfig(updates);
      setSavedAt(Date.now());
      // 触发父组件拉新 config → initialConfig 变化 → useEffect 同步 draft
      await onConfigRefresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  function cancel() {
    setDraft(buildDraft(initialConfig));
    setError(null);
    setSavedAt(null);
  }

  return (
    <div style={{ display: "grid", gap: 16 }}>
      <EditableSection title="SMTP 服务（发件）" testId="smtp">
        <FieldRow label="服务器" type="text" value={draft.SMTP_HOST} onChange={(v) => setField("SMTP_HOST", v)} testId="smtp-host" />
        <FieldRow label="端口" type="text" value={draft.SMTP_PORT} onChange={(v) => setField("SMTP_PORT", v)} testId="smtp-port" />
        <FieldRow label="用户名" type="text" value={draft.SMTP_USERNAME} onChange={(v) => setField("SMTP_USERNAME", v)} testId="smtp-username" />
        <FieldRow label="密码" type="password" value={draft.SMTP_PASSWORD} onChange={(v) => setField("SMTP_PASSWORD", v)} testId="smtp-password" />
      </EditableSection>

      <EditableSection title="IMAP 服务（收件）" testId="imap">
        <FieldRow label="服务器" type="text" value={draft.IMAP_HOST} onChange={(v) => setField("IMAP_HOST", v)} testId="imap-host" />
        <FieldRow label="端口" type="text" value={draft.IMAP_PORT} onChange={(v) => setField("IMAP_PORT", v)} testId="imap-port" />
        <FieldRow label="用户名" type="text" value={draft.IMAP_USERNAME} onChange={(v) => setField("IMAP_USERNAME", v)} testId="imap-username" />
        <FieldRow label="密码" type="password" value={draft.IMAP_PASSWORD} onChange={(v) => setField("IMAP_PASSWORD", v)} testId="imap-password" />
      </EditableSection>

      <EditableSection title="邮件审核" testId="review">
        <FieldRow
          label="是否需要审核"
          type="select"
          value={draft.EMAIL_REQUIRE_REVIEW}
          onChange={(v) => setField("EMAIL_REQUIRE_REVIEW", v)}
          options={[{ value: "true", label: "是" }, { value: "false", label: "否" }]}
          testId="review-require"
        />
        <FieldRow
          label="审核人邮箱"
          type="text"
          value={draft.REVIEWER_EMAIL}
          onChange={(v) => setField("REVIEWER_EMAIL", v)}
          testId="review-email"
        />
      </EditableSection>

      {/* 底部统一保存栏 */}
      <div style={{
        display: "flex", gap: 8, alignItems: "center",
        paddingTop: 8, borderTop: "1px solid var(--border)",
      }}>
        <button
          type="button"
          onClick={save}
          disabled={!isDirty || busy}
          data-testid="email-config-save"
          style={{
            padding: "8px 20px", fontSize: 14, fontWeight: 500,
            background: isDirty && !busy ? "var(--primary)" : "#e5e7eb",
            color: isDirty && !busy ? "white" : "#9ca3af",
            border: "none", borderRadius: 4,
            cursor: isDirty && !busy ? "pointer" : "not-allowed",
          }}
        >
          {busy ? "保存中..." : "保存"}
        </button>
        <button
          type="button"
          onClick={cancel}
          disabled={!isDirty || busy}
          data-testid="email-config-cancel"
          style={{
            padding: "8px 20px", fontSize: 14,
            background: "white", color: "var(--text)",
            border: "1px solid var(--border)", borderRadius: 4,
            cursor: !isDirty || busy ? "not-allowed" : "pointer",
          }}
        >
          取消
        </button>
        {savedAt && !isDirty && !error && (
          <span
            style={{ color: "#16a34a", fontSize: 13 }}
            data-testid="email-config-saved-msg"
          >
            ✓ 已保存
          </span>
        )}
        {error && (
          <span
            style={{ color: "#991b1b", fontSize: 13 }}
            data-testid="email-config-error"
          >
            ✗ {error}
          </span>
        )}
      </div>
    </div>
  );
}

// 顶部 tab 切换条：水平排列的按钮，激活态显示主色下划线
function Tabs({
  tab, onTabChange,
}: {
  tab: TabKey;
  onTabChange: (next: TabKey) => void;
}) {
  return (
    <div
      role="tablist"
      style={{
        display: "flex",
        gap: 0,
        borderBottom: "1px solid var(--border)",
        marginBottom: 16,
      }}
    >
      <TabButton
        label="邮件配置"
        active={tab === "email"}
        onClick={() => onTabChange("email")}
        testId="tab-email"
      />
      <TabButton
        label="资料上传"
        active={tab === "upload"}
        onClick={() => onTabChange("upload")}
        testId="tab-upload"
      />
      <TabButton
        label="公开数据源"
        active={tab === "sources"}
        onClick={() => onTabChange("sources")}
        testId="tab-sources"
      />
      <TabButton
        label="微信绑定"
        active={tab === "wechat"}
        onClick={() => onTabChange("wechat")}
        testId="tab-wechat"
      />
    </div>
  );
}

// 单个 tab 按钮：active 时主色下划线 + 加粗
// marginBottom: -1 让 active 下划线和 tab header 底边重合
function TabButton({
  label, active, onClick, testId,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
  testId: string;
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      data-testid={testId}
      data-active={active ? "true" : "false"}
      style={{
        padding: "8px 16px",
        background: "none",
        border: "none",
        borderBottom: active ? "2px solid var(--primary)" : "2px solid transparent",
        color: active ? "var(--primary)" : "var(--text-muted)",
        fontSize: 14,
        fontWeight: active ? 600 : 400,
        cursor: "pointer",
        marginBottom: -1,
      }}
    >
      {label}
    </button>
  );
}

// section 卡片包装：标题 + 字段行列表（children）
// 不再带"编辑"按钮（字段常驻可编辑）
function EditableSection({
  title, testId, children,
}: {
  title: string;
  testId: string;
  children: React.ReactNode;
}) {
  return (
    <section
      style={{
        background: "white", border: "1px solid var(--border)",
        borderRadius: 8, padding: 16,
      }}
      data-testid={testId}
    >
      <h3 style={{ fontSize: 14, fontWeight: 600, color: "var(--text-muted)", marginBottom: 12 }}>
        {title}
      </h3>
      <div style={{ display: "grid", gap: 8 }}>{children}</div>
    </section>
  );
}

// 字段行：label + input/select
// - text/password：单行 input（密码用 monospace 字体）
// - 密码字段：悬浮/聚焦 → 明文；离开/失焦 → 圆点（仅切 type，不加眼睛图标）
// - select：<select>，options 由 props 传入
function FieldRow({
  label, type, value, onChange, options, testId,
}: {
  label: string;
  type: "text" | "password" | "select";
  value: string;
  onChange: (v: string) => void;
  options?: Array<{ value: string; label: string }>;
  testId: string;
}) {
  // 密码字段专用：revealed 切换 input type（true=text 明文；false=password 圆点）
  // 非密码字段永远 false，不挂 hover/focus handler
  const [revealed, setRevealed] = useState(false);
  const isPassword = type === "password";
  const actualType = isPassword ? (revealed ? "text" : "password") : type;

  return (
    <div style={{
      display: "grid", gridTemplateColumns: "120px 1fr",
      gap: 12, fontSize: 14, alignItems: "center",
    }}>
      <span style={{ color: "var(--text-muted)" }}>{label}</span>
      {type === "select" ? (
        <select
          value={value}
          onChange={(e) => onChange(e.target.value)}
          data-testid={testId}
          style={{
            padding: "4px 6px",
            border: "1px solid var(--border)",
            borderRadius: 4,
            fontSize: 14,
          }}
        >
          {options?.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
      ) : (
        <input
          type={actualType}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onMouseEnter={isPassword ? () => setRevealed(true) : undefined}
          onMouseLeave={isPassword ? () => setRevealed(false) : undefined}
          onFocus={isPassword ? () => setRevealed(true) : undefined}
          onBlur={isPassword ? () => setRevealed(false) : undefined}
          data-testid={testId}
          data-revealed={isPassword ? (revealed ? "true" : "false") : undefined}
          style={{
            padding: "4px 6px",
            border: "1px solid var(--border)",
            borderRadius: 4,
            fontSize: 14,
            fontFamily: isPassword ? "monospace" : "inherit",
          }}
        />
      )}
    </div>
  );
}

// 资料上传 tab：FAQ / 外宣材料上传 + 客户价值等级标准 + 客户意向等级标准 + 共享保存栏
// - 三个 section 顺序垂直排列（窄宽度 560px）
// - 两个 level section：等各自数据加载完成才渲染（避免空 section）
// - draft state 全部上提到 UploadTab，section 变受控；两个 section 共享底部保存栏
// - 保存只 PATCH 有改动的 section；取消还原两个 draft
function UploadTab({
  queryRef, valueLevels, interestLevels, onRefreshValueLevels, onRefreshInterestLevels,
}: {
  queryRef: React.MutableRefObject<CRMQuery | null>;
  valueLevels: Levels | null;
  interestLevels: Levels | null;
  onRefreshValueLevels: () => Promise<void>;
  onRefreshInterestLevels: () => Promise<void>;
}) {
  // 把后端响应规范成"只含 levelKeys"的 map（多余 keys 忽略，缺失填空串）
  const [valueDraft, setValueDraft] = useState<Levels | null>(
    valueLevels ? normalizeLevels(valueLevels, VALUE_LEVEL_KEYS) : null,
  );
  const [interestDraft, setInterestDraft] = useState<Levels | null>(
    interestLevels ? normalizeLevels(interestLevels, INTENT_LEVEL_KEYS) : null,
  );

  // 服务端最新值变化（保存后 refresh）→ 重新同步 draft
  useEffect(() => {
    if (valueLevels) setValueDraft(normalizeLevels(valueLevels, VALUE_LEVEL_KEYS));
  }, [valueLevels]);
  useEffect(() => {
    if (interestLevels) setInterestDraft(normalizeLevels(interestLevels, INTENT_LEVEL_KEYS));
  }, [interestLevels]);

  // dirty 检测：与最新服务端值对比，任一 key 不等即视为脏
  const valueDirty =
    valueDraft !== null &&
    valueLevels !== null &&
    VALUE_LEVEL_KEYS.some(
      (k) => (valueDraft[k] ?? "") !== (valueLevels[k] ?? ""),
    );
  const interestDirty =
    interestDraft !== null &&
    interestLevels !== null &&
    INTENT_LEVEL_KEYS.some(
      (k) => (interestDraft[k] ?? "") !== (interestLevels[k] ?? ""),
    );
  const anyDirty = valueDirty || interestDirty;

  // 共享保存栏状态
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [savedAt, setSavedAt] = useState<number | null>(null);

  // 顺序 PATCH：value 先 → intent 后；任一失败抛错、后续不执行
  // onRefresh* 触发父组件 setValueLevels → UploadTab useEffect 重新同步 draft
  async function save() {
    if (!anyDirty || busy) return;
    setBusy(true);
    setError(null);
    try {
      if (valueDirty && valueDraft) {
        await queryRef.current!.patchGradingRules(valueDraft);
        await onRefreshValueLevels();
      }
      if (interestDirty && interestDraft) {
        await queryRef.current!.patchInterestLevel(interestDraft);
        await onRefreshInterestLevels();
      }
      setSavedAt(Date.now());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  // 取消：两个 draft 都还原到服务端最新值；清掉 saved/error 提示
  function cancel() {
    if (valueLevels) setValueDraft(normalizeLevels(valueLevels, VALUE_LEVEL_KEYS));
    if (interestLevels) setInterestDraft(normalizeLevels(interestLevels, INTENT_LEVEL_KEYS));
    setError(null);
    setSavedAt(null);
  }

  return (
    <div style={{ display: "grid", gap: 16 }}>
      <UploadSection />
      {valueDraft ? (
        <LevelStandardSection
          title="客户价值等级标准"
          testId="value-levels"
          levelKeys={VALUE_LEVEL_KEYS}
          draft={valueDraft}
          onDraftChange={setValueDraft}
        />
      ) : (
        <div style={{ color: "var(--text-muted)", fontSize: 13 }} data-testid="value-levels-loading">
          客户价值等级标准加载中...
        </div>
      )}
      {interestDraft ? (
        <LevelStandardSection
          title="客户意向等级标准"
          testId="intent-levels"
          levelKeys={INTENT_LEVEL_KEYS}
          draft={interestDraft}
          onDraftChange={setInterestDraft}
        />
      ) : (
        <div style={{ color: "var(--text-muted)", fontSize: 13 }} data-testid="intent-levels-loading">
          客户意向等级标准加载中...
        </div>
      )}
      {valueDraft && interestDraft && (
        <LevelsSaveBar
          busy={busy}
          error={error}
          savedAt={savedAt}
          anyDirty={anyDirty}
          onSave={save}
          onCancel={cancel}
        />
      )}
    </div>
  );
}

// 资料上传 tab：两个上传项 FAQ/外宣材料
function UploadSection() {
  return (
    <EditableSection title="资料上传" testId="upload">
      <UploadField
        title="FAQ"
        accept=".doc,.docx"
        maxSizeMB={10}
        endpoint="/api/uploads/faq"
        testId="upload-faq"
      />
      <UploadField
        title="外宣材料"
        accept=".pdf"
        maxSizeMB={10}
        endpoint="/api/uploads/attachment-moonstar"
        testId="upload-attachment-moonstar"
      />
    </EditableSection>
  );
}

// 等级标准编辑 section：受控组件，只渲染 N 行 textarea（无内部 state、无 save bar）
// - levelKeys: 该 section 对应的等级 key 列表（value=[A,B,C]，intent=[S,A,B,C]）
// - draft / onDraftChange: 来自父组件（UploadTab）的受控 draft 状态
// - 保存/取消逻辑全部在父组件的 LevelsSaveBar，section 本身保持纯展示 + 转发编辑事件
function LevelStandardSection({
  title, levelKeys, draft, onDraftChange, testId,
}: {
  title: string;
  levelKeys: readonly string[];
  draft: Levels;
  onDraftChange: (next: Levels) => void;
  testId: string;
}) {
  function setLevel(k: string, v: string) {
    onDraftChange({ ...draft, [k]: v });
  }

  return (
    <section
      data-testid={testId}
      style={{
        background: "white", border: "1px solid var(--border)",
        borderRadius: 8, padding: 12,
      }}
    >
      <h3 style={{ fontSize: 14, fontWeight: 600, color: "var(--text-muted)", marginBottom: 8 }}>
        {title}
      </h3>
      <div style={{ display: "grid", gap: 8 }}>
        {levelKeys.map((k) => (
          <div
            key={k}
            style={{
              display: "grid", gridTemplateColumns: "60px 1fr",
              gap: 12, fontSize: 14, alignItems: "start",
            }}
          >
            <span style={{ color: "var(--text-muted)", paddingTop: 6 }}>{k}</span>
            <textarea
              value={draft[k] ?? ""}
              onChange={(e) => setLevel(k, e.target.value)}
              data-testid={`${testId}-textarea-${k}`}
              rows={2}
              style={{
                padding: "4px 6px",
                border: "1px solid var(--border)",
                borderRadius: 4,
                fontSize: 13,
                fontFamily: "inherit",
                resize: "vertical",
                minHeight: 36,
              }}
            />
          </div>
        ))}
      </div>
    </section>
  );
}

// 两个 level section 共用的底部保存栏：
// - 保存：调用父组件 onSave（UploadTab 内部决定 PATCH 哪些脏 section）
// - 取消：调用父组件 onCancel（UploadTab 把两个 draft 都还原）
// - 已保存提示：savedAt 已设 且 不脏 且 无错
// - 错误提示：error 有值时显示
function LevelsSaveBar({
  busy, error, savedAt, anyDirty, onSave, onCancel,
}: {
  busy: boolean;
  error: string | null;
  savedAt: number | null;
  anyDirty: boolean;
  onSave: () => void;
  onCancel: () => void;
}) {
  const enabled = anyDirty && !busy;
  return (
    <div style={{
      display: "flex", gap: 8, alignItems: "center",
      paddingTop: 8, borderTop: "1px solid var(--border)",
    }}>
      <button
        type="button"
        onClick={onSave}
        disabled={!enabled}
        data-testid="levels-shared-save"
        style={{
          padding: "6px 16px", fontSize: 13, fontWeight: 500,
          background: enabled ? "var(--primary)" : "#e5e7eb",
          color: enabled ? "white" : "#9ca3af",
          border: "none", borderRadius: 4,
          cursor: enabled ? "pointer" : "not-allowed",
        }}
      >
        {busy ? "保存中..." : "保存"}
      </button>
      <button
        type="button"
        onClick={onCancel}
        disabled={!enabled}
        data-testid="levels-shared-cancel"
        style={{
          padding: "6px 16px", fontSize: 13,
          background: "white", color: "var(--text)",
          border: "1px solid var(--border)", borderRadius: 4,
          cursor: !enabled ? "not-allowed" : "pointer",
        }}
      >
        取消
      </button>
      {savedAt && !anyDirty && !error && (
        <span
          style={{ color: "#16a34a", fontSize: 13 }}
          data-testid="levels-shared-saved-msg"
        >
          ✓ 已保存
        </span>
      )}
      {error && (
        <span
          style={{ color: "#991b1b", fontSize: 13 }}
          data-testid="levels-shared-error"
        >
          ✗ {error}
        </span>
      )}
    </div>
  );
}

// 单个上传项：标签 + 隐藏 file input + "选择文件"按钮 + 状态展示
// 前端预校验文件后缀和大小，失败不发请求只 setError；
// 通过则用 fetch POST multipart，后端返回 {ok, path, size}。
function UploadField({
  title, accept, maxSizeMB, endpoint, testId,
}: {
  title: string;
  accept: string;
  maxSizeMB: number;
  endpoint: string;
  testId: string;
}) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [busy, setBusy] = useState(false);
  const [success, setSuccess] = useState<{ name: string; size: number } | null>(null);
  const [error, setError] = useState<string | null>(null);

  // 后端 base 走 CRMQuery 内部；这里也用同源的 /api
  const query = useRef(new CRMQuery()).current;

  function onPickFile() {
    inputRef.current?.click();
  }

  async function onChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    // 清空 input.value 以便下次再选同名文件能触发 onChange
    e.target.value = "";
    if (!file) return;

    setError(null);
    setSuccess(null);

    // 前端预校验：后缀（大小写不敏感）。accept 可以是 ".doc,.docx" 这样的逗号分隔列表。
    const lowerName = file.name.toLowerCase();
    const allowedExts = accept.toLowerCase().split(",").map((s) => s.trim()).filter(Boolean);
    if (!allowedExts.some((ext) => lowerName.endsWith(ext))) {
      setError(`仅支持 ${accept} 格式，当前文件：${file.name}`);
      return;
    }
    // 前端预校验：大小
    if (file.size > maxSizeMB * 1024 * 1024) {
      setError(`文件过大，限制 ${maxSizeMB} MB`);
      return;
    }

    setBusy(true);
    try {
      const isPdf = endpoint.endsWith("attachment-moonstar");
      const res = isPdf
        ? await query.uploadAttachmentMoonstar(file)
        : await query.uploadFaq(file);
      setSuccess({ name: file.name, size: res.size });
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ display: "grid", gap: 6 }}>
      <div style={{ fontSize: 14 }}>
        {title}
        <span style={{ color: "var(--text-muted)", fontSize: 12, marginLeft: 6 }}>
          （≤ {maxSizeMB} MB）
        </span>
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <input
          ref={inputRef}
          type="file"
          accept={accept}
          onChange={onChange}
          data-testid={`${testId}-input`}
          style={{ display: "none" }}
        />
        <button
          type="button"
          onClick={onPickFile}
          disabled={busy}
          data-testid={`${testId}-button`}
          className="btn-secondary"
        >
          {busy ? "上传中..." : "选择文件"}
        </button>
        {busy && (
          <span style={{ fontSize: 13, color: "var(--text-muted)" }}>上传中...</span>
        )}
        {!busy && success && (
          <span
            style={{ fontSize: 13, color: "#16a34a" }}
            data-testid={`${testId}-success`}
          >
            ✓ 已上传（{Math.max(1, Math.round(success.size / 1024))} KB）
          </span>
        )}
        {!busy && error && (
          <span
            style={{ fontSize: 13, color: "#991b1b" }}
            data-testid={`${testId}-error`}
          >
            ✗ {error}
          </span>
        )}
      </div>
    </div>
  );
}

// ----- 公开数据源 tab -----

// 内部 row 类型：UI 用的草稿行。
// originalName: 服务端加载时的 name（空字符串 = 新增行）。
// name: 当前 name（用户可改；originalName !== "" 时视为改名，会在保存时被 reject）。
// url / country / industry / type: 4 个其他字段。
interface SiteDraftRow {
  originalName: string;
  name: string;
  url: string;
  country: string;
  industry: string;
  type: string;
}

// 把后端 Site 列表转成 SiteDraftRow 列表（originalName = name）
function sitesToDraft(sites: Site[]): SiteDraftRow[] {
  return sites.map((s) => ({
    originalName: s.name,
    name: s.name,
    url: s.url ?? "",
    country: s.country ?? "",
    industry: s.industry ?? "",
    type: s.type ?? "",
  }));
}

// 公开数据源 tab：行级 CRUD + 共享保存栏
// - sites: 来自父组件的服务端最新值（null = 加载中）
// - 内部维护 draft；首屏或 sites 变化时同步
// - 搜索：仅 UI 过滤，不发请求（避免边输入边打后端）
// - 保存：diff（added/modified/removed）→ 并发 POST/PATCH/DELETE
// - 取消：draft 还原到服务端最新值
function PublicSourcesTab({
  queryRef, sites, onRefresh,
}: {
  queryRef: React.MutableRefObject<CRMQuery | null>;
  sites: Site[] | null;
  onRefresh: () => Promise<void>;
}) {
  const [draft, setDraft] = useState<SiteDraftRow[]>([]);
  const [search, setSearch] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [savedAt, setSavedAt] = useState<number | null>(null);

  // sites 变化（首屏加载 / 保存后 refresh）→ 重新同步 draft
  useEffect(() => {
    if (sites) setDraft(sitesToDraft(sites));
  }, [sites]);

  // 过滤：name / url / country / industry / type 任一字段包含 q 即显示
  const filtered = (() => {
    if (!search.trim()) return draft;
    const q = search.trim().toLowerCase();
    return draft.filter((r) =>
      r.name.toLowerCase().includes(q) ||
      r.url.toLowerCase().includes(q) ||
      r.country.toLowerCase().includes(q) ||
      r.industry.toLowerCase().includes(q) ||
      r.type.toLowerCase().includes(q)
    );
  })();

  // dirty 检测：与 latest snapshot 对比
  //   - 任何 draft 行有 originalName === "" → 视为新增
  //   - 任何 draft 行有 name/url/country/industry/type 字段改变 → 视为修改
  //   - 任何 originalName 不在 draft 列表里 → 视为删除
  const isDirty = (() => {
    if (!sites) return false;
    const origByName = new Map(sites.map((s) => [s.name, s]));
    // 检查新增 + 修改
    for (const r of draft) {
      if (r.originalName === "") return true; // 新增
      if (r.name !== r.originalName) return true; // 改名
      const orig = origByName.get(r.originalName);
      if (!orig) return true; // 名字改了指向不存在的 record
      if (r.url !== (orig.url ?? "")) return true;
      if (r.country !== (orig.country ?? "")) return true;
      if (r.industry !== (orig.industry ?? "")) return true;
      if (r.type !== (orig.type ?? "")) return true;
    }
    // 检查删除
    for (const s of sites) {
      if (!draft.some((r) => r.originalName === s.name)) return true;
    }
    return false;
  })();

  function updateRow(idx: number, patch: Partial<SiteDraftRow>) {
    setDraft((d) => d.map((r, i) => (i === idx ? { ...r, ...patch } : r)));
    setSavedAt(null);
  }

  function addRow() {
    setDraft((d) => [
      { originalName: "", name: "", url: "", country: "", industry: "", type: "" },
      ...d,
    ]);
    setSavedAt(null);
  }

  function deleteRow(idx: number) {
    setDraft((d) => d.filter((_, i) => i !== idx));
    setSavedAt(null);
  }

  async function save() {
    if (!isDirty || busy || !sites) return;
    setBusy(true);
    setError(null);
    try {
      // 计算三类操作
      const origByName = new Map(sites.map((s) => [s.name, s]));
      const toAdd: Site[] = [];
      const toModify: { oldName: string; patch: Partial<Omit<Site, "name">> }[] = [];
      for (const r of draft) {
        if (r.name.trim() === "" || r.url.trim() === "") {
          throw new Error("name 和 url 不能为空");
        }
        if (r.originalName === "") {
          // 新增
          toAdd.push({
            name: r.name.trim(),
            url: r.url.trim(),
            country: r.country.trim() || undefined,
            industry: r.industry.trim() || undefined,
            type: r.type.trim() || undefined,
          });
        } else {
          // 已在服务端的行：检查改名 + 字段变化
          if (r.name !== r.originalName) {
            throw new Error(`不能修改 name（${r.originalName} → ${r.name}）。如需改名请先删除再新增`);
          }
          const orig = origByName.get(r.originalName);
          if (!orig) {
            throw new Error(`记录已不存在：${r.originalName}`);
          }
          const patch: Partial<Omit<Site, "name">> = {};
          if (r.url !== (orig.url ?? "")) patch.url = r.url;
          if (r.country !== (orig.country ?? "")) patch.country = r.country;
          if (r.industry !== (orig.industry ?? "")) patch.industry = r.industry;
          if (r.type !== (orig.type ?? "")) patch.type = r.type;
          if (Object.keys(patch).length > 0) {
            toModify.push({ oldName: r.originalName, patch });
          }
        }
      }
      // 计算删除：所有 originalName 不在 draft 里的原行
      const draftOriginalNames = new Set(draft.map((r) => r.originalName).filter(Boolean));
      const toDelete = sites.filter((s) => !draftOriginalNames.has(s.name)).map((s) => s.name);

      // 并发派发：add / modify / delete 三类各跑自己的并发
      // 任一失败抛错；失败时服务端可能部分生效（diff 类操作的固有风险）
      const q = queryRef.current!;
      await Promise.all([
        ...toAdd.map((s) => q.addSite(s)),
        ...toModify.map((m) => q.updateSite(m.oldName, m.patch)),
        ...toDelete.map((n) => q.deleteSite(n)),
      ]);
      setSavedAt(Date.now());
      await onRefresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  function cancel() {
    if (sites) setDraft(sitesToDraft(sites));
    setError(null);
    setSavedAt(null);
  }

  if (!sites) {
    return (
      <div style={{ color: "var(--text-muted)", fontSize: 13 }} data-testid="sources-loading">
        公开数据源加载中...
      </div>
    );
  }

  return (
    <div style={{ display: "grid", gap: 16 }}>
      {/* 顶部：搜索 + 添加行 */}
      <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
        <input
          type="text"
          placeholder="搜索 名称 / 链接 / 国家 / 行业 / 内容类型"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          data-testid="sources-search-input"
          style={{
            flex: 1, padding: "6px 8px",
            border: "1px solid var(--border)", borderRadius: 4, fontSize: 13,
          }}
        />
        <button
          type="button"
          onClick={addRow}
          data-testid="sources-add-button"
          className="btn-secondary"
        >
          + 添加行
        </button>
      </div>

      {/* 表头 */}
      <div
        data-testid="sources-header"
        style={{
          display: "grid",
          gridTemplateColumns: "1.2fr 2fr 1fr 1fr 1.3fr 60px",
          gap: 8, padding: "6px 4px",
          fontSize: 12, fontWeight: 600, color: "var(--text-muted)",
          borderBottom: "1px solid var(--border)",
        }}
      >
        <span>名称</span>
        <span>链接</span>
        <span>国家</span>
        <span>行业</span>
        <span>内容类型</span>
        <span></span>
      </div>

      {/* 行列表 */}
      {filtered.length === 0 ? (
        <div style={{ color: "var(--text-muted)", fontSize: 13, padding: 8 }} data-testid="sources-empty">
          {draft.length === 0 ? "暂无数据，点击「+ 添加行」开始" : "无匹配项"}
        </div>
      ) : (
        <div style={{ display: "grid", gap: 6 }} data-testid="sources-rows">
          {filtered.map((r) => {
            // 找到原始 idx（filtered 是过滤后的；删除/编辑需要用原 idx）
            const idx = draft.indexOf(r);
            const isNew = r.originalName === "";
            return (
              <div
                key={`${r.originalName}#${idx}`}
                data-testid={`sources-row-${idx}`}
                style={{
                  display: "grid",
                  gridTemplateColumns: "1.2fr 2fr 1fr 1fr 1.3fr 60px",
                  gap: 8, alignItems: "center",
                }}
              >
                <input
                  type="text"
                  value={r.name}
                  onChange={(e) => updateRow(idx, { name: e.target.value })}
                  data-testid={`sources-row-${idx}-name`}
                  data-new={isNew ? "true" : "false"}
                  placeholder={isNew ? "新名称" : ""}
                  style={cellInputStyle}
                />
                <input
                  type="text"
                  value={r.url}
                  onChange={(e) => updateRow(idx, { url: e.target.value })}
                  data-testid={`sources-row-${idx}-url`}
                  placeholder="https://"
                  style={cellInputStyle}
                />
                <input
                  type="text"
                  value={r.country}
                  onChange={(e) => updateRow(idx, { country: e.target.value })}
                  data-testid={`sources-row-${idx}-country`}
                  style={cellInputStyle}
                />
                <input
                  type="text"
                  value={r.industry}
                  onChange={(e) => updateRow(idx, { industry: e.target.value })}
                  data-testid={`sources-row-${idx}-industry`}
                  style={cellInputStyle}
                />
                <select
                  value={r.type}
                  onChange={(e) => updateRow(idx, { type: e.target.value })}
                  data-testid={`sources-row-${idx}-type`}
                  style={cellSelectStyle}
                >
                  <option value="">--</option>
                  {SITE_TYPE_OPTIONS.map((o) => (
                    <option key={o.value} value={o.value}>{o.label}</option>
                  ))}
                </select>
                <button
                  type="button"
                  onClick={() => deleteRow(idx)}
                  data-testid={`sources-row-${idx}-delete`}
                  className="btn-secondary"
                  style={{ padding: "4px 8px", fontSize: 12 }}
                >
                  删除
                </button>
              </div>
            );
          })}
        </div>
      )}

      {/* 底部：保存 / 取消 */}
      <div style={{
        display: "flex", gap: 8, alignItems: "center",
        paddingTop: 8, borderTop: "1px solid var(--border)",
      }}>
        <button
          type="button"
          onClick={save}
          disabled={!isDirty || busy}
          data-testid="sources-save"
          style={{
            padding: "6px 16px", fontSize: 13, fontWeight: 500,
            background: isDirty && !busy ? "var(--primary)" : "#e5e7eb",
            color: isDirty && !busy ? "white" : "#9ca3af",
            border: "none", borderRadius: 4,
            cursor: isDirty && !busy ? "pointer" : "not-allowed",
          }}
        >
          {busy ? "保存中..." : "保存"}
        </button>
        <button
          type="button"
          onClick={cancel}
          disabled={!isDirty || busy}
          data-testid="sources-cancel"
          style={{
            padding: "6px 16px", fontSize: 13,
            background: "white", color: "var(--text)",
            border: "1px solid var(--border)", borderRadius: 4,
            cursor: isDirty && !busy ? "pointer" : "not-allowed",
          }}
        >
          取消
        </button>
        {savedAt && !isDirty && !error && (
          <span
            style={{ color: "#16a34a", fontSize: 13 }}
            data-testid="sources-saved-msg"
          >
            ✓ 已保存
          </span>
        )}
        {error && (
          <span
            style={{ color: "#991b1b", fontSize: 13 }}
            data-testid="sources-error"
          >
            ✗ {error}
          </span>
        )}
      </div>
    </div>
  );
}

// ----- 微信绑定 tab -----

// 微信绑定 tab：单行 table + 「绑定」按钮 + modal 展示二维码
// 异步协议：POST 立即返 202 + task_id，前端用 GET 轮询拿结果，避免 UI 阻塞 2 分钟。
// 状态机（useState 持有），由 handleBind + interval 回调驱动迁移：
//   - idle             : 未点 / 已结束可重试
//   - submitting       : POST 提交中（按钮显示「启动中...」）
//   - polling          : GET 轮询中（按钮显示「等待生成二维码... (Ns)」）
//   - done             : 轮询到 link/qr 非空 + status 是 done/running/pending，
//                        弹 modal 显示 QR + link（无 warning，流程正常）；
//                        **轮询不停**，继续等 bound=true（用户扫码成功的信号）
//   - doneWithWarning  : 轮询到 link/qr 非空 + status 是 expired/failed（进程被中断
//                        但已抓到 QR），弹 modal 顶部加黄色「进程被中断，二维码可能
//                        还有效」提示让用户试扫；**轮询不停**
//   - success          : 轮询到 bound=true（openclaw 已输出成功标记「已将此
//                        OpenClaw 连接到微信。」），modal 自动关闭，section 内
//                        显示绿色「绑定成功」banner，状态栏变「已绑定」
//   - failed           : link/qr 空 + status 是 failed/expired/404 / 客户端 150s
//                        超时 / POST 失败 / 异常 done（done 但 link/qr 空，兜底）
// 错误信息：失败时显示在按钮下方一行（data-testid="wechat-bind-error"）。
// 成功信息：success 状态时显示在按钮下方一行（data-testid="wechat-bind-success"）。
//
// 关键轮询不停止的语义:done/doneWithWarning 状态时 docker exec 还在跑(openclaw
// 等用户扫码),需要持续 poll 直到 bound=true 或终态。原本实现是 done 就
// clearPollTimer,导致 bound=true 后前端永远收不到通知,用户扫码后界面无反应。
// 新逻辑:进 done 不停轮询,绑 bound=true 切到 success 才停。

// 客户端超时上限：150 秒（2 分半）。
// 理由：后端二维码生成最多 2 分钟；客户端给 30s 缓冲应对网络抖动、modal 关闭后
// 残余轮询等场景。比 130s 多 20s 余量，比 180s 短 30s 避免用户傻等。
const WECHAT_BIND_CLIENT_TIMEOUT_MS = 150_000;

// 轮询间隔：1 秒。后端 5xx/网络错误时由 CRMFetchError 抛出，捕到后保留当前
// BindState（不立即报错），让下次 tick 重试；只在「客户端 150s 超时」或「终态
// (done/failed/expired/404)」时切 BindState。
const WECHAT_BIND_POLL_INTERVAL_MS = 1_000;

type BindState =
  | { kind: "idle" }
  | { kind: "submitting" }
  | { kind: "polling"; taskId: string; elapsed: number }
  | { kind: "done"; result: WechatBindPollResult }
  | { kind: "doneWithWarning"; result: WechatBindPollResult }
  | { kind: "success"; result: WechatBindPollResult }
  | { kind: "failed"; error: string; expired: boolean };

function WechatBindTab({
  queryRef,
}: {
  queryRef: React.MutableRefObject<CRMQuery | null>;
}) {
  const [state, setState] = useState<BindState>({ kind: "idle" });
  // interval id 放在 ref 里（不需要 re-render 触发，effect 清理时统一 clear）
  const pollTimerRef = useRef<number | null>(null);
  // 记录 polling 起点（ms），用于「elapsed」显示与 150s 客户端超时判定
  const pollStartRef = useRef<number>(0);

  // 清理轮询定时器
  function clearPollTimer() {
    if (pollTimerRef.current !== null) {
      window.clearInterval(pollTimerRef.current);
      pollTimerRef.current = null;
    }
  }

  // 组件卸载时清理 interval（避免 setState on unmounted component）
  useEffect(() => {
    return () => clearPollTimer();
  }, []);

  // 启动轮询：1s 一次调 getWechatBindStatus，根据 status 决定停/继续
  // 设计要点：
  //   - 1 个 effect 管：1) elapsed 递增；2) GET 调用；3) 终态判定
  //   - 失败（fetch throw）保留 BindState.polling，下个 tick 重试
  //   - 客户端 150s 超时 → 切 failed，clear interval
  function startPolling(taskId: string) {
    clearPollTimer();
    pollStartRef.current = Date.now();
    setState({ kind: "polling", taskId, elapsed: 0 });
    pollTimerRef.current = window.setInterval(async () => {
      const elapsedMs = Date.now() - pollStartRef.current;
      // 客户端 150s 超时：主动停 interval + 切 failed
      if (elapsedMs >= WECHAT_BIND_CLIENT_TIMEOUT_MS) {
        clearPollTimer();
        setState({
          kind: "failed",
          error: "已超时，请重试",
          expired: true,
        });
        return;
      }
      try {
        const res = await queryRef.current!.getWechatBindStatus(taskId);

        // 0. bound=true (最高优先级):openclaw 已确认连接微信成功
        // (stdout 含「已将此 OpenClaw 连接到微信。」标记)。exec 通常会立即
        // 自然退出,前端切到「绑定成功」状态:modal 自动关闭,inline 显示成功 banner。
        // 这是 done → bound 流程的终态,polling 停在这里。
        if (res.bound === true) {
          clearPollTimer();
          setState({ kind: "success", result: res });
          return;
        }

        // 1. link/qr 已抓到 → 立刻弹 modal,不等 status 终态。
        // 后端"早期发布":docker exec 打印完 link 那一行就 store.Update,无需等到
        // 2 分钟 timeout 才会写。所以 running 状态也可能有 link/qr —— 这种场景
        // 下前端要能立刻展示,而不是按 status="running" 走「更新 elapsed」路径。
        // warning 派生自 status:expired/failed 表示进程被中断/失败,QR 可能失效,
        // 加黄条提示;done/running/pending 表示流程正常,不加 warning。
        //
        // **关键:不 clearPollTimer**。docker exec 还在跑(openclaw 等用户扫码),
        // 需要继续轮询以检测 bound=true(用户扫码成功)。bound 一旦命中,切到
        // success 状态并停轮询。
        if ((res.link ?? "") !== "" || (res.qr ?? "") !== "") {
          const warning = res.status === "expired" || res.status === "failed";
          const targetKind = warning ? "doneWithWarning" : "done";
          setState((s) => {
            // 已在目标状态(避免不必要 re-render);注意 done→doneWithWarning
            // 或反之的过渡,后端 status 可能在 timeout 时从 running 切到 expired
            if (s.kind === targetKind) return s;
            return { kind: targetKind, result: res };
          });
          return;
        }

        // 2. link/qr 空 → 按 status 决定
        if (res.status === "pending" || res.status === "running") {
          // 还在跑:更新 elapsed(按钮文案用),不切 state(避免不必要 re-render)
          setState((s) =>
            s.kind === "polling" ? { ...s, elapsed: Math.floor(elapsedMs / 1000) } : s,
          );
          return;
        }
        if (res.status === "done") {
          // 异常:done 但 link/qr 空 —— 后端契约上不应发生(此处兜底)
          clearPollTimer();
          setState({
            kind: "failed",
            error: "微信绑定完成,但未返回二维码",
            expired: false,
          });
          return;
        }
        if (res.status === "failed") {
          clearPollTimer();
          setState({
            kind: "failed",
            error: res.error ?? "微信绑定失败",
            expired: res.expired ?? false,
          });
          return;
        }
        if (res.status === "expired") {
          // expired + link/qr 空 → 真正失败(docker 没打印 QR 也没 link)
          clearPollTimer();
          setState({
            kind: "failed",
            error: res.error ?? "二维码生成超时",
            expired: true,
          });
          return;
        }
      } catch (e) {
        // 404 单独判:task 不存在
        const err = e as { status?: number; message?: string };
        if (err && err.status === 404) {
          clearPollTimer();
          setState({
            kind: "failed",
            error: "任务已过期，请重试",
            expired: true,
          });
          return;
        }
        // 5xx / 网络错误:保留 polling,下个 tick 重试;elapsed 仍然要更新
        // (用户看到倒计时在动)
        setState((s) =>
          s.kind === "polling" ? { ...s, elapsed: Math.floor(elapsedMs / 1000) } : s,
        );
      }
    }, WECHAT_BIND_POLL_INTERVAL_MS);
  }

  // 关闭 modal：清 modal state + clearPollTimer + fire-and-forget 调 cancel 端点
  // 结束 docker exec 进程组(免等 2 分钟 timeout 兜底)。
  //
  // 取消对 done/doneWithWarning 最关键——exec 还在挂等用户扫码,关闭后必须杀
  // 否则下次点「绑定」会旧 exec 跟新 exec 同时跑。**clearPollTimer 同样关键**:
  // done/doneWithWarning 状态轮询是不停的(等 bound),关闭 modal 必须停轮询,
  // 否则下次 poll tick 会把 state 再切回 done 又把 modal 弹回来。
  //
  // success 状态时 exec 已自然退出或即将退出,调用 cancel 是 best-effort 清理
  // (后端对已终态 task 返 200 + cancelled=false,不会出错)。
  function closeModal() {
    setState((s) => {
      if (s.kind === "done" || s.kind === "doneWithWarning" || s.kind === "success") {
        clearPollTimer();
        // fire-and-forget: 调 cancel 失败也不影响 UI 状态
        queryRef.current!.cancelWechatBind(s.result.task_id).catch(() => {
          // 静默吞掉;常见失败原因:task 已 TTL 清理(404)、网络抖动
        });
        return { kind: "idle" };
      }
      return s;
    });
  }

  // 触发绑定：cancel 当前 polling → POST 拿 task_id → 启轮询
  async function handleBind() {
    // 重新点绑定：清掉旧 interval（如果有）
    clearPollTimer();
    setState({ kind: "submitting" });
    try {
      const sub = await queryRef.current!.bindWechat();
      startPolling(sub.task_id);
    } catch (e) {
      // CRMFetchError 的 message 来自后端 body.error
      const err = e as Error;
      setState({
        kind: "failed",
        error: err?.message ?? String(e),
        expired: false,
      });
    }
  }

  // ESC 键关闭 modal（仅在 modal 打开时挂监听）
  useEffect(() => {
    if (state.kind !== "done" && state.kind !== "doneWithWarning") return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") closeModal();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.kind]);

  // 按钮文案 + disabled 逻辑由 BindState 决定
  const isBusy = state.kind === "submitting" || state.kind === "polling";
  let buttonLabel: string;
  if (state.kind === "submitting") buttonLabel = "启动中...";
  else if (state.kind === "polling") buttonLabel = `等待生成二维码... (${state.elapsed}s)`;
  else buttonLabel = "绑定";

  return (
    <div style={{ display: "grid", gap: 16 }}>
      <section
        data-testid="wechat-bind-section"
        style={{
          background: "white",
          border: "1px solid var(--border)",
          borderRadius: 8,
          padding: 16,
        }}
      >
        <h3 style={{
          fontSize: 14, fontWeight: 600,
          color: "var(--text-muted)", marginBottom: 12,
        }}>
          微信绑定
        </h3>

        <div
          data-testid="wechat-bind-table"
          style={{
            display: "grid",
            gridTemplateColumns: "100px 1fr 120px",
            gap: 12,
            fontSize: 14,
            alignItems: "center",
            padding: "8px 0",
            borderTop: "1px solid var(--border)",
            borderBottom: "1px solid var(--border)",
          }}
        >
          <span style={{ color: "var(--text-muted)" }}>状态</span>
          <span data-testid="wechat-bind-status">
            {state.kind === "success" ? (
              <span style={{ color: "#16a34a", fontWeight: 600 }}>✓ 已绑定</span>
            ) : (
              "未绑定"
            )}
          </span>
          <span style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
            <button
              type="button"
              onClick={handleBind}
              disabled={isBusy}
              data-testid="wechat-bind-button"
              style={{
                padding: "6px 16px",
                fontSize: 13,
                cursor: isBusy ? "not-allowed" : "pointer",
                background: isBusy ? "#9ca3af" : "var(--primary)",
                color: "white",
                border: "none",
                borderRadius: 4,
                opacity: isBusy ? 0.7 : 1,
              }}
            >
              {buttonLabel}
            </button>
          </span>
        </div>

        {state.kind === "failed" && (
          <div
            data-testid="wechat-bind-error"
            style={{
              marginTop: 10, fontSize: 13, color: "#991b1b",
            }}
          >
            ✗ {state.error}
          </div>
        )}

        {/* 绑定进行中提示:用户提交后到拿到 QR/link 期间(可能 2 分钟)不能离开页面,
            否则轮询会断、modal 看不到(组件 unmount 清理 interval)。
            提示用 amber 色文字而非 yellow banner,避免跟 error 视觉权重混淆。 */}
        {(state.kind === "submitting" || state.kind === "polling") && (
          <div
            data-testid="wechat-bind-hint"
            style={{
              marginTop: 10, fontSize: 13, color: "#92400e",
            }}
          >
            ⚠ 绑定过程中请勿离开该页面
          </div>
        )}

        {/* 绑定成功 inline 提示:bound=true 时 polling 自动切到 success 状态,
            modal 关闭,在 section 内显示绿色 banner 告知用户。banner 持久显示
            直到用户点「重新绑定」,不会自动消失——用户需明确知道绑定成功状态。

            两种文案:
              - 新连成功(already_bound 缺失/false):标题「绑定成功」+ 描述 OpenClaw 已连上微信
              - 已连接过(already_bound=true):标题「该用户已绑定」+ 描述 无需重复连接
            两种用同一个 wechat-bind-success testid,文案差异通过 textContent 区分。 */}
        {state.kind === "success" && (
          <div
            data-testid="wechat-bind-success"
            data-already-bound={state.result.already_bound ? "true" : "false"}
            style={{
              marginTop: 10,
              padding: "10px 14px",
              background: "#dcfce7",
              border: "1px solid #86efac",
              borderRadius: 4,
              fontSize: 13,
              color: "#166534",
              lineHeight: 1.5,
            }}
          >
            {state.result.already_bound ? (
              <>
                <div style={{ fontWeight: 600, marginBottom: 2 }}>✓ 该用户已绑定</div>
                <div style={{ fontSize: 12, color: "#15803d" }}>
                  该 OpenClaw 已连接过微信，无需重复连接。如需更换微信账号可再次点击「绑定」按钮。
                </div>
              </>
            ) : (
              <>
                <div style={{ fontWeight: 600, marginBottom: 2 }}>✓ 绑定成功</div>
                <div style={{ fontSize: 12, color: "#15803d" }}>
                  OpenClaw 已连接到微信。后续邮件收发将使用新通道。如需重新绑定可再次点击「绑定」按钮。
                </div>
              </>
            )}
          </div>
        )}
      </section>

      {/* modal 只在 QR 阶段展示。success 状态 modal 自动关闭,inline banner 取而代之。 */}
      {(state.kind === "done" || state.kind === "doneWithWarning") && (
        <WechatBindModal
          result={state.result}
          warning={state.kind === "doneWithWarning"}
          onClose={closeModal}
        />
      )}
    </div>
  );
}

// 微信扫码 modal：
// - 全屏 fixed 居中，遮罩点击关闭
// - 标题 "微信扫码登录"
// - warning=true 时：标题下方加黄色 banner 提示"进程被中断，但二维码可能还有效"
// - 二维码区：<pre> 等宽字体，白底黑字
// - 链接区：可点击的 <a target="_blank">
// - 底部 [关闭] 按钮
// - 角色：dialog；按 ESC 关闭（由父组件 useEffect 挂监听）
// 注:绑定成功(success 状态)不再展示此 modal,而是父组件在 section 内联展示
// wechat-bind-success banner(自动关闭 modal 的视觉效果)。
function WechatBindModal({
  result, warning, onClose,
}: {
  result: WechatBindPollResult;
  // warning=true：后端超时但已抓到 QR/link,modal 顶部加黄色 banner 提示用户试扫
  warning: boolean;
  onClose: () => void;
}) {
  // done 时 link/qr 必有；为类型安全 fallback 空串
  const link = result.link ?? "";
  const qr = result.qr ?? "";
  return (
    <div
      data-testid="wechat-bind-modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="wechat-bind-modal-title"
      onClick={onClose}
      style={{
        position: "fixed",
        inset: 0,
        background: "rgba(0,0,0,0.5)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: "white",
          borderRadius: 8,
          padding: 24,
          width: "min(520px, 92vw)",
          maxHeight: "90vh",
          overflow: "auto",
          boxShadow: "0 10px 40px rgba(0,0,0,0.2)",
        }}
      >
        <h3
          id="wechat-bind-modal-title"
          data-testid="wechat-bind-modal-title"
          style={{ fontSize: 16, fontWeight: 600, marginTop: 0, marginBottom: warning ? 12 : 16 }}
        >
          微信扫码登录
        </h3>

        {warning && (
          <div
            data-testid="wechat-bind-warning"
            style={{
              background: "#fef3c7",
              border: "1px solid #fcd34d",
              borderRadius: 4,
              padding: "8px 12px",
              marginBottom: 16,
              fontSize: 13,
              color: "#92400e",
              lineHeight: 1.5,
            }}
          >
            ⚠ 进程被中断，但二维码可能还有效。请尽快扫码，若失败请重新点击「绑定」。
          </div>
        )}

        {/* 二维码区：等宽字体 + 小字号让 Unicode 块字符能塞下 */}
        <div
          style={{
            display: "flex", justifyContent: "center",
            marginBottom: 16,
          }}
        >
          <pre
            data-testid="wechat-bind-qr"
            style={{
              fontFamily: "monospace",
              lineHeight: 1,
              // Unicode 块字符（▀▄█ 等）每个宽度 ≈ 1 个等宽字符，但实际渲染略宽
              // 10px 是经验值：能在 480px 内塞下 33 列块字符
              fontSize: 10,
              background: "#fff",
              color: "#000",
              padding: 12,
              border: "1px solid var(--border)",
              borderRadius: 4,
              margin: 0,
              overflow: "auto",
              maxWidth: "100%",
            }}
          >
            {qr}
          </pre>
        </div>

        {/* 链接区：可点击新窗口 */}
        <div
          style={{
            fontSize: 13, color: "var(--text-muted)",
            marginBottom: 16, wordBreak: "break-all",
          }}
        >
          或访问链接：
          <a
            href={link}
            target="_blank"
            rel="noopener noreferrer"
            data-testid="wechat-bind-link"
            style={{ color: "var(--primary)", marginLeft: 6 }}
          >
            {link}
          </a>
        </div>

        {/* 关闭按钮 */}
        <div style={{ display: "flex", justifyContent: "flex-end" }}>
          <button
            type="button"
            onClick={onClose}
            data-testid="wechat-bind-modal-close"
            className="btn-secondary"
          >
            关闭
          </button>
        </div>
      </div>
    </div>
  );
}
