// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Id = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
export type Text = string;
export type Optional = string;
export type Day = string;

/**
 * View model of the company data page: one row per company with what is stored for it and what is missing and why. The builder computes every figure and text; the view computes nothing.
 */
export interface CompanyDataPage {
  schema: "company_data_page";
  schema_version: 1;
  run_id: Id;
  built_at: Timestamp;
  data_health_path: Text;
  columns: Column[];
  companies: Company[];
  glossary: GlossaryRow[];
  gaps: Gap[];
}
export interface Column {
  key: Text;
  label: Text;
  gloss: Optional;
}
export interface Company {
  key: Text;
  symbol: Text;
  name: Optional;
  market: Text;
  issuer: Optional;
  fiscal_year_end: Optional;
  price_to: Day | null;
  bar_count: string;
  statements: boolean;
  latest_quarter: Optional;
  eps_quarters: string;
  next_report: Day | null;
  missing: Missing[];
  quarters_path: Optional;
}
export interface Missing {
  code:
    | "jp"
    | "no_cik"
    | "fundamentals_off"
    | "no_statements"
    | "annual_only"
    | "no_instrument"
    | "no_bars"
    | "no_next_report";
  text: Text;
}
export interface GlossaryRow {
  term: Text;
  text: Text;
}
export interface Gap {
  kind: "gap";
  reason: string;
  text: string;
}
