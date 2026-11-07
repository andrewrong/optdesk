// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type BuildId = string;
/**
 * @maxItems 20
 */
export type DataTimes =
  | []
  | [SourceTime]
  | [SourceTime, SourceTime]
  | [SourceTime, SourceTime, SourceTime]
  | [SourceTime, SourceTime, SourceTime, SourceTime]
  | [SourceTime, SourceTime, SourceTime, SourceTime, SourceTime]
  | [SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime]
  | [SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime]
  | [SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime]
  | [SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime]
  | [
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime
    ]
  | [
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime
    ]
  | [
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime
    ]
  | [
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime
    ]
  | [
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime
    ]
  | [
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime
    ]
  | [
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime
    ]
  | [
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime
    ]
  | [
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime
    ]
  | [
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime
    ]
  | [
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime,
      SourceTime
    ];
export type DataTime = string;
export type Source = string;
/**
 * @maxItems 10
 */
export type Facts =
  | []
  | [Pair]
  | [Pair, Pair]
  | [Pair, Pair, Pair]
  | [Pair, Pair, Pair, Pair]
  | [Pair, Pair, Pair, Pair, Pair]
  | [Pair, Pair, Pair, Pair, Pair, Pair]
  | [Pair, Pair, Pair, Pair, Pair, Pair, Pair]
  | [Pair, Pair, Pair, Pair, Pair, Pair, Pair, Pair]
  | [Pair, Pair, Pair, Pair, Pair, Pair, Pair, Pair, Pair]
  | [Pair, Pair, Pair, Pair, Pair, Pair, Pair, Pair, Pair, Pair];
export type Label = string;
export type Value = string;
export type GeneratedAt = string;
/**
 * @maxItems 10
 */
export type Notes =
  | []
  | [string]
  | [string, string]
  | [string, string, string]
  | [string, string, string, string]
  | [string, string, string, string, string]
  | [string, string, string, string, string, string]
  | [string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string];
export type Nothing = string | null;
export type Heading = string;
export type Key = string;
/**
 * @minItems 3
 * @maxItems 3
 */
export type Lines = [string, string, string];
/**
 * @maxItems 50
 */
export type Reliability = Reliability1[];
export type RunId = string;
export type RunStatus = "ok" | "partial" | "failed";
export type Schema = "option_result_page";
export type SchemaVersion = 1;
export type Scope = string;
export type Assignment = string | null;
/**
 * @minItems 2
 * @maxItems 2
 */
export type BooksLines = [string, string];
export type BooksNote = string;
export type BooksTitle = string;
export type Empty = string | null;
export type Label1 = string;
export type Muted = boolean;
export type Text = string;
export type Value1 = number;
/**
 * @maxItems 600
 */
export type Points = SeriesPoint[];
export type Summary = string | null;
export type Title = string;
/**
 * @maxItems 10
 */
export type EndingsLines =
  | []
  | [string]
  | [string, string]
  | [string, string, string]
  | [string, string, string, string]
  | [string, string, string, string, string]
  | [string, string, string, string, string, string]
  | [string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string];
export type EndingsTitle = string;
/**
 * @maxItems 4
 */
export type FigureNotes =
  [] | [string] | [string, string] | [string, string, string] | [string, string, string, string];
/**
 * @minItems 6
 * @maxItems 6
 */
export type Figures = [Pair, Pair, Pair, Pair, Pair, Pair];
export type Heading1 = string;
export type Key1 = string;
export type Lead = string;
export type Protection = string | null;
export type Scope1 = string;
/**
 * @maxItems 100
 */
export type Sections = ResultSection[];
export type DataTime1 = string;
export type StaleAfter = string;
export type Text1 = string;
export type Title1 = string;

export interface OptionResultPage {
  build_id: BuildId;
  data_times: DataTimes;
  facts: Facts;
  generated_at: GeneratedAt;
  notes: Notes;
  nothing: Nothing;
  reliability: Reliability;
  run_id: RunId;
  run_status: RunStatus;
  schema: Schema;
  schema_version: SchemaVersion;
  scope: Scope;
  sections: Sections;
  stale: StaleLabel | null;
  title: Title1;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: DataTime;
  source: Source;
}
/**
 * A term and its value, both text as Python wrote them.
 */
export interface Pair {
  label: Label;
  value: Value;
}
export interface Reliability1 {
  heading: Heading;
  key: Key;
  lines: Lines;
}
export interface ResultSection {
  assignment: Assignment;
  books_lines: BooksLines;
  books_note: BooksNote;
  books_title: BooksTitle;
  endings: Series;
  endings_lines: EndingsLines;
  endings_title: EndingsTitle;
  figure_notes: FigureNotes;
  figures: Figures;
  heading: Heading1;
  key: Key1;
  lead: Lead;
  months: Series;
  protection: Protection;
  running: Series;
  scope: Scope1;
}
/**
 * A chart's title and points. ``summary`` is the points' ``{label} {text}`` joined by ``；``, the
 * text that stands for the chart (its ARIA text); ``empty`` is the sentence that stands in when
 * there is no point (then ``summary`` is absent).
 */
export interface Series {
  empty: Empty;
  points: Points;
  summary: Summary;
  title: Title;
}
/**
 * One month or one ending: the axis label, the number the chart draws, the text shown and
 * whether the point is muted (a month that is not over).
 */
export interface SeriesPoint {
  label: Label1;
  muted: Muted;
  text: Text;
  value: Value1;
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
  data_time: DataTime1;
  stale_after: StaleAfter;
  text: Text1;
}
