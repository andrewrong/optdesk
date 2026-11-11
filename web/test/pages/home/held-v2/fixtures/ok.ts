// Synthetic payload written from contracts/schemas/page.held_companies_summary.v3.json.
import type { HeldCompaniesSummary } from '../../../../../src/generated/held_companies_summary.v3'

export const ok = {
  "build_id": "build-demo-0001",
  "card": {
    "columns": [
      {
        "id": "company",
        "label": "Company"
      },
      {
        "id": "price",
        "label": "Price"
      },
      {
        "id": "earnings",
        "label": "Earnings date"
      },
      {
        "id": "day_change",
        "label": "Day change"
      },
      {
        "id": "five_day_change",
        "label": "5-day change"
      }
    ],
    "data_times_text": "US data day 10-23 · JP data day 10-23",
    "gaps": [
      {
        "kind": "gap",
        "reason": "no_news_source",
        "text": "News, last 5 days: no data source yet"
      }
    ],
    "notes": [
      "MSFT earnings date shows the date only: pre-market and after-hours times have no data source yet."
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
          "text": "big move"
        },
        "earnings": {
          "date": "2026-10-28",
          "kind": "value",
          "text": "10-28 after hours"
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
            "text": "HV30 (own, basis hv30_v1): no reading yet"
          },
          "iv30": {
            "kind": "gap",
            "reason": "no_data",
            "text": "Own iv30: no reading yet"
          },
          "ivp": {
            "kind": "gap",
            "reason": "no_data",
            "text": "Own ivp: no reading yet"
          },
          "label": "Own, basis hv30_v1 / iv30_v1 / ivp_v1 · data day 10-23",
          "reconciliation": "n/a",
          "source": "Own"
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
          "text": "No quote"
        },
        "symbol": "MSFT",
        "volatility": {
          "data_day": "2026-10-23",
          "hv30": {
            "kind": "gap",
            "reason": "no_data",
            "text": "HV30 (own, basis hv30_v1): no reading yet"
          },
          "iv30": {
            "kind": "gap",
            "reason": "no_data",
            "text": "Own iv30: no reading yet"
          },
          "ivp": {
            "kind": "gap",
            "reason": "no_data",
            "text": "Own ivp: no reading yet"
          },
          "label": "Own, basis hv30_v1 / iv30_v1 / ivp_v1 · data day 10-23",
          "reconciliation": "n/a",
          "source": "Own"
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
          "text": "11-19 pre-market"
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
          "text": "big move"
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
            "text": "HV30 (own, basis hv30_v1): no reading yet"
          },
          "iv30": {
            "kind": "gap",
            "reason": "no_data",
            "text": "Own iv30: no reading yet"
          },
          "ivp": {
            "kind": "gap",
            "reason": "no_data",
            "text": "Own ivp: no reading yet"
          },
          "label": "Own, basis hv30_v1 / iv30_v1 / ivp_v1 · data day 10-23",
          "reconciliation": "n/a",
          "source": "Own"
        }
      }
    ],
    "say": "Holding 9 · 3 with earnings within 28 days · News, last 5 days: no data source yet.",
    "sharp_note": "Day change of 3% or more and 5-day change of 8% or more are marked big move.",
    "stale": {
      "data_time": "2026-10-23T21:00:00+08:00",
      "is_stale": false,
      "text": "Data day 10-23"
    },
    "subtitle": "Earnings calendar and changes",
    "title": "Held companies"
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
