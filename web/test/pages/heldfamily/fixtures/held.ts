// A typed copy of contracts/fixtures/page.held_family_page.v1.ok.json: a value, a negative value, a gap and each earnings reason.
import type { HeldFamilyPage } from '@/generated/held_family_page.v1'

export const held: HeldFamilyPage = {
  "schema": "held_family_page",
  "schema_version": 1,
  "run_id": "run-demo-0001",
  "build_id": "build-demo-0001",
  "scope": "held-family",
  "generated_at": "2026-11-21T09:00:00+08:00",
  "data_times": [
    {
      "data_time": "2026-11-20T00:00:00-05:00",
      "source": "household_capture"
    }
  ],
  "run_status": "ok",
  "stale": null,
  "title": "Held companies",
  "notes": [
    "Synthetic fixture: invented names and numbers, not for trading decisions."
  ],
  "gaps": [],
  "total": {
    "text": "340,000",
    "currency": "USD",
    "day": "2026-11-20",
    "accounts_text": "main + wife + daughter"
  },
  "columns": [
    {
      "id": "company",
      "label": "Company",
      "hint": "Synthetic hint: the company"
    },
    {
      "id": "accounts",
      "label": "Held in",
      "hint": "Synthetic hint: the accounts that hold it"
    },
    {
      "id": "value",
      "label": "Market value (USD)",
      "hint": "Synthetic hint: quantity times the latest close"
    },
    {
      "id": "share",
      "label": "Share of total",
      "hint": "Synthetic hint: value over the family total"
    },
    {
      "id": "earnings",
      "label": "Next earnings",
      "hint": "Synthetic hint: date and days to it"
    }
  ],
  "rows": [
    {
      "company": "AAA",
      "accounts": [
        {
          "alias": "main",
          "label": "main"
        },
        {
          "alias": "wife",
          "label": "wife"
        }
      ],
      "value": {
        "kind": "value",
        "text": "41,230"
      },
      "share": {
        "kind": "value",
        "text": "12.1 %"
      },
      "earnings": {
        "kind": "date",
        "date": "2026-11-25",
        "days_to": 4,
        "text": "2026-11-25 (in 4 days)"
      }
    },
    {
      "company": "EEE",
      "accounts": [
        {
          "alias": "main",
          "label": "main"
        }
      ],
      "value": {
        "kind": "value",
        "text": "-2,000"
      },
      "share": {
        "kind": "value",
        "text": "-0.6 %"
      },
      "earnings": {
        "kind": "gap",
        "reason": "no_row",
        "text": "no date: the earnings job has no row for this company"
      }
    },
    {
      "company": "DDD",
      "accounts": [
        {
          "alias": "main",
          "label": "main"
        },
        {
          "alias": "daughter",
          "label": "daughter"
        }
      ],
      "value": {
        "kind": "gap",
        "text": "gap: no mark",
        "reason": "missing_quote"
      },
      "share": {
        "kind": "gap",
        "text": "gap: no mark",
        "reason": "missing_quote"
      },
      "earnings": {
        "kind": "date",
        "date": "2026-11-21",
        "days_to": 0,
        "text": "2026-11-21 (today)"
      }
    },
    {
      "company": "CCC",
      "accounts": [
        {
          "alias": "wife",
          "label": "wife"
        }
      ],
      "value": {
        "kind": "value",
        "text": "12,400"
      },
      "share": {
        "kind": "value",
        "text": "3.6 %"
      },
      "earnings": {
        "kind": "gap",
        "reason": "unsupported",
        "text": "no date: the earnings job does not support this company"
      }
    },
    {
      "company": "FFF",
      "accounts": [
        {
          "alias": "main",
          "label": "main"
        }
      ],
      "value": {
        "kind": "value",
        "text": "9,000"
      },
      "share": {
        "kind": "value",
        "text": "2.6 %"
      },
      "earnings": {
        "kind": "gap",
        "reason": "row_error",
        "text": "no date: the earnings row is an error"
      }
    },
    {
      "company": "GGG",
      "accounts": [
        {
          "alias": "wife",
          "label": "wife"
        }
      ],
      "value": {
        "kind": "value",
        "text": "8,000"
      },
      "share": {
        "kind": "value",
        "text": "2.4 %"
      },
      "earnings": {
        "kind": "gap",
        "reason": "no_reader",
        "text": "no date: no earnings reader"
      }
    },
    {
      "company": "HHH",
      "accounts": [
        {
          "alias": "main",
          "label": "main"
        }
      ],
      "value": {
        "kind": "value",
        "text": "7,000"
      },
      "share": {
        "kind": "value",
        "text": "2.1 %"
      },
      "earnings": {
        "kind": "gap",
        "reason": "no_underlying",
        "text": "no date: no underlying row for this symbol"
      }
    },
    {
      "company": "III",
      "accounts": [
        {
          "alias": "main",
          "label": "main"
        }
      ],
      "value": {
        "kind": "value",
        "text": "6,000"
      },
      "share": {
        "kind": "value",
        "text": "1.8 %"
      },
      "earnings": {
        "kind": "gap",
        "reason": "no_issuer",
        "text": "no date: underlying row has no issuer"
      }
    },
    {
      "company": "JJJ",
      "accounts": [
        {
          "alias": "wife",
          "label": "wife"
        }
      ],
      "value": {
        "kind": "value",
        "text": "5,000"
      },
      "share": {
        "kind": "value",
        "text": "1.5 %"
      },
      "earnings": {
        "kind": "gap",
        "reason": "past_date",
        "text": "no date: the stored date is already past"
      }
    }
  ],
  "rows_sum": {
    "kind": "value",
    "text": "82,530 USD = 24.3 % of 340,000"
  },
  "lead_notes": [
    "Synthetic note: 2 rows have a close older than the data day."
  ]
}
