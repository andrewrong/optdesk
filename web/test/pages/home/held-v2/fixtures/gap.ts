// Synthetic payload written from contracts/schemas/page.held_companies_summary.v3.json.
import type { HeldCompaniesSummary } from '../../../../../src/generated/held_companies_summary.v3'

export const gap = {
  "build_id": "build-demo-0001",
  "card": {
    "columns": [
      {
        "id": "company",
        "label": "公司"
      },
      {
        "id": "price",
        "label": "价格"
      },
      {
        "id": "earnings",
        "label": "财报日"
      },
      {
        "id": "day_change",
        "label": "日涨跌"
      },
      {
        "id": "five_day_change",
        "label": "5日涨跌"
      }
    ],
    "data_times_text": "数据日 10-23",
    "gaps": [
      {
        "kind": "gap",
        "reason": "missing_input",
        "text": "持仓公司数据不足：原因未知"
      }
    ],
    "notes": [
      "财报日只显示日期：盘前、盘后时间暂无数据源。"
    ],
    "rows": [],
    "say": "持仓公司数据不足：原因未知",
    "sharp_note": "日涨跌 3% 及以上、5日涨跌 8% 及以上标“大幅”。",
    "stale": {
      "data_time": "2026-10-23T21:00:00+08:00",
      "is_stale": false,
      "text": "数据日 10-23"
    },
    "subtitle": "财报日历与变化",
    "title": "持仓公司"
  },
  "data_times": [],
  "generated_at": "2026-10-23T21:00:00+08:00",
  "run_id": "run-demo-0001",
  "run_status": "partial",
  "schema": "held_companies_summary",
  "schema_version": 3,
  "scope": "home",
  "stale_after": "2026-10-24T21:00:00+08:00"
} satisfies HeldCompaniesSummary
