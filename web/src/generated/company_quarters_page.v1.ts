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
 * View model of the quarterly table of one company: one row per fiscal quarter, one cell per figure, each cell with its flags and the reason when it has no value. The builder computes every figure and text; the view computes nothing.
 */
export interface CompanyQuartersPage {
  schema: "company_quarters_page";
  schema_version: 1;
  run_id: Id;
  built_at: Timestamp;
  company: Company | null;
  choices: Choice[];
  columns: Column[];
  quarters: Quarter[];
  notes: Text[];
  gaps: Gap[];
}
export interface Company {
  key: Text;
  symbol: Text;
  name: Optional;
  issuer: Optional;
  fiscal_year_end: Optional;
}
export interface Choice {
  key: Text;
  label: Text;
  path: Text;
}
export interface Column {
  key: Text;
  label: Text;
  gloss: Optional;
}
export interface Quarter {
  label: Optional;
  period_end: Day;
  filing_date: Day | null;
  cells: {
    [k: string]: Cell;
  };
}
export interface Cell {
  text: Optional;
  missing: boolean;
  derived: boolean;
  restated: boolean;
  concept: Optional;
  why_text: Optional;
}
export interface Gap {
  kind: "gap";
  reason: string;
  text: string;
}
