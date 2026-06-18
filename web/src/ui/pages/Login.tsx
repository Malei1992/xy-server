import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { login } from "@/query/users";
import { setLogin } from "@/ui/auth";
import { PasswordInput } from "@/ui/components/PasswordInput";
import { CRMFetchError } from "@/query/loader";

// 登录页:账号 + 密码 + 登录按钮
// - 账号/密码都填了才能点登录(任一为空按钮 disabled)
// - 提交后调 login():
//     200 → setLogin(account) + navigate("/")
//     401 → 内联展示后端 error(默认"账号或密码错误")
//     404 → 内联展示后端 error(默认"账号不存在")
//     400/500 → 展示后端 error 或 fallback 文案
// - 提交中按钮 disabled,避免重复提交
// - 错误清除:用户继续编辑任一输入时清掉旧错误

export function Login() {
  const navigate = useNavigate();
  const [account, setAccount] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // 按钮启用条件:两个都非空 + 不在 busy
  const canSubmit = account.trim() !== "" && password !== "" && !busy;

  function setAccountAndClearError(v: string) {
    setAccount(v);
    if (error) setError(null);
  }
  function setPasswordAndClearError(v: string) {
    setPassword(v);
    if (error) setError(null);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    setBusy(true);
    setError(null);
    try {
      const res = await login({ account: account.trim(), password });
      setLogin(res.account);
      // 跳到首页(按现有 routes 行为,/ 会被重定向到 /customers)
      navigate("/", { replace: true });
    } catch (e) {
      // 后端 error 文案优先(已在 query/users 提取到 CRMFetchError.message)
      const msg = e instanceof CRMFetchError
        ? e.message
        : e instanceof Error
        ? e.message
        : String(e);
      setError(msg || "登录失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "var(--bg)",
      }}
    >
      <form
        onSubmit={handleSubmit}
        data-testid="login-form"
        style={{
          width: 360,
          padding: 28,
          background: "white",
          border: "1px solid var(--border)",
          borderRadius: 8,
          boxShadow: "0 4px 16px rgba(0,0,0,0.04)",
          display: "grid",
          gap: 14,
        }}
      >
        <h2
          style={{
            fontSize: 20,
            fontWeight: 600,
            color: "var(--primary)",
            marginBottom: 4,
            textAlign: "center",
          }}
        >
          星月管理系统
        </h2>

        <label style={fieldStyle}>
          <span style={labelStyle}>账号</span>
          <input
            type="text"
            value={account}
            onChange={(e) => setAccountAndClearError(e.target.value)}
            placeholder="请输入账号"
            autoComplete="username"
            data-testid="account-input"
            style={inputStyle}
          />
        </label>

        <label style={fieldStyle}>
          <span style={labelStyle}>密码</span>
          <PasswordInput
            value={password}
            onChange={setPasswordAndClearError}
            placeholder="请输入密码"
            autoComplete="current-password"
          />
        </label>

        {error && (
          <div
            data-testid="login-error"
            role="alert"
            style={{
              fontSize: 13,
              color: "#991b1b",
              background: "#fef2f2",
              border: "1px solid #fecaca",
              borderRadius: 4,
              padding: "6px 10px",
            }}
          >
            ✗ {error}
          </div>
        )}

        <button
          type="submit"
          data-testid="login-submit"
          disabled={!canSubmit}
          style={{
            marginTop: 4,
            padding: "10px 0",
            fontSize: 14,
            fontWeight: 500,
            background: canSubmit ? "var(--primary)" : "#e5e7eb",
            color: canSubmit ? "white" : "#9ca3af",
            border: "none",
            borderRadius: 4,
            cursor: canSubmit ? "pointer" : "not-allowed",
          }}
        >
          {busy ? "登录中..." : "登录"}
        </button>
      </form>
    </div>
  );
}

const fieldStyle: React.CSSProperties = {
  display: "grid",
  gap: 4,
  fontSize: 14,
};
const labelStyle: React.CSSProperties = {
  color: "var(--text-muted)",
  fontSize: 13,
};
const inputStyle: React.CSSProperties = {
  padding: "8px 10px",
  border: "1px solid var(--border)",
  borderRadius: 4,
  fontSize: 14,
  fontFamily: "inherit",
};
