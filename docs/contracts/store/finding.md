# store/finding

Package `internal/store/finding` declares the findings of the daily data check (`docs/design/data-check.md`, Interface, entry 1). It holds declarations only; `internal/store/findingpg` implements `Writer`, `Reader` and `Probe`. The writer and the probe use the line `OT_PG_DSN_DATACHECK` (role `ot_w_datacheck`), the reader the pages line.

## Severities

`error`: wrong data stored as good. `warn`: a gap, data missing or late. `unknown`: the rule could not decide, so the run says nothing about the data it covers.

## Rule ids

`reading_repeat`, `option_open_late_quote`, `candidates_older_than_settings`, `capture_gap`, `plan_absent`, `gap_unclosed`. The ids are fixed strings; a new rule adds one and never changes one.

## Evidence vocabulary

`ok`, `same`, `rows`, `late`, `hours_behind`, `keys`, `missing`, `expired`, `cells`, `age_hours`, `read_failed`. Each is a `Count{Name, N}`; a new name is added, never changed.

## What a finding holds

A finding holds counts and fixed words, never free text, an id or a secret. It is keyed by (rule, dataset, day, subject), not by its counts; `IsNew` is set by the writer and ignored on input.
