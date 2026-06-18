import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { listUsers, createUser, changePassword } from "@/query/users";
import type { UserListItem } from "@/query/types";
import { clearLogin, getLoggedInAccount } from "@/ui/auth";
import { PasswordInput } from "@/ui/components/PasswordInput";

// 用户管理页:
// - 进入页面自动调 listUsers
// - 顶部按钮:「新增用户」「退出登录」
// - 表格:每行账号 + 「修改密码」按钮
// - 新增 / 修改密码 都在弹窗内完成
// - 弹窗的取消 / 提交 / 校验失败 / 后端错误都按现有 Settings 风格内联展示
//
// 弹窗的 testid:
//   add-user-modal / add-user-account / add-user-password / add-user-submit /
//   add-user-cancel / add-user-error
//   change-pw-modal / change-pw-old / change-pw-new / change-pw-confirm /
//   change-pw-submit / change-pw-cancel / change-pw-error / change-pw-mismatch
//   user-row-<account> / change-pw-button-<account>

export function Users() {
  const navigate = useNavigate();
  const [users, setUsers] = useState<UserListItem[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [changeTarget, setChangeTarget] = useState<string | null>(null);

  async function refresh() {
    try {
      const list = await listUsers();
      setUsers(list);
      setLoadError(null);
    } catch (e) {
      setLoadError(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  function handleLogout() {
    clearLogin();
    navigate("/login", { replace: true });
  }

  return (
    <div style={{ padding: 24, maxWidth: 720 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 16 }}>
        <h2 style={{ fontSize: 18, margin: 0 }}>用户管理</h2>
        <div style={{ display: "flex", gap: 8 }}>
          <button
            type="button"
            data-testid="add-user-button"
            onClick={() => setAddOpen(true)}
            style={{ padding: "6px 16px", fontSize: 13 }}
          >
            + 新增用户
          </button>
          <button
            type="button"
            data-testid="logout-button"
            onClick={handleLogout}
            className="btn-secondary"
            style={{ padding: "6px 16px", fontSize: 13 }}
          >
            退出登录
          </button>
        </div>
      </div>

      {loadError && (
        <div
          data-testid="users-error"
          style={{
            padding: 10,
            background: "#fef2f2",
            border: "1px solid #fecaca",
            borderRadius: 4,
            color: "#991b1b",
            fontSize: 13,
            marginBottom: 12,
          }}
        >
          ✗ 加载失败:{loadError}
        </div>
      )}

      {!loadError && users && users.length === 0 && (
        <div
          data-testid="users-empty"
          style={{ padding: 24, color: "var(--text-muted)", fontSize: 13, textAlign: "center" }}
        >
          暂无用户
        </div>
      )}

      {users && users.length > 0 && (
        <table style={{ width: "100%" }}>
          <thead>
            <tr>
              <th>账号</th>
              <th style={{ width: 140 }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => {
              const isSelf = u.account === getLoggedInAccount();
              return (
                <tr key={u.account} data-testid={`user-row-${u.account}`}>
                  <td>{u.account}{isSelf && <span style={{ marginLeft: 6, color: "var(--text-muted)", fontSize: 12 }}>(我)</span>}</td>
                  <td>
                    <button
                      type="button"
                      data-testid={`change-pw-button-${u.account}`}
                      onClick={() => setChangeTarget(u.account)}
                      className="btn-secondary"
                      style={{ padding: "4px 10px", fontSize: 12 }}
                    >
                      修改密码
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}

      {addOpen && (
        <AddUserModal
          onClose={() => setAddOpen(false)}
          onCreated={async () => {
            setAddOpen(false);
            await refresh();
          }}
        />
      )}

      {changeTarget && (
        <ChangePasswordModal
          account={changeTarget}
          onClose={() => setChangeTarget(null)}
          onChanged={() => {
            setChangeTarget(null);
          }}
        />
      )}
    </div>
  );
}

// ----- 新增用户弹窗 -----
// 字段:账号(普通 text input) + 密码(PasswordInput)
// 提交按钮:账号密码都非空才 enabled
// 成功后:onCreated() → 父组件关弹窗 + 刷新列表
function AddUserModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void | Promise<void> }) {
  const [account, setAccount] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const canSubmit = account.trim() !== "" && password !== "" && !busy;

  async function submit() {
    if (!canSubmit) return;
    setBusy(true);
    setError(null);
    try {
      await createUser({ account: account.trim(), password });
      await onCreated();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <ModalOverlay testId="add-user-modal" title="新增用户" onClose={onClose}>
      <ModalBody>
        <ModalField label="账号">
          <input
            type="text"
            value={account}
            onChange={(e) => setAccount(e.target.value)}
            placeholder="账号(英文/数字/下划线,1-20 字符)"
            autoComplete="off"
            data-testid="add-user-account"
            style={modalInputStyle}
          />
        </ModalField>
        <ModalField label="密码">
          <PasswordInput
            value={password}
            onChange={setPassword}
            placeholder="密码(1-20 字符)"
            autoComplete="new-password"
            data-testid="add-user-password"
          />
        </ModalField>
        {error && <ModalError testId="add-user-error" message={error} />}
      </ModalBody>
      <ModalFooter
        submitTestId="add-user-submit"
        cancelTestId="add-user-cancel"
        onSubmit={submit}
        onCancel={onClose}
        submitDisabled={!canSubmit}
        submitLabel={busy ? "提交中..." : "创建"}
      />
    </ModalOverlay>
  );
}

// ----- 修改密码弹窗 -----
// 字段:旧密码 + 新密码 + 确认新密码(都 PasswordInput)
// 提交按钮:三栏都非空 + newPassword === confirmNewPassword 才 enabled
// 成功后:onChanged() → 父组件关弹窗
function ChangePasswordModal({
  account, onClose, onChanged,
}: {
  account: string;
  onClose: () => void;
  onChanged: () => void;
}) {
  const [oldPassword, setOld] = useState("");
  const [newPassword, setNew] = useState("");
  const [confirmNew, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // 第一次进来自动 focus 第一个输入
  const firstRef = useRef<HTMLInputElement | null>(null);

  const mismatch = newPassword !== "" && confirmNew !== "" && newPassword !== confirmNew;
  const canSubmit = !mismatch && oldPassword !== "" && newPassword !== "" && confirmNew !== "" && !busy;

  async function submit() {
    if (!canSubmit) return;
    setBusy(true);
    setError(null);
    try {
      await changePassword(account, {
        oldPassword,
        newPassword,
        confirmNewPassword: confirmNew,
      });
      onChanged();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <ModalOverlay
      testId="change-pw-modal"
      title={`修改密码：${account}`}
      onClose={onClose}
      onMounted={() => firstRef.current?.focus()}
    >
      <ModalBody>
        <ModalField label="旧密码">
          <input
            ref={firstRef}
            type="password"
            value={oldPassword}
            onChange={(e) => setOld(e.target.value)}
            placeholder="旧密码"
            autoComplete="current-password"
            data-testid="change-pw-old"
            style={modalInputStyle}
          />
        </ModalField>
        <ModalField label="新密码">
          <PasswordInput
            value={newPassword}
            onChange={setNew}
            placeholder="新密码(1-20 字符)"
            autoComplete="new-password"
            data-testid="change-pw-new"
          />
        </ModalField>
        <ModalField label="确认新密码">
          <PasswordInput
            value={confirmNew}
            onChange={setConfirm}
            placeholder="再输一次新密码"
            autoComplete="new-password"
            data-testid="change-pw-confirm"
          />
        </ModalField>
        {mismatch && (
          <div
            data-testid="change-pw-mismatch"
            style={{ fontSize: 12, color: "#991b1b", marginTop: -4 }}
          >
            ✗ 两次新密码不一致
          </div>
        )}
        {error && <ModalError testId="change-pw-error" message={error} />}
      </ModalBody>
      <ModalFooter
        submitTestId="change-pw-submit"
        cancelTestId="change-pw-cancel"
        onSubmit={submit}
        onCancel={onClose}
        submitDisabled={!canSubmit}
        submitLabel={busy ? "提交中..." : "确认修改"}
      />
    </ModalOverlay>
  );
}

// ----- 模态框通用组件 -----
// 与现有 Settings 风格一致:fixed 全屏遮罩 + 居中白卡 + 遮罩点击关闭
// 简化:取消 / 提交 按钮在 footer 行;title 在 header
function ModalOverlay({
  testId, title, onClose, onMounted, children,
}: {
  testId: string;
  title: string;
  onClose: () => void;
  onMounted?: () => void;
  children: React.ReactNode;
}) {
  useEffect(() => {
    onMounted?.();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps
  return (
    <div
      data-testid={testId}
      role="dialog"
      aria-modal="true"
      onClick={onClose}
      style={{
        position: "fixed", inset: 0,
        background: "rgba(0,0,0,0.5)",
        display: "flex", alignItems: "center", justifyContent: "center",
        zIndex: 1000,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: "white", borderRadius: 8, padding: 24,
          width: "min(420px, 92vw)",
          maxHeight: "90vh", overflow: "auto",
          boxShadow: "0 10px 40px rgba(0,0,0,0.2)",
        }}
      >
        <h3 style={{ fontSize: 16, fontWeight: 600, marginTop: 0, marginBottom: 16 }}>{title}</h3>
        {children}
      </div>
    </div>
  );
}

function ModalBody({ children }: { children: React.ReactNode }) {
  return <div style={{ display: "grid", gap: 12 }}>{children}</div>;
}

function ModalField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label style={{ display: "grid", gap: 4, fontSize: 13 }}>
      <span style={{ color: "var(--text-muted)" }}>{label}</span>
      {children}
    </label>
  );
}

function ModalError({ testId, message }: { testId: string; message: string }) {
  return (
    <div
      data-testid={testId}
      style={{
        fontSize: 12, color: "#991b1b",
        background: "#fef2f2", border: "1px solid #fecaca",
        borderRadius: 4, padding: "6px 10px",
      }}
    >
      ✗ {message}
    </div>
  );
}

function ModalFooter({
  submitTestId, cancelTestId, onSubmit, onCancel, submitDisabled, submitLabel,
}: {
  submitTestId: string;
  cancelTestId: string;
  onSubmit: () => void;
  onCancel: () => void;
  submitDisabled: boolean;
  submitLabel: string;
}) {
  return (
    <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 20 }}>
      <button
        type="button"
        data-testid={cancelTestId}
        onClick={onCancel}
        className="btn-secondary"
        style={{ padding: "6px 16px", fontSize: 13 }}
      >
        取消
      </button>
      <button
        type="button"
        data-testid={submitTestId}
        onClick={onSubmit}
        disabled={submitDisabled}
        style={{
          padding: "6px 16px", fontSize: 13,
          background: submitDisabled ? "#e5e7eb" : "var(--primary)",
          color: submitDisabled ? "#9ca3af" : "white",
          border: "none", borderRadius: 4,
          cursor: submitDisabled ? "not-allowed" : "pointer",
        }}
      >
        {submitLabel}
      </button>
    </div>
  );
}

const modalInputStyle: React.CSSProperties = {
  padding: "8px 10px",
  border: "1px solid var(--border)",
  borderRadius: 4,
  fontSize: 14,
  fontFamily: "inherit",
};
