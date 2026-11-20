// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Id = string;
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
/**
 * An RFC 3339 time with any offset.
 */
export type DataTime = string;
export type Id1 = string;
export type Ease = Ease1 | Gap;
export type AriaText = string;
export type Text = string;
export type Tone = ("up" | "down" | "flat" | "none") | null;
export type Unit = "index_points";
export type Value = number;
export type Text1 = string;
export type Unit1 = "percent_of_track";
export type Value1 = number;
export type ReadingText = string;
/**
 * @minItems 5
 * @maxItems 5
 */
export type Ticks = [string, string, string, string, string];
export type WeekChange = IndexPointsFigure | Gap;
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
export type Text2 = string;
export type EaseTitle = string;
/**
 * @maxItems 20
 */
export type Events =
  | []
  | [Event]
  | [Event, Event]
  | [Event, Event, Event]
  | [Event, Event, Event, Event]
  | [Event, Event, Event, Event, Event]
  | [Event, Event, Event, Event, Event, Event]
  | [Event, Event, Event, Event, Event, Event, Event]
  | [Event, Event, Event, Event, Event, Event, Event, Event]
  | [Event, Event, Event, Event, Event, Event, Event, Event, Event]
  | [Event, Event, Event, Event, Event, Event, Event, Event, Event, Event]
  | [Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event]
  | [Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event]
  | [Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event]
  | [Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event]
  | [Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event]
  | [Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event, Event]
  | [
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event
    ]
  | [
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event
    ]
  | [
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event
    ]
  | [
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event,
      Event
    ];
export type Date = string;
export type DaysTo = number;
export type InWindow = boolean;
export type NameText = string;
export type Previous = string | Gap;
export type WhenText = string;
export type EventsTitle = string;
/**
 * @maxItems 20
 */
export type Gaps =
  | []
  | [Gap]
  | [Gap, Gap]
  | [Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap];
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
/**
 * @maxItems 20
 */
export type Indicators =
  | []
  | [Indicator]
  | [Indicator, Indicator]
  | [Indicator, Indicator, Indicator]
  | [Indicator, Indicator, Indicator, Indicator]
  | [Indicator, Indicator, Indicator, Indicator, Indicator]
  | [Indicator, Indicator, Indicator, Indicator, Indicator, Indicator]
  | [Indicator, Indicator, Indicator, Indicator, Indicator, Indicator, Indicator]
  | [Indicator, Indicator, Indicator, Indicator, Indicator, Indicator, Indicator, Indicator]
  | [Indicator, Indicator, Indicator, Indicator, Indicator, Indicator, Indicator, Indicator, Indicator]
  | [Indicator, Indicator, Indicator, Indicator, Indicator, Indicator, Indicator, Indicator, Indicator, Indicator]
  | [
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator
    ]
  | [
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator
    ]
  | [
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator
    ]
  | [
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator
    ]
  | [
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator
    ]
  | [
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator
    ]
  | [
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator
    ]
  | [
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator
    ]
  | [
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator
    ]
  | [
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator,
      Indicator
    ];
export type ChangeText = string | null;
export type DataDay = string | null;
export type DetailHref = string | null;
export type Group = "rates" | "credit" | "dollar_japan";
export type Hint = string;
export type Label = string;
export type Latest = PercentFigure | LevelFigure | Gap;
export type Text3 = string;
export type Tone1 = ("up" | "down" | "flat" | "none") | null;
export type Unit2 = "percent";
export type Value2 = number;
export type Text4 = string;
export type Tone2 = ("up" | "down" | "flat" | "none") | null;
export type Unit3 = "index_points" | "jpy_per_usd" | "percent";
export type Value3 = number;
export type Name = string;
export type SourceText = string;
export type Status = "good" | "late" | "missing";
export type IndicatorsTitle = string;
export type Note = string;
/**
 * @minItems 4
 * @maxItems 4
 */
export type Rows = [LeadRow, LeadRow, LeadRow, LeadRow];
export type ChangeText1 = string | null;
export type DataDay1 = string | null;
export type Hint1 = string;
export type Key = "tbill" | "hy_spread" | "next_event" | "index_iv";
export type Label1 = string;
export type State = "ok" | "gap";
export type Text5 = string;
export type Title = string;
export type Liquidity = Liquidity1 | Gap;
export type BackgroundText = string;
/**
 * @minItems 4
 * @maxItems 4
 */
export type Lines = [LiquidityLine, LiquidityLine, LiquidityLine, LiquidityLine];
export type Key1 = "reserves" | "money_market" | "credit" | "conditions";
export type Label2 = string;
export type Sentence = Sentence1 | Gap;
export type Sentence1 = string;
export type SourceText1 = string;
/**
 * @maxItems 4
 */
