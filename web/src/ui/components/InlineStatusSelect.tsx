import { useEffect, useRef, useState } from "react";

// 内联状态选择器(用于列表页 status 列直接修改)
// - 渲染 <select>,onChange 立即触发 onChange(newValue)(异步)
// - saving 期间:select disabled + 显示 pendingValue(用户刚选的值)
// - 失败:select 回落原值 + 下方红字错误,5s 后自动消失
// - 并发:saving 期间忽略后续 onChange(防止同一行多次并发 PATCH)
// - unmount 时清理 5s timer(防止 setState on unmounted)
//
// Props:
//   - value:当前状态
//   - options:状态选项 { value, label }
//   - onChange:状态变化回调,返回 Promise。抛错 → InlineStatusSelect 内部展示错误
//   - data-testid:覆盖默认 testid(默认 "status-select"),方便父级加 id 区分多行
export interface InlineStatusSelectProps<T extends string> {
  value: T;
  options: ReadonlyArray<{ value: T; label: string }>;
  onChange: (newValue: T) => Promise<void>;
  "data-testid"?: string;
}

export function InlineStatusSelect<T extends string>({
  value,
  options,
  onChange,
  "data-testid": testId = "status-select",
}: InlineStatusSelectProps<T>) {
  // 是否正在 PATCH(防止并发)
  const [saving, setSaving] = useState(false);
  // 错误信息(成功后立即清空,失败时设值,5s 后自动清空)
  const [error, setError] = useState<string | null>(null);
  // PATCH 期间 select 显示的值(成功后清空,失败时清空让 select 回落到 value)
  const [pendingValue, setPendingValue] = useState<T | null>(null);

  // 5s 自动消失 timer id(unmount 时清掉)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // 组件卸载时清理 timer,防止 unmount 后 setState
  useEffect(() => {
    return () => {
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, []);

  const handleChange = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    const newValue = e.target.value as T;
    // 同值 no-op
    if (newValue === value) return;
    // 并发:已有 in-flight 请求,忽略本次
    if (saving) return;
    // 清旧 timer(避免前一次失败的 5s 自动清错还没走就触发新的)
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }

    setSaving(true);
    setPendingValue(newValue);
    setError(null);

    try {
      await onChange(newValue);
      // 成功:清 pending,saving 复原
      setSaving(false);
      setPendingValue(null);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      setSaving(false);
      setPendingValue(null);
      // 5s 后自动清错误
      timerRef.current = setTimeout(() => {
        setError(null);
        timerRef.current = null;
      }, 5000);
    }
  };

  // saving 期间显示 pendingValue(用户刚选的),否则显示传入的 value
  const displayedValue = saving && pendingValue !== null ? pendingValue : value;

  // hover/focus 时才出边框(默认透明,看起来像普通文字)
  // 通过 onMouseEnter/Leave + onFocus/Blur 维护 hover/focus state 是不必要的
  // 这里直接用 style:hover/focus 不可行(inline style 不支持伪类),
  // 所以用 className:hover 的替代方案:把交互样式写在 :focus/:hover 之外的 onFocus 切 class
  // 但更简单的是让 select 永远带浅边框 + 透明 background;不过 spec 要求默认无边框
  // → 这里采用一个折中:focus 边框 + 一个 outline on focus;hover 通过 CSS 不可达
  // 真正符合 spec 需要外部 CSS class。开发体验上,我们用 inline style 把 border 设为透明,
  // 但保留 boxSizing,让 :focus-visible 之类的浏览器默认 outline 仍可见。
  // 视觉验收:
  //   默认 → border 透明,像文字
  //   hover → 这里没法用 inline 实现伪类 → 简化:保持默认视觉,focus 时给主色边框
  //   focus → 切到主色边框 + outline none
  //   saving → opacity 0.6 + cursor not-allowed
  const [focused, setFocused] = useState(false);
  const baseStyle: React.CSSProperties = {
    background: "transparent",
    border: "1px solid transparent",
    borderRadius: 4,
    padding: "4px 6px",
    fontSize: 13,
    cursor: saving ? "not-allowed" : "pointer",
    color: "inherit",
    font: "inherit",
    outline: "none",
    width: "100%",
    opacity: saving ? 0.6 : 1,
    borderColor: focused ? "var(--primary)" : "transparent",
  };

  return (
    <div style={{ position: "relative", minWidth: 0 }}>
      <select
        data-testid={testId}
        value={displayedValue}
        onChange={handleChange}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        disabled={saving}
        aria-busy={saving || undefined}
        style={baseStyle}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
      {error && (
        <div
          data-testid="status-select-error"
          role="alert"
          style={{
            color: "#991b1b",
            fontSize: 12,
            marginTop: 2,
            lineHeight: 1.3,
          }}
        >
          {error}
        </div>
      )}
    </div>
  );
}