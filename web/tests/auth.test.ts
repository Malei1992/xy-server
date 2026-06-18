import { describe, it, expect, beforeEach } from "vitest";
import { isLoggedIn, getLoggedInAccount, setLogin, clearLogin } from "../src/ui/auth";

// localStorage helpers 4 个函数:
//   - isLoggedIn(): boolean        看 crm_logged_in === "true"
//   - getLoggedInAccount(): string | null  读 crm_account(没有时返 null)
//   - setLogin(account): void      写两个 key
//   - clearLogin(): void           清两个 key

const K_LOGGED = "crm_logged_in";
const K_ACCOUNT = "crm_account";

beforeEach(() => {
  // 每个测试前都重置 localStorage
  localStorage.clear();
});

describe("isLoggedIn", () => {
  it("初始时(localStorage 空)返 false", () => {
    expect(isLoggedIn()).toBe(false);
  });

  it("crm_logged_in = 'true' 时返 true", () => {
    localStorage.setItem(K_LOGGED, "true");
    expect(isLoggedIn()).toBe(true);
  });

  it("crm_logged_in = 其它字符串 时返 false", () => {
    localStorage.setItem(K_LOGGED, "yes");
    expect(isLoggedIn()).toBe(false);
    localStorage.setItem(K_LOGGED, "1");
    expect(isLoggedIn()).toBe(false);
    localStorage.setItem(K_LOGGED, "");
    expect(isLoggedIn()).toBe(false);
  });
});

describe("getLoggedInAccount", () => {
  it("无 crm_account 时返 null", () => {
    expect(getLoggedInAccount()).toBeNull();
  });

  it("有 crm_account 时返该字符串", () => {
    localStorage.setItem(K_ACCOUNT, "alice");
    expect(getLoggedInAccount()).toBe("alice");
  });
});

describe("setLogin", () => {
  it("同时写 crm_logged_in='true' 和 crm_account=<account>", () => {
    setLogin("admin");
    expect(localStorage.getItem(K_LOGGED)).toBe("true");
    expect(localStorage.getItem(K_ACCOUNT)).toBe("admin");
  });

  it("setLogin 之后 isLoggedIn() 和 getLoggedInAccount() 联动正确", () => {
    setLogin("bob");
    expect(isLoggedIn()).toBe(true);
    expect(getLoggedInAccount()).toBe("bob");
  });

  it("setLogin 重复调用会以最后一次写入为准", () => {
    setLogin("a");
    setLogin("b");
    expect(getLoggedInAccount()).toBe("b");
  });
});

describe("clearLogin", () => {
  it("清掉两个 key,清后 isLoggedIn()=false / getLoggedInAccount()=null", () => {
    setLogin("admin");
    clearLogin();
    expect(isLoggedIn()).toBe(false);
    expect(getLoggedInAccount()).toBeNull();
    expect(localStorage.getItem(K_LOGGED)).toBeNull();
    expect(localStorage.getItem(K_ACCOUNT)).toBeNull();
  });

  it("在原本就没有登录态时调用也不报错", () => {
    expect(() => clearLogin()).not.toThrow();
  });
});
