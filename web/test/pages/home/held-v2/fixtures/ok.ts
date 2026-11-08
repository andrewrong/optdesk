// Synthetic payload written from contracts/schemas/page.held_companies_summary.v3.json.
import type { HeldCompaniesSummary } from '../../../../../src/generated/held_companies_summary.v3'

export const ok = {
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
    "data_times_text": "美股数据日 10-23 · 日股数据日 10-23",
    "gaps": [
      {
        "kind": "gap",
        "reason": "no_news_source",
        "text": "近 5 日消息：暂无数据源"
      }
    ],
    "notes": [
      "MSFT 财报日只显示日期：盘前、盘后时间暂无数据源。"
    ],
    "rows": [
      {
        "currency": "USD",
        "day_change": {
          "kind": "value",
          "text": "+3.4%",
          "tone": "up",
          "unit": "percent",
          "value": 3.4
        },
        "day_sharp": {
          "kind": "sharp",
          "text": "大幅"
        },
        "earnings": {
          "date": "2026-10-28",
          "kind": "value",
          "text": "10-28 盘后"
        },
        "five_day_change": {
          "kind": "value",
          "text": "+5.0%",
          "tone": "up",
          "unit": "percent",
          "value": 5
        },
        "five_day_sharp": {
          "kind": "none"
        },
        "price": {
          "kind": "value",
          "text": "$214.30",
          "unit": "price",
          "value": 214.3
        },
        "symbol": "AMD",
        "volatility": {
          "data_day": "2026-10-23",
          "hv30": {
            "kind": "gap",
            "reason": "no_data",
            "text": "HV30（自有，口径 hv30_v1）：暂无读数"
          },
          "iv30": {
            "kind": "gap",
            "reason": "no_data",
            "text": "自有 iv30：暂无读数"
          },
          "ivp": {
            "kind": "gap",
            "reason": "no_data",
            "text": "自有 ivp：暂无读数"
          },
          "label": "自有 own，口径 hv30_v1 / iv30_v1 / ivp_v1 · 数据日 10-23",
          "reconciliation": "缺",
          "source": "自有 own"
        }
      },
      {
        "currency": "USD",
        "day_change": {
          "kind": "value",
          "text": "−0.4% · 10-22",
          "tone": "down",
          "unit": "percent",
          "value": -0.4
        },
        "day_sharp": {
          "kind": "none"
        },
        "earnings": {
          "date": "2026-11-12",
          "kind": "value",
          "text": "11-12"
        },
        "five_day_change": {
          "kind": "value",
          "text": "−1.0%",
          "tone": "down",
          "unit": "percent",
          "value": -1
        },
        "five_day_sharp": {
          "kind": "none"
        },
        "price": {
          "kind": "gap",
          "reason": "missing_quote",
          "text": "无报价"
        },
        "symbol": "MSFT",
        "volatility": {
          "data_day": "2026-10-23",
          "hv30": {
            "kind": "gap",
            "reason": "no_data",
            "text": "HV30（自有，口径 hv30_v1）：暂无读数"
          },
          "iv30": {
            "kind": "gap",
            "reason": "no_data",
            "text": "自有 iv30：暂无读数"
          },
          "ivp": {
            "kind": "gap",
            "reason": "no_data",
            "text": "自有 ivp：暂无读数"
          },
          "label": "自有 own，口径 hv30_v1 / iv30_v1 / ivp_v1 · 数据日 10-23",
          "reconciliation": "缺",
          "source": "自有 own"
        }
      },
      {
        "currency": "USD",
        "day_change": {
          "kind": "value",
          "text": "+0.8%",
          "tone": "up",
          "unit": "percent",
          "value": 0.8
        },
        "day_sharp": {
          "kind": "none"
        },
        "earnings": {
          "date": "2026-11-19",
          "kind": "value",
          "text": "11-19 盘前"
        },
        "five_day_change": {
          "kind": "value",
          "text": "−8.4%",
          "tone": "down",
          "unit": "percent",
          "value": -8.4
        },
        "five_day_sharp": {
          "kind": "sharp",
          "text": "大幅"
        },
        "price": {
          "kind": "value",
          "text": "$131.55",
          "unit": "price",
          "value": 131.55
        },
        "symbol": "NVDA",
        "volatility": {
          "data_day": "2026-10-23",
          "hv30": {
            "kind": "gap",
            "reason": "no_data",
            "text": "HV30（自有，口径 hv30_v1）：暂无读数"
          },
          "iv30": {
            "kind": "gap",
            "reason": "no_data",
            "text": "自有 iv30：暂无读数"
          },
          "ivp": {
            "kind": "gap",
            "reason": "no_data",
            "text": "自有 ivp：暂无读数"
          },
          "label": "自有 own，口径 hv30_v1 / iv30_v1 / ivp_v1 · 数据日 10-23",
          "reconciliation": "缺",
          "source": "自有 own"
        }
      }
    ],
    "say": "持有 9 家 · 3 家 28 日内财报 · 近 5 日消息：暂无数据源。",
    "sharp_note": "日涨跌 3% 及以上、5日涨跌 8% 及以上标“大幅”。",
    "stale": {
      "data_time": "2026-10-23T21:00:00+08:00",
      "is_stale": false,
      "text": "数据日 10-23"
    },
    "subtitle": "财报日历与变化",
    "title": "持仓公司"
  },
  "data_times": [
    {
      "data_time": "2026-10-23T21:00:00+08:00",
      "source": "held_companies"
    }
  ],
  "generated_at": "2026-10-23T21:00:00+08:00",
  "run_id": "run-demo-0001",
  "run_status": "ok",
  "schema": "held_companies_summary",
  "schema_version": 3,
  "scope": "home",
  "stale_after": "2026-10-24T21:00:00+08:00"
} satisfies HeldCompaniesSummary
