// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Text = string;
export type Kind = "gap";
export type Reason =
  | "missing_quote"
  | "no_data"
  | "unsupported_currency"
  | "not_built"
  | "no_reader"
  | "no_news_source"
  | "no_rule"
  | "market_closed"
  | "stale"
  | "flex_time_unknown"
  | "jp_slice_missing"
  | "missing_input"
  | "stale_input";
export type Text1 = string;
export type Optional = string;

/**
 * View model of the job console page: every job, the form that runs one, the newest requests with one line per key. Every visible string is a field; the view computes nothing.
 */
export interface JobConsole {
  schema: "job_console";
  schema_version: 1;
  generated_at: string;
  title: Text;
  sentence: string;
  form: Form | null;
  gaps: Gap[];
  jobs: Job[];
  /**
   * @maxItems 20
   */
  requests:
    | []
    | [Request]
    | [Request, Request]
    | [Request, Request, Request]
    | [Request, Request, Request, Request]
    | [Request, Request, Request, Request, Request]
    | [Request, Request, Request, Request, Request, Request]
    | [Request, Request, Request, Request, Request, Request, Request]
    | [Request, Request, Request, Request, Request, Request, Request, Request]
    | [Request, Request, Request, Request, Request, Request, Request, Request, Request]
    | [Request, Request, Request, Request, Request, Request, Request, Request, Request, Request]
    | [Request, Request, Request, Request, Request, Request, Request, Request, Request, Request, Request]
    | [Request, Request, Request, Request, Request, Request, Request, Request, Request, Request, Request, Request]
    | [
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request
      ]
    | [
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request
      ]
    | [
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request
      ]
    | [
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request
      ]
    | [
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request
      ]
    | [
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request
      ]
    | [
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request
      ]
    | [
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request,
        Request
      ];
  selected: number | null;
  refresh_seconds: 0 | 5;
}
export interface Form {
  post_path: "/owner-job-console";
  expires: Text;
  mac: Text;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  kind: Kind;
  reason: Reason;
  text: Text1;
}
export interface Job {
  id: string;
  kind: "any_day" | "today_only" | "listed_only";
  today: string;
  note: string | null;
  force: boolean;
  force_why: Optional;
  why_text: Optional;
  needs_text: Optional;
}
export interface Request {
  id: number;
  job: string;
  day: string;
  force: boolean;
  state: "queued" | "running" | "done" | "refused" | "failed" | "interrupted" | "withdrawn";
  state_text: Text;
  at_text: string;
  reason: Optional;
  keys: Key[];
}
export interface Key {
  dataset: Text;
  subject: Text;
  outcome:
    | "already_present"
    | "waiting"
    | "skipped"
    | "not_run"
    | "failed"
    | "partial"
    | "produced"
    | "unconfirmed"
    | "nothing_found";
  outcome_text: Text;
  present: Optional;
  reason: Optional;
}
