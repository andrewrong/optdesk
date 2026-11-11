import type { MacroSummary } from "../../../../../src/generated/macro_summary.v2"

export const ok = {
  "build_id": "build-demo-0001",
  "card": {
    "data_times_text": "Ease data day 10-23; event dates follow the release calendar, no release time.",
    "ease": {
      "aria_text": "Ease 56, neutral, −0.2 vs last week; fixed scale 0 to 100, neutral band 34 to 66.",
      "figure": {
        "text": "56",
        "unit": "index_points",
        "value": 56.0
      },
      "marker_percent": {
        "text": "56",
        "unit": "percent_of_track",
        "value": 56.0
      },
      "neutral_band": {
        "high": {
          "text": "66%",
          "unit": "percent_of_track",
          "value": 66.0
        },
        "low": {
          "text": "34%",
          "unit": "percent_of_track",
          "value": 34.0
        }
      },
      "reading_text": "Now 56, neutral; fixed scale 0 to 100.",
      "ticks": [
        "0",
        "25",
        "50",
        "75",
        "100"
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
        "name_text": "US CPI",
        "previous": "3.0% (Sep)",
        "when_text": "Wed 10-28"
      },
      {
        "date": "2026-11-06",
        "name_text": "US nonfarm payrolls",
        "previous": "+120k (Sep)",
        "when_text": "Fri 11-06"
      },
      {
        "date": "2026-11-17",
        "name_text": "US retail sales",
        "previous": {
          "reason": "no_data",
          "text": "Previous: no data yet"
        },
        "when_text": "Tue 11-17"
      }
    ],
    "gaps": [
      {
        "reason": "no_data",
        "text": "Previous: no data yet"
      }
    ],
    "notes": [
      "Dates come from the FRED release calendar and central bank schedules; dates only, no release time.",
      "Previous is the latest stored period of the series, labelled with its month; with no stored value or no matching series it shows n/a."
    ],
    "say": "Ease is neutral, −0.2 vs last week. The event this week is CPI.",
    "stale": {
      "data_time": "2026-10-23T00:00:00+08:00",
      "stale_after": "2026-10-27T09:30:00+08:00",
      "text": "Data day 10-23"
    },
    "title": "Macro"
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
