// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Text = string;
export type Count = number;
export type Figure = string | null;

/**
 * View model of the strategy replay page: the form with its defaults (view form) and the form with the report under it (view report). The route-off page is the view form with form null and the notice key form_off. Every figure is a decimal text or null written by the builder; the view computes nothing.
 */
export interface StrategyReplayPage {
  schema: "strategy_replay_page";
  schema_version: 1;
  generated_at: string;
  title: Text;
  view: "form" | "report";
  post_path: "strategy-replay-post";
  intro: Text;
  form: Form | null;
  restart_hint: Text;
  notices: Notice[];
  report: Report | null;
}
export interface Form {
  fields: Field[];
  submit_label: Text;
}
export interface Field {
  name: "lots" | "alias" | "as_of" | "early_keep" | "early_dte" | "min_lots";
  label: Text;
  hint: Text;
  kind: "choice" | "text";
  value: string;
  options: Option[];
}
export interface Option {
  value: string;
  label: Text;
}
export interface Notice {
  key:
    | "form_off"
    | "timed_out"
    | "busy"
    | "read_failed"
    | "bad_lots"
    | "bad_as_of"
    | "bad_early_keep"
    | "bad_early_dte"
    | "bad_min_lots";
  text: Text;
}
export interface Report {
  as_of: string;
  lots_label: Text;
  first_quote_day: string | null;
  last_quote_day: string | null;
  variants: Variant[];
  counts: Counts;
  excluded: Excluded[];
  lines: Line[];
  notes: Text[];
  empty_text: Text | null;
  hints?: {
    counts: {
      lots: Text;
      lots_with_quote: Text;
      still_open: Text;
      lot_days: Text;
      quote_days: Text;
    };
    columns: {
      set: Text;
      variant: Text;
      currency: Text;
      lots: Text;
      cycles: Text;
      closed_early: Text;
      held: Text;
      assigned: Text;
      rolled: Text;
      in_the_money: Text;
      kept_share: Text;
      mean_gross: Text;
      worst_gross: Text;
      hold_kept_share: Text;
      actual_kept_share: Text;
    };
  };
}
export interface Variant {
  name: string;
  params_hash: string;
  hint?: Text;
}
export interface Counts {
  lots: Count;
  lots_with_quote: Count;
  still_open: Count;
  lot_days: Count;
  quote_days: Count;
}
export interface Excluded {
  reason: string;
  text: Text;
  lots: Count;
}
export interface Line {
  set: string;
  variant: string;
  currency: string;
  lots: Count;
  cycles: Count;
  closed_early: Count;
  held: Count;
  assigned: Count;
  rolled: Count;
  in_the_money: Count;
  kept_share: Figure;
  mean_gross: Figure;
  worst_gross: Figure;
  hold_kept_share: Figure;
  actual_kept_share: Figure;
  under_minimum: boolean;
  minimum_text: Text | null;
}
