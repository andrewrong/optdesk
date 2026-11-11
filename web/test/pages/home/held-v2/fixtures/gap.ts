// Synthetic payload written from contracts/schemas/page.held_companies_summary.v3.json.
import type { HeldCompaniesSummary } from '../../../../../src/generated/held_companies_summary.v3'

export const gap = {
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
    "data_times_text": "Data day 10-23",
    "gaps": [
      {
        "kind": "gap",
        "reason": "missing_input",
        "text": "Held companies: not enough data, reason unknown"
      }
    ],
    "notes": [
      "Earnings date shows the date only: pre-market and after-hours times have no data source yet."
    ],
    "rows": [],
    "say": "Held companies: not enough data, reason unknown",
    "sharp_note": "Day change of 3% or more and 5-day change of 8% or more are marked big move.",
    "stale": {
      "data_time": "2026-10-23T21:00:00+08:00",
      "is_stale": false,
      "text": "Data day 10-23"
    },
    "subtitle": "Earnings calendar and changes",
    "title": "Held companies"
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