export type Taps =
  | []
  | [LiquidityTap]
  | [LiquidityTap, LiquidityTap]
  | [LiquidityTap, LiquidityTap, LiquidityTap]
  | [LiquidityTap, LiquidityTap, LiquidityTap, LiquidityTap];
export type Key2 = "stlfsi" | "reserves_over_assets" | "discount_window" | "net_liquidity";
export type Label3 = string;
/**
 * @minItems 1
 * @maxItems 4
 */
export type Rows1 =
  | [LiquidityRow]
  | [LiquidityRow, LiquidityRow]
  | [LiquidityRow, LiquidityRow, LiquidityRow]
  | [LiquidityRow, LiquidityRow, LiquidityRow, LiquidityRow];
export type Label4 = string;
export type Text6 = Text7 | Gap;
export type Text7 = string;
export type DataDay2 = string | null;
export type Note1 = Note2 | null;
export type Note2 = string;
export type LiquidityTitle = string;
/**
 * @maxItems 20
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
  | [string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string, string, string, string]
  | [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string
    ]
  | [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string
    ]
  | [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string
    ]
  | [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string
    ]
  | [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string
    ]
  | [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string
    ];
export type NotesTitle = string;
export type RunStatus = "ok" | "partial" | "failed";
export type Schema = "macro_page";
export type SchemaVersion = 2;
export type Sentence2 = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp1 = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp2 = string;
export type Text8 = string;
export type Title1 = string;

export interface MacroPageV2 {
  build_id: Id;
  data_times: DataTimes;
  ease: Ease;
  ease_title: EaseTitle;
  events: Events;
  events_title: EventsTitle;
  gaps: Gaps;
  generated_at: Timestamp;
  indicators: Indicators;
  indicators_title: IndicatorsTitle;
  lead?: Lead;
  liquidity: Liquidity;
  liquidity_title: LiquidityTitle;
  notes: Notes;
  notes_title: NotesTitle;
  run_id: Id;
  run_status: RunStatus;
  schema: Schema;
  schema_version: SchemaVersion;
  scope: Id;
  sentence: Sentence2;
  stale: StaleLabel;
  title: Title1;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: DataTime;
  source: Id1;
}
export interface Ease1 {
  aria_text: AriaText;
  figure: IndexPointsFigure;
  marker_percent: PercentOfTrackFigure;
  neutral_band: NeutralBand;
  reading_text: ReadingText;
  ticks: Ticks;
  week_change: WeekChange;
}
/**
 * A position on the stored 0 to 100 ease scale, or a change in it.
 */
export interface IndexPointsFigure {
  text: Text;
  tone?: Tone;
  unit: Unit;
  value: Value;
}
/**
 * A marker position on its track, 0 to 100.
 */
export interface PercentOfTrackFigure {
  text: Text1;
  unit: Unit1;
  value: Value1;
}
/**
 * The ends of the neutral band on the track, ``percent_of_track``.
 */
export interface NeutralBand {
  high: PercentOfTrackFigure;
  low: PercentOfTrackFigure;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  reason: Reason;
  text: Text2;
}
export interface Event {
  date: Date;
  days_to?: DaysTo;
  in_window?: InWindow;
  name_text: NameText;
  previous: Previous;
  when_text: WhenText;
}
export interface Indicator {
  change_text?: ChangeText;
  data_day?: DataDay;
  detail_href: DetailHref;
  group?: Group;
  hint?: Hint;
  label: Label;
  latest: Latest;
  name: Name;
  source_text: SourceText;
  status: Status;
}
/**
 * A fraction times 100, signed (``+2.1%``).
 */
export interface PercentFigure {
  text: Text3;
  tone?: Tone1;
  unit: Unit2;
  value: Value2;
}
/**
 * A level that is not a percent: index points, yen per dollar or a percent level.
 */
export interface LevelFigure {
  text: Text4;
  tone?: Tone2;
  unit: Unit3;
  value: Value3;
}
export interface Lead {
  note: Note;
  rows: Rows;
  title: Title;
}
export interface LeadRow {
  change_text: ChangeText1;
  data_day: DataDay1;
  hint: Hint1;
  key: Key;
  label: Label1;
  state: State;
  text: Text5;
}
export interface Liquidity1 {
  background_text: BackgroundText;
  lines: Lines;
  taps: Taps;
}
export interface LiquidityLine {
  key: Key1;
  label: Label2;
  sentence: Sentence;
  source_text: SourceText1;
}
export interface LiquidityTap {
  key: Key2;
  label: Label3;
  rows: Rows1;
  note: Note1;
}
export interface LiquidityRow {
  label: Label4;
  text: Text6;
  data_day: DataDay2;
}
/**
 * The times and the label; the browser only compares its clock with stale_after.
 */
export interface StaleLabel {
  data_time: Timestamp1;
  stale_after: Timestamp2;
  text: Text8;
}
