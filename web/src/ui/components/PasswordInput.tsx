import { useState } from "react";

// 密码输入框:受控组件,内部维护 "是否明文显示" 状态
//
// Props:
//   - value / onChange:标准受控 input 约定
//   - placeholder / autoComplete:透传给 input
//   - data-testid:外部可覆盖 input 的 testid(默认 "password-input");
//     toggle 按钮的 testid 始终是 "password-toggle",不受外部 data-testid 影响
//
// 行为:
//   - 默认 type="password"(圆点遮罩)
//   - 右侧 toggle 按钮(眼睛图标)点击 → 在 "password" / "text" 之间切 type
//   - 输入本身始终受控:不接管 onChange 的语义,纯转发

export function PasswordInput({
  value,
  onChange,
  placeholder,
  autoComplete,
  "data-testid": testId = "password-input",
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  autoComplete?: string;
  "data-testid"?: string;
}) {
  // visible=true → type="text" 明文;false → "password" 圆点
  const [visible, setVisible] = useState(false);

  return (
    <span
      style={{
        // 让 input 和 toggle 同行排列,toggle 绝对定位在右侧
        position: "relative",
        display: "inline-block",
        width: "100%",
      }}
    >
      <input
        type={visible ? "text" : "password"}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        autoComplete={autoComplete}
        data-testid={testId}
        // 给 toggle 按钮留出右侧空间,避免文字被眼睛图标遮住
        style={{
          width: "100%",
          padding: "8px 36px 8px 10px",
          border: "1px solid var(--border)",
          borderRadius: 4,
          fontSize: 14,
          fontFamily: "monospace",
        }}
      />
      <button
        type="button"
        onClick={() => setVisible((v) => !v)}
        // 阻止 button 抢走 form 默认 submit(若以后嵌入 form)
        // 同时阻止 mousedown 让 input 失焦
        onMouseDown={(e) => e.preventDefault()}
        data-testid="password-toggle"
        data-visible={visible ? "true" : "false"}
        aria-label={visible ? "隐藏密码" : "显示密码"}
        aria-pressed={visible}
        style={{
          position: "absolute",
          right: 4,
          top: "50%",
          transform: "translateY(-50%)",
          padding: "2px 6px",
          background: "transparent",
          border: "none",
          // 不与全局 button 主题冲突:背景透明、字色用灰色
          color: "var(--text-muted)",
          cursor: "pointer",
          fontSize: 14,
          lineHeight: 1,
        }}
      >
        {visible ? (
          // 显示态(密码可见):睁眼 — 眼睑弧线 + 中心瞳孔
          <svg
            width="16"
            height="16"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="M1 8s2.5-5 7-5 7 5 7 5-2.5 5-7 5-7-5-7-5z" />
            <circle cx="8" cy="8" r="2" />
          </svg>
        ) : (
          // 隐藏态(密码不可见):眼睛被斜线划掉 — 眼睑弧线 + 中心瞳孔 + 斜杠
          <svg
            width="16"
            height="16"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="M1 8s2.5-5 7-5 7 5 7 5-2.5 5-7 5-7-5-7-5z" />
            <circle cx="8" cy="8" r="2" />
            <line x1="2" y1="14" x2="14" y2="2" />
          </svg>
        )}
      </button>
    </span>
  );
}
