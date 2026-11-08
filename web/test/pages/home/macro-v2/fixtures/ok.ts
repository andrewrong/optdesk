import type { MacroSummary } from "../../../../../src/generated/macro_summary.v2"

export const ok = {
  "build_id": "build-demo-0001",
  "card": {
    "data_times_text": "松紧度数据日 10-23；事件日期按发布日历，无发布时间。",
    "ease": {
      "aria_text": "松紧度 +0.2，中性，较上周 −0.2；固定刻度 -2 到 +2，中性带 -0.5 到 +0.5。",
      "figure": {
        "text": "+0.2",
        "unit": "index_points",
        "value": 0.2
      },
      "marker_percent": {
        "text": "55",
        "unit": "percent_of_track",
        "value": 55
      },
      "neutral_band": {
        "high": {
          "text": "+0.5",
          "unit": "percent_of_track",
          "value": 62.5
        },
        "low": {
          "text": "-0.5",
          "unit": "percent_of_track",
          "value": 37.5
        }
      },
      "reading_text": "当前 +0.2，中性；固定刻度 -2 到 +2。",
      "ticks": [
        "-2",
        "-1",
        "0",
        "+1",
        "+2"
      ],
      "week_change": {
        "text": "−0.2",
        "unit": "index_points",
        "value": -0.2
      }
    },
    "events": [
      {
        "date": "2026-10-28",
        "name_text": "美国 CPI",
        "previous": "3.0%（9月）",
        "when_text": "周三 10-28"
      },
      {
        "date": "2026-11-06",
        "name_text": "美国非农就业",
        "previous": "+12.0 万（9月）",
        "when_text": "周五 11-06"
      },
      {
        "date": "2026-11-17",
        "name_text": "美国零售销售",
        "previous": {
          "reason": "no_data",
          "text": "前值暂缺"
        },
        "when_text": "周二 11-17"
      }
    ],
    "gaps": [
      {
        "reason": "no_data",
        "text": "前值暂缺"
      }
    ],
    "notes": [
      "日期来自 FRED 发布日历和央行日程，只有日期，没有发布时间。",
      "前值是序列最新存储的一期，标注所属月份；没有存储或没有对应序列的标为暂缺。"
    ],
    "say": "松紧度中性，较上周 −0.2。本周的事件是 CPI。",
    "stale": {
      "data_time": "2026-10-23T00:00:00+08:00",
      "stale_after": "2026-10-27T09:30:00+08:00",
      "text": "数据日 10-23"
    },
    "title": "宏观"
  },
  "data_times": [
    {
      "data_time": "2026-10-23T00:00:00+08:00",
      "source": "macro_ease"
    }
  ],
  "generated_at": "2026-10-26T09:30:00+08:00",
  "run_id": "run-demo-0001",
  "run_status": "partial",
  "schema": "macro_summary",
  "schema_version": 2,
  "scope": "home-macro"
} satisfies MacroSummary
