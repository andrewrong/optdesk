// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Id = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
export type Day = string;
export type Level = "pass" | "warn" | "fail" | "unknown";
export type Text = string;
export type Count = number;
export type Optional = string;

/**
 * View model of the base data page: one verdict per base dataset, its newest value with date and age, and the checks that failed. Version 2 may add a cross-check against the broker or a second source (optional cross_check on a row and cross_checks); version 1 never writes them. The builder computes every level, count, age and text; the view computes nothing.
 */
export interface BaseDataPage {
  schema: "base_data_page";
  schema_version: 1;
  run_id: Id;
  built_at: Timestamp;
  as_of: Day;
  last_sessions: LastSessions;
  top: Top;
  groups: Group[];
  glossary: GlossaryRow[];
  derived_note: string;
  repair_path: "/data-health";
  gaps: Gap[];
  cross_checks?: CrossCheck[];
}
export interface LastSessions {
  XNYS: Day;
  XTKS: Day;
}
export interface Top {
  level: Level;
  text_zh: Text;
  counts: Counts;
}
export interface Counts {
  pass: Count;
  warn: Count;
  fail: Count;
  unknown: Count;
}
export interface Group {
  name: Text;
  rows: Row[];
}
export interface Row {
  dataset: Text;
  label: Text;
  label_zh: Text;
  level: Level;
  newest_value: Optional;
  newest_day: Day | null;
  newest_subject: Optional;
  age_text: Optional;
  failed_summary: Optional;
  checks: Check[];
  cross_check?: CrossCheck;
}
export interface Check {
  rule: Text;
  label: Text;
  hint_zh: Text;
  level: Level;
  n: Count;
  bad: Count;
  share: Optional;
  warn_at: Optional;
  fail_at: Optional;
  threshold_source: Optional;
  example: Optional;
}
export interface CrossCheck {
  pair_id: Text;
  level: Level;
  n: Count;
  matched: Count;
  diff: Count;
  missing: Count;
  tolerance_text: Optional;
  source_id: Text;
  run_at: Timestamp | null;
  /**
   * @maxItems 20
   */
  items:
    | []
    | [CrossCheckItem]
    | [CrossCheckItem, CrossCheckItem]
    | [CrossCheckItem, CrossCheckItem, CrossCheckItem]
    | [CrossCheckItem, CrossCheckItem, CrossCheckItem, CrossCheckItem]
    | [CrossCheckItem, CrossCheckItem, CrossCheckItem, CrossCheckItem, CrossCheckItem]
    | [CrossCheckItem, CrossCheckItem, CrossCheckItem, CrossCheckItem, CrossCheckItem, CrossCheckItem]
    | [CrossCheckItem, CrossCheckItem, CrossCheckItem, CrossCheckItem, CrossCheckItem, CrossCheckItem, CrossCheckItem]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ]
    | [
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem,
        CrossCheckItem
      ];
}
export interface CrossCheckItem {
  key: Text;
  ours: Optional;
  other: Optional;
  diff: Optional;
  kind: Text;
  hint_zh: Optional;
}
export interface GlossaryRow {
  term: Text;
  text: Text;
}
export interface Gap {
  kind: "gap";
  reason: "facts_unavailable" | "view_unavailable" | "coverage_unavailable";
  text: string;
}
