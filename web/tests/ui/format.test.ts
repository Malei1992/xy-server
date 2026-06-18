import { describe, it, expect } from "vitest";
import {
  formatList, formatValue, formatDateTime,
  formatTaskType, formatTaskStatus,
} from "@/ui/format";

describe("formatList (— 语义：列表中的空值)", () => {
  it("returns '—' for undefined", () => {
    expect(formatList(undefined)).toBe("—");
  });

  it("returns '—' for empty string", () => {
    expect(formatList("")).toBe("—");
  });

  it("returns single string as-is", () => {
    expect(formatList("lei7_ma@sina.com")).toBe("lei7_ma@sina.com");
  });

  it("joins array with '，'", () => {
    expect(formatList(["a@x.com", "b@x.com"])).toBe("a@x.com，b@x.com");
  });

  it("joins single-element array without separator", () => {
    expect(formatList(["a@x.com"])).toBe("a@x.com");
  });

  it("returns '—' for empty array", () => {
    expect(formatList([])).toBe("—");
  });
});

describe("formatValue (无 语义：详情页 schema 字段缺失)", () => {
  it("returns '无' for undefined", () => {
    expect(formatValue(undefined)).toBe("无");
  });

  it("returns '无' for null", () => {
    expect(formatValue(null)).toBe("无");
  });

  it("returns '无' for empty string", () => {
    expect(formatValue("")).toBe("无");
  });

  it("returns '无' for empty array", () => {
    expect(formatValue([])).toBe("无");
  });

  it("returns string value as-is", () => {
    expect(formatValue("B")).toBe("B");
  });

  it("returns number value as string", () => {
    expect(formatValue(42)).toBe("42");
  });

  it("joins array with '，'", () => {
    expect(formatValue(["a", "b", "c"])).toBe("a，b，c");
  });

  it("returns '无' for object (not rendered here)", () => {
    expect(formatValue({ foo: 1 })).toBe("无");
  });
});

describe("formatDateTime", () => {
  it("returns '无' for undefined", () => {
    expect(formatDateTime(undefined)).toBe("无");
  });

  it("returns '无' for null", () => {
    expect(formatDateTime(null)).toBe("无");
  });

  it("returns '无' for invalid date string", () => {
    expect(formatDateTime("not-a-date")).toBe("无");
  });

  it("returns formatted local time for ISO8601 string", () => {
    const result = formatDateTime("2026-06-01T03:25:42Z");
    expect(result).not.toBe("无");
    expect(result).toMatch(/2026/);
  });
});

describe("formatTaskType", () => {
  it("returns '无' for undefined", () => {
    expect(formatTaskType(undefined)).toBe("无");
  });

  it("returns '无' for null", () => {
    expect(formatTaskType(null)).toBe("无");
  });

  it("returns '无' for empty string", () => {
    expect(formatTaskType("")).toBe("无");
  });

  it("translates all 9 known task types", () => {
    expect(formatTaskType("data_insufficient")).toBe("数据不足");
    expect(formatTaskType("compliance_blocked")).toBe("合规阻断");
    expect(formatTaskType("llm_failure")).toBe("LLM 失败");
    expect(formatTaskType("human_notify_failed")).toBe("人工通知失败");
    expect(formatTaskType("review_timeout")).toBe("审查超时");
    expect(formatTaskType("complex_inquiry")).toBe("复杂咨询");
    expect(formatTaskType("anomaly_alert")).toBe("异常告警");
    expect(formatTaskType("low_confidence")).toBe("低置信度");
    expect(formatTaskType("send_failed")).toBe("发送失败");
  });

  it("returns raw value for unknown task type (forward compat)", () => {
    expect(formatTaskType("future_type_xxx")).toBe("future_type_xxx");
  });
});

describe("formatTaskStatus", () => {
  it("returns '无' for undefined", () => {
    expect(formatTaskStatus(undefined)).toBe("无");
  });

  it("returns '无' for null", () => {
    expect(formatTaskStatus(null)).toBe("无");
  });

  it("returns '无' for empty string", () => {
    expect(formatTaskStatus("")).toBe("无");
  });

  it("translates all 4 known task statuses", () => {
    expect(formatTaskStatus("pending")).toBe("待处理");
    expect(formatTaskStatus("in_progress")).toBe("处理中");
    expect(formatTaskStatus("resolved")).toBe("已解决");
    expect(formatTaskStatus("dismissed")).toBe("已驳回");
  });

  it("returns raw value for unknown task status (forward compat)", () => {
    expect(formatTaskStatus("unknown_state")).toBe("unknown_state");
  });
});
