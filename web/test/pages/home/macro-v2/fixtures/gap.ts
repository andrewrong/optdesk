import type { MacroSummary } from "../../../../../src/generated/macro_summary.v2"

export const gap = {
  "build_id": "build-demo-0001",
  "card": {
    "data_times_text": "松紧度数据日 无；事件日期按发布日历，无发布时间。",
    "ease": {
      "reason": "no_data",
      "text": "松紧度暂无数据：没有存储的读数"
    },
    "events": [
      {
        "date": "2026-10-28",
        "name_text": "美国 CPI",
        "previous": {
          "reason": "no_data",
          "text": "前值暂缺"
        },
        "when_text": "周三 10-28"
      },
      {
        "date": "2026-11-06",
        "name_text": "美国非农就业",
        "previous": {
          "reason": "no_data",
          "text": "前值暂缺"
        },
        "when_text": "周五 11-06"
      }
    ],
    "gaps": [
      {
        "reason": "no_data",
        "text": "松紧度暂无数据：没有存储的读数"
      },
      {
        "reason": "no_data",
        "text": "前值暂缺"
      }
    ],
    "notes": [
      "日期来自 FRED 发布日历和央行日程，只有日期，没有发布时间。",
      "前值是序列最新存储的一期，标注所属月份；没有存储或没有对应序列的标为暂缺。"
    ],
    "say": "松紧度暂无数据。本周的事件是 CPI。",
    "stale": {
      "data_time": "2026-10-26T09:30:00+08:00",
      "stale_after": "2026-10-27T09:30:00+08:00",
      "text": "没有数据日"
    },
    "title": "宏观"
  },
  "data_times": [],
  "generated_at": "2026-10-26T09:30:00+08:00",
  "run_id": "run-demo-0001",
  "run_status": "partial",
  "schema": "macro_summary",
  "schema_version": 2,
  "scope": "home-macro"
} satisfies MacroSummary
