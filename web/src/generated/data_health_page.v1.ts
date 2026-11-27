// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Id = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
export type Text = string;
export type Count = number;
export type Day = string;
export type Optional = string;
export type RepeatedFailure = {
  key: string;
  kind: "failed" | "empty";
  days: number;
  runs: number;
  first_day: string;
  last_day: string;
  last_class: string;
  class_text: Text;
};

/**
 * View model of the data health page: the coverage of each dataset over the window, cell by cell, and the repair each cell offers. The builder computes every state, count, age and text; the view computes nothing.
 */
export interface DataHealthPage {
  schema: "data_health_page";
  schema_version: 1;
  run_id: Id;
  built_at: Timestamp;
  form: Form | null;
  datasets: Dataset[];
  glossary: GlossaryRow[];
  overwrite_note: string;
  gaps: Gap[];
  failures_note?: Text;
  failures_none_text?: Text;
  running?: Running[];
  gateways?: Gateway[];
  data_check?: DataCheck | null;
}
export interface Form {
  post_path: "/owner-job-console";
  expires: Text;
  mac: Text;
}
export interface Dataset {
  dataset_id: Text;
  label: Text;
  job_id: string | null;
  calendar: Text;
  gap_days: Count;
  repair_all: "button" | "none";
  newest: Newest[];
  cells: Cell[];
  failures_total?: Count;
  failures_hidden?: Count;
  failures_hidden_text?: Optional;
  repeated_failures?: RepeatedFailure[];
}
export interface Newest {
  series_id: Text;
  day: Day;
  age_days: Count;
}
export interface Cell {
  day: Day;
  state: "ok" | "no_value" | "expired" | "missing" | "not_run";
  keys: Count;
  ok: Count;
  no_value: Count;
  expired: Count;
  missing: Count;
  /**
   * @maxItems 20
   */
  missing_keys:
    | []
    | [Text]
    | [Text, Text]
    | [Text, Text, Text]
    | [Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text
      ];
  repair: "none" | "button" | "not_refetchable";
  why_text: Optional;
  due_text: Optional;
}
export interface GlossaryRow {
  term: Text;
  text: Text;
}
export interface Gap {
  kind: "gap";
  reason: "coverage_unavailable" | "failures_unavailable" | "data_check_unavailable";
  text: string;
}
export interface Running {
  line: Text;
  job?: string;
  started_at?: string;
  keys_done?: number;
  keys_total?: number;
  step?: string;
  elapsed_minutes?: number;
  usual_minutes?: number;
  slow?: boolean;
}
export interface Gateway {
  name: Text;
  verdict: "session_up" | "session_retrying" | "farm_down" | "unreachable";
  farms: GatewayFarm[];
  note: Optional;
  since?: string | "";
  since_text?: Optional;
}
export interface GatewayFarm {
  name: Text;
  up: boolean;
}
export interface DataCheck {
  check_day: Day | "";
  finished_at: string | "";
  none_text: Optional;
  rules_total: Count;
  rules_unknown: Count;
  title: Text;
  note: Text;
  counts: DataCheckCounts;
  findings: DataCheckFinding[];
}
export interface DataCheckCounts {
  error: Count;
  warn: Count;
  unknown: Count;
}
export interface DataCheckFinding {
  rule: Text;
  label: Text;
  dataset: Text;
  day: Day;
  severity: "error" | "warn" | "unknown";
  severity_text: Text;
  is_new: boolean;
  evidence_text: Text;
  why_text: Text;
}
