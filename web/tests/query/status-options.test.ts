import { describe, it, expect } from "vitest";
import {
  PROJECT_STATUS_OPTIONS,
  TASK_STATUS_OPTIONS,
  OPPORTUNITY_STATUS_OPTIONS,
} from "@/query/types";

// 3 个 STATUS_OPTIONS 常量是 STATUS_EDIT_MODAL 的下拉数据源 + 列表页的展示源
// 顺序 / value / label 都按 spec 2026-06-22 表格定。

describe("PROJECT_STATUS_OPTIONS", () => {
  it("contains 5 entries in spec order", () => {
    expect(PROJECT_STATUS_OPTIONS.map((o) => o.value)).toEqual([
      "跟进中",
      "谈判中",
      "签约中",
      "已落地",
      "已关闭",
    ]);
  });

  it("labels match values (项目状态本身就是中文)", () => {
    for (const o of PROJECT_STATUS_OPTIONS) {
      expect(o.label).toBe(o.value);
    }
  });
});

describe("TASK_STATUS_OPTIONS", () => {
  it("contains 4 entries in spec order (英文 enum, 中文 label)", () => {
    expect(TASK_STATUS_OPTIONS.map((o) => o.value)).toEqual([
      "pending",
      "in_progress",
      "resolved",
      "dismissed",
    ]);
    expect(TASK_STATUS_OPTIONS.map((o) => o.label)).toEqual([
      "待处理",
      "处理中",
      "已解决",
      "已驳回",
    ]);
  });
});

describe("OPPORTUNITY_STATUS_OPTIONS", () => {
  it("contains 4 entries in spec order", () => {
    expect(OPPORTUNITY_STATUS_OPTIONS.map((o) => o.value)).toEqual([
      "待评估",
      "跟进中",
      "已转化",
      "已关闭",
    ]);
  });

  it("labels match values (公开信息状态本身就是中文)", () => {
    for (const o of OPPORTUNITY_STATUS_OPTIONS) {
      expect(o.label).toBe(o.value);
    }
  });
});
