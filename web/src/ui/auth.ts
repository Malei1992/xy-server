// 登录态 helpers(全部走 localStorage,key 是 crm_logged_in / crm_account)
//
// 设计:登录态只用 localStorage,没有 token 也没有 cookie。
// 退出登录只清这两个 key + 跳 /login(由调用方负责跳转)。
// 任何其他接口都不做鉴权(后端明确不要 token),所以前端只需保证 UI 层 gate。

const KEY_LOGGED_IN = "crm_logged_in";
const KEY_ACCOUNT = "crm_account";

// 当前是否处于登录态:严格比较 crm_logged_in === "true"
// 任何其它值(含空串 / 缺失 / "yes")都视为未登录。
export function isLoggedIn(): boolean {
  return localStorage.getItem(KEY_LOGGED_IN) === "true";
}

// 读取当前登录账号。未登录或 key 缺失时返 null。
export function getLoggedInAccount(): string | null {
  return localStorage.getItem(KEY_ACCOUNT);
}

// 写入登录态:crm_logged_in = "true" + crm_account = <account>
export function setLogin(account: string): void {
  localStorage.setItem(KEY_LOGGED_IN, "true");
  localStorage.setItem(KEY_ACCOUNT, account);
}

// 清除登录态:两个 key 都 removeItem(原本没有也不报错)
export function clearLogin(): void {
  localStorage.removeItem(KEY_LOGGED_IN);
  localStorage.removeItem(KEY_ACCOUNT);
}
