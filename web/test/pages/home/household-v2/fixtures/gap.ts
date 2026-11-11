import type { HouseholdSummary } from "../../../../../src/generated/household_summary.v2"

// Written from contracts/schemas/page.household_summary.v2.json alone; synthetic.
export const gap = {
  "build_id": "build-demo-0001",
  "card": {
    "data_times_text": "Net value 10-21 · Cash 10-21 · Previous trading day 10-21",
    "gaps": [
      {
        "reason": "missing_input",
        "text": "No daily statement this month yet"
      },
      {
        "reason": "missing_input",
        "text": "No country breakdown, cash share not shown yet"
      }
    ],
    "notes": [
      "The previous-day change may include position transfers in or out."
    ],
    "stale": {
      "data_time": "2026-10-22T05:05:00+08:00",
      "stale_after": "2026-10-23T05:35:00+08:00",
      "text": "Data day 10-21"
    },
    "subtitle": "Two accounts total · data day 10-21",
    "tiles": {
      "cash": {
        "caption": {
          "text": "+$156,700",
          "tone": "up",
          "unit": "usd_signed",
          "value": 156700
        },
        "explain": "Sum of the cash (broker TotalCashValue, USD) of the member accounts. The large figure is cash as a share of net value, the small figure is the amount; the share needs cash, net value and country breakdown from the same day. Negative cash is a margin loan: the label becomes Margin loan and the share shows On margin.",
        "figure": {
          "reason": "missing_input",
          "text": "No country breakdown, cash share not shown yet"
        },
        "label": "Cash"
      },
      "day_change": {
        "caption": {
          "text": "+0.45%",
          "tone": "up",
          "unit": "percent",
          "value": 0.45
        },
        "explain": "The Flex daily total of the latest report day, minus the total of the previous report day, summed over members, USD. Includes deposits, withdrawals and position transfers of the day, not pure return. The percentage is this change divided by the total of the previous US trading day, not shown when that total is missing.",
        "figure": {
          "text": "+$1,860",
          "tone": "up",
          "unit": "usd_signed",
          "value": 1860
        },
        "label": "Previous trading day"
      },
      "month_to_date": {
        "caption": {
          "reason": "missing_input",
          "text": "No daily statement this month yet"
        },
        "explain": "Month change (month-end total minus month-start total minus deposits, USD) divided by the month-start total, simple method, not a time-weighted return. The month starts on 2026-10-01. The timing of deposits within the month is ignored.",
        "figure": {
          "reason": "missing_input",
          "text": "No daily statement this month yet"
        },
        "label": "Month to date (excl. deposits and withdrawals)"
      },
      "net_worth": {
        "caption": {
          "text": "USD"
        },
        "explain": "Sum of the net liquidation values (broker NetLiquidation, USD) of the household members' accounts, from the latest after-close snapshot; not shown when a member has no snapshot or the snapshot times are too far apart. Non-USD stocks such as JPY are converted at the broker ledger rate, hence the ≈.",
        "figure": {
          "text": "≈$412,300",
          "unit": "USD",
          "value": 412300
        },
        "label": "Net value"
      }
    },
    "title": "Household overview"
  },
  "currency": "USD",
  "data_times": [
    {
      "data_time": "2026-10-22T05:05:00+08:00",
      "source": "net_liquidation"
    },
    {
      "data_time": "2026-10-22T05:00:00+08:00",
      "source": "flex_net_value"
    }
  ],
  "generated_at": "2026-10-22T09:30:00+08:00",
  "run_id": "run-demo-0001",
  "run_status": "partial",
  "schema": "household_summary",
  "schema_version": 2,
  "scope": "household"
} satisfies HouseholdSummary
