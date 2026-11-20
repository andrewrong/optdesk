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
  schema_version: 2;
  run_id: Id;
  built_at: Timestamp;
  data_health_path: Text;
  columns: Column[];
  companies: Company[];
  glossary: GlossaryRow[];
  gaps: Gap[];
  /**
   * Where the daily bars behind this page came from: ibkr, yfinance (fallback while IB history fails) or mixed. Absent when the page shows no bar-based figure or all bars are ibkr.
   */
  bars_source?: "ibkr" | "yfinance" | "mixed";
  /**
   * Visible text saying that the bars are not from IB, when bars_source is yfinance or mixed.
   */
  bars_source_note?: string;
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
