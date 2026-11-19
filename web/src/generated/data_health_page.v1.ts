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
export interface RepeatedFailure {
  key: string;
  kind: "failed" | "empty";
  days: number;
  runs: number;
  first_day: Day;
  last_day: Day;
  last_class: string;
  class_text: Text;
}
export interface GlossaryRow {
  term: Text;
  text: Text;
}
export interface Gap {
  kind: "gap";
  reason: "coverage_unavailable" | "failures_unavailable";
  text: string;
}
