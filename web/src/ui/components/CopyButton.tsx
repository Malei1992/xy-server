import { useEffect, useRef, useState } from "react";

// 复制按钮：点击后把 text 写入剪贴板，按钮临时变成 ✅，1.5s 后恢复为复制图标
// - 剪贴板写入失败（无权限 / 非安全上下文）时不显示成功态
// - 卸载时清掉定时器，避免在已卸载组件上 setState
export function CopyButton({
  text, label = "复制",
}: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  const handleClick = async () => {
    const ok = await copyToClipboard(text);
    if (!ok) return;
    setCopied(true);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), 1500);
  };

  return (
    <button
      type="button"
      onClick={handleClick}
      aria-label={copied ? "已复制" : label}
      data-testid="copy-btn"
      data-copied={copied ? "true" : "false"}
      style={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        padding: "2px 6px",
        minWidth: 28,
        height: 22,
        fontSize: 12,
        lineHeight: 1,
        // 与全局主色按钮区分：白底浅蓝边，hover 浅蓝背景
        background: "white",
        color: copied ? "#16a34a" : "var(--primary)",
        border: copied ? "1px solid #16a34a" : "1px solid var(--primary)",
        borderRadius: 4,
        cursor: "pointer",
        flexShrink: 0,
      }}
    >
      {copied ? "✅" : (
        // Feather 风格的复制图标：两张叠在一起的矩形
        <svg
          width="13" height="13" viewBox="0 0 24 24"
          fill="none" stroke="currentColor"
          strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
          aria-hidden="true"
        >
          <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
        </svg>
      )}
    </button>
  );
}

async function copyToClipboard(text: string): Promise<boolean> {
  try {
    if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // 权限被拒 / 非安全上下文等情况
  }
  return false;
}
