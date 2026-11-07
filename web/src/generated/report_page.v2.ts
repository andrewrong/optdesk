// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Left = string;
export type Right = string;
export type Accounts = Accounts1 | Gap;
/**
 * @maxItems 12
 */
export type Columns =
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
  | [string, string, string, string, string, string, string, string, string, string, string, string];
/**
 * @maxItems 20
 */
export type Rows =
  | []
  | [AccountRow]
  | [AccountRow, AccountRow]
  | [AccountRow, AccountRow, AccountRow]
  | [AccountRow, AccountRow, AccountRow, AccountRow]
  | [AccountRow, AccountRow, AccountRow, AccountRow, AccountRow]
  | [AccountRow, AccountRow, AccountRow, AccountRow, AccountRow, AccountRow]
  | [AccountRow, AccountRow, AccountRow, AccountRow, AccountRow, AccountRow, AccountRow]
  | [AccountRow, AccountRow, AccountRow, AccountRow, AccountRow, AccountRow, AccountRow, AccountRow]
  | [AccountRow, AccountRow, AccountRow, AccountRow, AccountRow, AccountRow, AccountRow, AccountRow, AccountRow]
  | [
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow
    ]
  | [
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow
    ]
  | [
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow
    ]
  | [
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow
    ]
  | [
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow
    ]
  | [
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow
    ]
  | [
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow
    ]
  | [
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow
    ]
  | [
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow
    ]
  | [
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow
    ]
  | [
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow,
      AccountRow
    ];
export type Account = Cell | Gap;
export type Text = string;
export type Tone = "up" | "down" | "flat" | "none";
export type Unit =
  | (
      | "USD"
      | "usd_signed"
      | "percent"
      | "share_percent"
      | "index_points"
      | "percent_of_track"
      | "vol_points"
      | "percentage_points"
    )
  | null;
export type Value = number | null;
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
export type Biggest = Cell | Gap;
export type DayAmount = Cell | Gap;
export type DayPct = Cell | Gap;
export type Net = Cell | Gap;
export type DataTimes = string;
export type GapText = string;
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
export type Key = string;
export type Href = string;
export type Label = string;
export type Meters =
  | []
  | [Meter]
  | [Meter, Meter]
  | [Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter, Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter]
  | [Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter, Meter]
  | [
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter
    ]
  | [
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter
    ]
  | [
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter
    ]
  | [
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter,
      Meter
    ]
  | null;
export type Account1 = string;
export type CashAfterAssignment = Cell | Gap;
export type MarginBuffer = Cell | Gap;
export type Movers = Movers1 | Gap;
/**
 * @maxItems 12
 */
export type Columns1 =
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
  | [string, string, string, string, string, string, string, string, string, string, string, string];
export type Heading = string;
export type Account2 = Cell | Gap;
export type DayAmount1 = Cell | Gap;
export type Note = Cell | Gap;
export type Symbol = Cell | Gap;
/**
 * @maxItems 50
 */
export type Rows1 = MoverRow[];
export type Note1 = string | null;
export type Say = string;
export type State = "ok" | "partial" | "gap";
export type Sub = string;
export type Title = string;
export type DataTimes1 = string;
export type GapText1 = string;
/**
 * @maxItems 20
 */
export type Gaps1 =
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
export type Key1 = string;
export type Meters1 =
  | []
  | [Meter1]
  | [Meter1, Meter1]
  | [Meter1, Meter1, Meter1]
  | [Meter1, Meter1, Meter1, Meter1]
  | [Meter1, Meter1, Meter1, Meter1, Meter1]
  | [Meter1, Meter1, Meter1, Meter1, Meter1, Meter1]
  | [Meter1, Meter1, Meter1, Meter1, Meter1, Meter1, Meter1]
  | [Meter1, Meter1, Meter1, Meter1, Meter1, Meter1, Meter1, Meter1]
  | [Meter1, Meter1, Meter1, Meter1, Meter1, Meter1, Meter1, Meter1, Meter1]
  | [Meter1, Meter1, Meter1, Meter1, Meter1, Meter1, Meter1, Meter1, Meter1, Meter1]
  | null;
export type Label1 = string;
export type Percent = SharePercentFigure | Gap;
export type Text2 = string;
export type Unit1 = "share_percent";
export type Value1 = number;
export type Tone1 = "up" | "down" | "flat" | "none";
export type Value2 =
  | CountFigure
  | MoneyCell
  | UsdFigure
  | UsdSignedFigure
  | PercentFigure
  | SharePercentFigure
  | IndexLevelFigure
  | VolPointsFigure
  | PercentagePointsFigure
  | Gap;
export type Text3 = string;
export type Unit2 = "count";
export type Value3 = number;
export type Text4 = string;
export type Unit3 = "USD";
export type Value4 = number;
export type Currency = "USD" | "JPY";
export type Text5 = string;
export type Tone2 = ("up" | "down" | "flat" | "none") | null;
export type Unit4 = "usd" | "usd_per_share" | "usd_signed";
export type Value5 = number;
export type Text6 = string;
export type Tone3 = ("up" | "down" | "flat" | "none") | null;
export type Unit5 = "usd_signed";
export type Value6 = number;
export type Text7 = string;
export type Tone4 = ("up" | "down" | "flat" | "none") | null;
export type Unit6 = "percent";
export type Value7 = number;
export type Text8 = string;
export type Unit7 = "index_points";
export type Value8 = number;
export type Source = string | null;
export type Text9 = string;
export type Tone5 = ("up" | "down" | "flat" | "none") | null;
export type Unit8 = "vol_points";
export type Value9 = number;
export type Text10 = string;
export type Tone6 = ("up" | "down" | "flat" | "none") | null;
export type Unit9 = "percentage_points";
export type Value10 = number;
export type Note2 = string | null;
export type Say1 = string;
export type State1 = "ok" | "partial" | "gap";
export type Sub1 = string;
export type Title1 = string;
export type Candidates = Candidates1 | Gap | null;
/**
 * @maxItems 20
 */
export type Groups =
  | []
  | [CandidateGroup]
  | [CandidateGroup, CandidateGroup]
  | [CandidateGroup, CandidateGroup, CandidateGroup]
  | [CandidateGroup, CandidateGroup, CandidateGroup, CandidateGroup]
  | [CandidateGroup, CandidateGroup, CandidateGroup, CandidateGroup, CandidateGroup]
  | [CandidateGroup, CandidateGroup, CandidateGroup, CandidateGroup, CandidateGroup, CandidateGroup]
  | [CandidateGroup, CandidateGroup, CandidateGroup, CandidateGroup, CandidateGroup, CandidateGroup, CandidateGroup]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ]
  | [
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup,
      CandidateGroup
    ];
export type Line1 = string;
export type Line2 = string;
export type Liquidity = string;
export type Structure = string;
export type Symbol1 = string;
export type Verdict = string;
/**
 * @maxItems 50
 */
export type Items = CandidateItem[];
export type Label2 = string;
export type Heading1 = string;
export type DataTimes2 = string;
export type GapText2 = string;
/**
 * @maxItems 20
 */
export type Gaps2 =
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
export type Key2 = string;
export type Note3 = string | null;
export type Say2 = string;
export type State2 = "ok" | "partial" | "gap";
export type Sub2 = string;
export type Title2 = string;
export type Trades = Trades1 | Gap;
/**
 * @maxItems 12
 */
export type Columns2 =
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
  | [string, string, string, string, string, string, string, string, string, string, string, string];
export type Footnote = string | null;
export type Account3 = Cell | Gap;
export type Action = Cell | Gap;
export type Instrument = Cell | Gap;
export type Price = Cell | Gap;
export type Quantity = Cell | Gap;
export type Time = Cell | Gap;
/**
 * @maxItems 200
 */
export type Rows2 = TradeRow[];
export type Annualized =
  | CountFigure
  | MoneyCell
  | UsdFigure
  | UsdSignedFigure
  | PercentFigure
  | SharePercentFigure
  | IndexLevelFigure
  | VolPointsFigure
  | PercentagePointsFigure
  | Gap;
export type Premium =
  | CountFigure
  | MoneyCell
  | UsdFigure
  | UsdSignedFigure
  | PercentFigure
  | SharePercentFigure
  | IndexLevelFigure
  | VolPointsFigure
  | PercentagePointsFigure
  | Gap;
export type Strike =
  | CountFigure
  | MoneyCell
  | UsdFigure
  | UsdSignedFigure
  | PercentFigure
  | SharePercentFigure
  | IndexLevelFigure
  | VolPointsFigure
  | PercentagePointsFigure
  | Gap;
export type Symbol2 = string;
export type Text11 = string;
export type Tone7 = "up" | "down" | "flat" | "none";
export type DataTimes3 = string;
export type GapText3 = string;
/**
 * @maxItems 20
 */
export type Gaps3 =
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
export type Groups1 =
  | []
  | [SectorGroup]
  | [SectorGroup, SectorGroup]
  | [SectorGroup, SectorGroup, SectorGroup]
  | [SectorGroup, SectorGroup, SectorGroup, SectorGroup]
  | [SectorGroup, SectorGroup, SectorGroup, SectorGroup, SectorGroup]
  | [SectorGroup, SectorGroup, SectorGroup, SectorGroup, SectorGroup, SectorGroup]
  | [SectorGroup, SectorGroup, SectorGroup, SectorGroup, SectorGroup, SectorGroup, SectorGroup]
  | [SectorGroup, SectorGroup, SectorGroup, SectorGroup, SectorGroup, SectorGroup, SectorGroup, SectorGroup]
  | [
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup
    ]
  | [
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup
    ]
  | [
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup
    ]
  | [
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup,
      SectorGroup
    ]
  | null;
export type Columns3 = string[];
export type Dim = boolean;
/**
 * @maxItems 6
 */
export type Figures =
  | []
  | [
      | CountFigure
      | MoneyCell
      | UsdFigure
      | UsdSignedFigure
      | PercentFigure
      | SharePercentFigure
      | IndexLevelFigure
      | VolPointsFigure
      | PercentagePointsFigure
      | Gap
    ]
  | [
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      )
    ]
  | [
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      )
    ]
  | [
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      )
    ]
  | [
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      )
    ]
  | [
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      ),
      (
        | CountFigure
        | MoneyCell
        | UsdFigure
        | UsdSignedFigure
        | PercentFigure
        | SharePercentFigure
        | IndexLevelFigure
        | VolPointsFigure
        | PercentagePointsFigure
        | Gap
      )
    ];
export type Kind = "candidate";
export type Symbol3 = string;
export type Text12 = string;
export type Tone8 = "up" | "down" | "flat" | "none";
export type Kind1 = "watch";
export type Symbol4 = string;
export type Text13 = string;
export type Tone9 = "up" | "down" | "flat" | "none";
/**
 * @maxItems 30
 */
export type Rows3 = (CandidateRow | WatchRow)[];
export type Sector = string;
export type WatchTitle = string | null;
export type Key3 = string;
export type Note4 = string | null;
export type Credit =
  | CountFigure
  | MoneyCell
  | UsdFigure
  | UsdSignedFigure
  | PercentFigure
  | SharePercentFigure
  | IndexLevelFigure
  | VolPointsFigure
  | PercentagePointsFigure
  | Gap;
export type LongStrike =
  | CountFigure
  | MoneyCell
  | UsdFigure
  | UsdSignedFigure
  | PercentFigure
  | SharePercentFigure
  | IndexLevelFigure
  | VolPointsFigure
  | PercentagePointsFigure
  | Gap;
export type MaxLoss =
  | CountFigure
  | MoneyCell
  | UsdFigure
  | UsdSignedFigure
  | PercentFigure
  | SharePercentFigure
  | IndexLevelFigure
  | VolPointsFigure
  | PercentagePointsFigure
  | Gap;
export type ShortStrike =
  | CountFigure
  | MoneyCell
  | UsdFigure
  | UsdSignedFigure
  | PercentFigure
  | SharePercentFigure
  | IndexLevelFigure
  | VolPointsFigure
  | PercentagePointsFigure
  | Gap;
export type Symbol5 = string;
export type Text14 = string;
export type Tone10 = "up" | "down" | "flat" | "none";
export type Say3 = string;
export type State3 = "ok" | "partial" | "gap";
export type Sub3 = string;
export type Title3 = string;
export type ChartAria = string;
export type DataTimes4 = string;
export type GapText4 = string;
/**
 * @maxItems 20
 */
export type Gaps4 =
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
export type Key4 = string;
export type Note5 = string | null;
export type Proxies =
  | []
  | [Proxy]
  | [Proxy, Proxy]
  | [Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy]
  | [Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy, Proxy]
  | [
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy
    ]
  | [
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy
    ]
  | [
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy
    ]
  | [
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy,
      Proxy
    ]
  | Gap;
export type Day = Cell | Gap;
export type FiveDay = Cell | Gap;
export type Label3 = string;
export type Symbol6 = string;
export type Detail = string;
export type Title4 = string;
export type Say4 = string;
export type Sector1 = Sector2 | Gap;
/**
 * @maxItems 12
 */
export type Columns4 =
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
  | [string, string, string, string, string, string, string, string, string, string, string, string];
export type Heading2 = string;
export type D1 = Cell | Gap;
export type D20 = Cell | Gap;
export type D5 = Cell | Gap;
export type Etf = Cell | Gap;
export type Sector3 = Cell | Gap;
/**
 * @maxItems 30
 */
export type Rows4 = SectorRow[];
export type State4 = "ok" | "partial" | "gap";
export type Sub4 = string;
export type Title5 = string;
export type DataTimes5 = string;
/**
 * @maxItems 6
 */
export type Figures1 =
  | []
  | [MarketFigure]
  | [MarketFigure, MarketFigure]
  | [MarketFigure, MarketFigure, MarketFigure]
  | [MarketFigure, MarketFigure, MarketFigure, MarketFigure]
  | [MarketFigure, MarketFigure, MarketFigure, MarketFigure, MarketFigure]
  | [MarketFigure, MarketFigure, MarketFigure, MarketFigure, MarketFigure, MarketFigure];
export type Figure =
  | CountFigure
  | MoneyCell
  | UsdFigure
  | UsdSignedFigure
  | PercentFigure
  | SharePercentFigure
  | IndexLevelFigure
  | VolPointsFigure
  | PercentagePointsFigure
  | Gap;
export type Label4 = string;
export type GapText5 = string;
/**
 * @maxItems 20
 */
export type Gaps5 =
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
export type Key5 = string;
export type Note6 = string | null;
export type Say5 = string;
export type Sentence = string | null;
export type State5 = "ok" | "partial" | "gap";
export type Sub5 = string;
export type Title6 = string;
export type Companies = Companies1 | Gap;
/**
 * @maxItems 12
 */
export type Columns5 =
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
  | [string, string, string, string, string, string, string, string, string, string, string, string];
export type Day1 = Cell | Gap;
export type News = Cell | Gap;
export type Symbol7 = Cell | Gap;
/**
 * @maxItems 50
 */
export type Rows5 = CompanyRow[];
export type DataTimes6 = string;
export type GapText6 = string;
/**
 * @maxItems 20
 */
export type Gaps6 =
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
export type Indices = Indices1 | Gap;
/**
 * @maxItems 12
 */
export type Columns6 =
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
  | [string, string, string, string, string, string, string, string, string, string, string, string];
/**
 * @maxItems 20
 */
export type Rows6 =
  | []
  | [IndexRow]
  | [IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow]
  | [
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow
    ]
  | [
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow
    ]
  | [
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow
    ]
  | [
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow
    ]
  | [
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow
    ]
  | [
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow
    ]
  | [
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow
    ]
  | [
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow
    ]
  | [
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow,
      IndexRow
    ];
export type Close = Cell | Gap;
export type Day2 = Cell | Gap;
export type FiveDay1 = Cell | Gap;
export type Name = Cell | Gap;
export type SourceText = string | null;
export type Key6 = string;
export type MutedNote = string | null;
export type Note7 = string | null;
export type Say6 = string;
export type State6 = "ok" | "partial" | "gap";
export type Sub6 = string;
export type Title7 = string;
export type DataTimes7 = string;
export type GapText7 = string;
/**
 * @maxItems 20
 */
export type Gaps7 =
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
export type Key7 = string;
export type Note8 = string | null;
export type Rest = RestRow[] | null;
/**
 * @minItems 5
 * @maxItems 5
 */
export type Cells = [string | Gap, string | Gap, string | Gap, string | Gap, string | Gap];
export type RestColumns =
  | []
  | [string]
  | [string, string]
  | [string, string, string]
  | [string, string, string, string]
  | [string, string, string, string, string]
  | null;
export type RestTitle = string | null;
export type Rows7 = Reminder[] | null;
export type Text15 = string;
export type Tone11 = "up" | "down" | "flat" | "none";
export type Say7 = string;
export type State7 = "ok" | "partial" | "gap";
export type Sub7 = string;
export type Title8 = string;
export type DataTimes8 = string;
export type GapText8 = string;
/**
 * @maxItems 20
 */
export type Gaps8 =
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
export type Items1 = RiskItem[] | Gap;
export type Detail1 = string;
export type IconKey = string;
export type Kind2 = string;
export type Level = "n" | "w" | "d";
export type Title9 = string;
export type Key8 = string;
export type Note9 = string | null;
export type Say8 = string;
export type State8 = "ok" | "partial" | "gap";
export type Sub8 = string;
export type Title10 = string;
export type Collapsed = boolean;
export type DataTimes9 = string;
export type GapText9 = string;
/**
 * @maxItems 20
 */
export type Gaps9 =
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
export type Key9 = string;
export type Lines =
  | []
  | [StatusLine]
  | [StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine, StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | null;
export type Label5 = string;
export type Text16 = string;
export type Note10 = string | null;
export type Say9 = string;
export type State9 = "ok" | "partial" | "gap";
export type Sub9 = string;
export type Title11 = string;
export type DataTimes10 = string;
export type GapText10 = string;
/**
 * @maxItems 20
 */
export type Gaps10 =
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
export type Key10 = string;
export type Note11 = string | null;
export type Say10 = string;
export type State10 = "ok" | "partial" | "gap";
export type Sub10 = string;
export type Tiles1 = [Tile, Tile, Tile, Tile] | null;
export type Figure1 =
  | CountFigure
  | MoneyCell
  | UsdFigure
  | UsdSignedFigure
  | PercentFigure
  | SharePercentFigure
  | IndexLevelFigure
  | VolPointsFigure
  | PercentagePointsFigure
  | Gap;
export type Label6 = string;
export type Note12 = string | null;
export type Tone12 = "up" | "down" | "flat" | "none";
export type Title12 = string;
export type Chips =
  | []
  | [Chip]
  | [Chip, Chip]
  | [Chip, Chip, Chip]
  | [Chip, Chip, Chip, Chip]
  | [Chip, Chip, Chip, Chip, Chip]
  | [Chip, Chip, Chip, Chip, Chip, Chip]
  | null;
export type Text17 = string;
export type DesignNote = string | null;
export type GeneratedAt = string;
export type Market = "US" | "JP";
export type Say11 = string;
export type Schema = "report_page";
export type SchemaVersion = 2;
export type SessionDate = string;
export type Slot = "open" | "close";
export type DataTime = string;
export type StaleAfter = string;
export type Text18 = string;
/**
 * @maxItems 20
 */
export type StatusLines =
  | []
  | [StatusLine]
  | [StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine, StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine]
  | [StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine, StatusLine]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ]
  | [
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine,
      StatusLine
    ];
export type Title13 = string;

export interface ReportPage {
  banner: Banner;
  blocks: Blocks;
  chips?: Chips;
  design_note?: DesignNote;
  generated_at: GeneratedAt;
  market: Market;
  say: Say11;
  schema: Schema;
  schema_version: SchemaVersion;
  session_date: SessionDate;
  slot: Slot;
  stale?: StaleLabel | null;
  status_lines: StatusLines;
  title: Title13;
}
export interface Banner {
  left: Left;
  right: Right;
}
/**
 * The five close blocks under fixed keys, then the six optional open blocks (the writer sorts keys; ``in_order`` is the board order).
 */
export interface Blocks {
  account_moves: AccountMovesBlock;
  accounts?: Accounts2 | null;
  actions: ActionsBlock;
  candidates?: Candidates2 | null;
  global_assets: GlobalAssetsBlock;
  market_line?: MarketLine | null;
  market_watch: MarketWatchBlock;
  reminders?: Reminders | null;
  risks: RisksBlock;
  system_status?: SystemStatus | null;
  tiles?: Tiles | null;
}
export interface AccountMovesBlock {
  accounts: Accounts;
  data_times: DataTimes;
  gap_text?: GapText;
  gaps: Gaps;
  key: Key;
  link?: Link | null;
  meters?: Meters;
  movers: Movers;
  note?: Note1;
  say: Say;
  state: State;
  sub: Sub;
  title: Title;
}
export interface Accounts1 {
  columns: Columns;
  rows: Rows;
}
export interface AccountRow {
  account: Account;
  biggest: Biggest;
  day_amount: DayAmount;
  day_pct: DayPct;
  net: Net;
}
/**
 * One displayed table cell.
 */
export interface Cell {
  text: Text;
  tone: Tone;
  unit?: Unit;
  value?: Value;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  reason: Reason;
  text: Text1;
}
export interface Link {
  href: Href;
  label: Label;
}
export interface Meter {
  account: Account1;
  cash_after_assignment: CashAfterAssignment;
  margin_buffer: MarginBuffer;
}
export interface Movers1 {
  columns: Columns1;
  heading: Heading;
  rows: Rows1;
}
export interface MoverRow {
  account: Account2;
  day_amount: DayAmount1;
  note: Note;
  symbol: Symbol;
}
export interface Accounts2 {
  data_times: DataTimes1;
  gap_text?: GapText1;
  gaps: Gaps1;
  key: Key1;
  link?: Link | null;
  meters?: Meters1;
  note?: Note2;
  say: Say1;
  state: State1;
  sub: Sub1;
  title: Title1;
}
/**
 * An account meter: ``percent`` is the share of the account's limit used, whole percent.
 */
export interface Meter1 {
  label: Label1;
  percent: Percent;
  tone: Tone1;
  value: Value2;
}
/**
 * A share in whole percent, 0 to 100.
 */
export interface SharePercentFigure {
  text: Text2;
  unit: Unit1;
  value: Value1;
}
/**
 * A count of things (opportunities, groups), whole and not negative.
 */
export interface CountFigure {
  text: Text3;
  unit: Unit2;
  value: Value3;
}
/**
 * A money cell with a currency and, for JPY, an optional display-only ``approx_usd``.
 */
export interface MoneyCell {
  approx_usd?: UsdFigure | null;
  currency: Currency;
  text: Text5;
  tone?: Tone2;
  unit: Unit4;
  value: Value5;
}
/**
 * A displayed US-dollar amount (two decimals in ``value``, whole dollars in ``text``).
 */
export interface UsdFigure {
  text: Text4;
  unit: Unit3;
  value: Value4;
}
/**
 * A signed US-dollar amount; ``text`` starts with ``+`` or ``−`` (U+2212).
 */
export interface UsdSignedFigure {
  text: Text6;
  tone?: Tone3;
  unit: Unit5;
  value: Value6;
}
/**
 * A fraction times 100, signed (``+2.1%``).
 */
export interface PercentFigure {
  text: Text7;
  tone?: Tone4;
  unit: Unit6;
  value: Value7;
}
/**
 * The level of a published market index in index points as its publisher states it (a Cboe
 * volatility index: 18.20 is 18.20 points). Not the 0 to 100 ease scale of ``IndexPointsFigure``:
 * the level has no upper bound in practice, the cap here only rejects a corrupt value.
 */
export interface IndexLevelFigure {
  text: Text8;
  unit: Unit7;
  value: Value8;
}
/**
 * Vol points (GLOSSARY "Vol point"), signed (``+2.1``).
 */
export interface VolPointsFigure {
  source?: Source;
  text: Text9;
  tone?: Tone5;
  unit: Unit8;
  value: Value9;
}
/**
 * A difference of two percents, signed.
 */
export interface PercentagePointsFigure {
  text: Text10;
  tone?: Tone6;
  unit: Unit9;
  value: Value10;
}
export interface ActionsBlock {
  candidates?: Candidates;
  data_times: DataTimes2;
  gap_text?: GapText2;
  gaps: Gaps2;
  key: Key2;
  link?: Link | null;
  note?: Note3;
  say: Say2;
  state: State2;
  sub: Sub2;
  title: Title2;
  trades: Trades;
}
export interface Candidates1 {
  groups: Groups;
  heading: Heading1;
}
export interface CandidateGroup {
  items: Items;
  label: Label2;
}
export interface CandidateItem {
  line1: Line1;
  line2: Line2;
  liquidity: Liquidity;
  structure: Structure;
  symbol: Symbol1;
  verdict: Verdict;
}
export interface Trades1 {
  columns: Columns2;
  footnote?: Footnote;
  rows: Rows2;
}
export interface TradeRow {
  account: Account3;
  action: Action;
  instrument: Instrument;
  price: Price;
  quantity: Quantity;
  time: Time;
}
/**
 * 候选: sector groups, at most one covered call and one put spread.
 */
export interface Candidates2 {
  covered_call?: CoveredCall | null;
  data_times: DataTimes3;
  full_page?: Link | null;
  gap_text?: GapText3;
  gaps: Gaps3;
  groups?: Groups1;
  key: Key3;
  link?: Link | null;
  note?: Note4;
  put_spread?: PutSpread | null;
  say: Say3;
  state: State3;
  sub: Sub3;
  title: Title3;
}
export interface CoveredCall {
  annualized: Annualized;
  premium: Premium;
  strike: Strike;
  symbol: Symbol2;
  text: Text11;
  tone: Tone7;
}
export interface SectorGroup {
  columns: Columns3;
  rows: Rows3;
  sector: Sector;
  watch_title?: WatchTitle;
}
export interface CandidateRow {
  dim?: Dim;
  figures: Figures;
  kind?: Kind;
  symbol: Symbol3;
  text: Text12;
  tone: Tone8;
}
/**
 * A 待定/看名单 row: a name that is not a candidate yet, with the reason in ``text``.
 */
export interface WatchRow {
  kind?: Kind1;
  symbol: Symbol4;
  text: Text13;
  tone: Tone9;
}
export interface PutSpread {
  credit: Credit;
  long_strike: LongStrike;
  max_loss: MaxLoss;
  short_strike: ShortStrike;
  symbol: Symbol5;
  text: Text14;
  tone: Tone10;
}
export interface GlobalAssetsBlock {
  chart_aria: ChartAria;
  data_times: DataTimes4;
  gap_text?: GapText4;
  gaps: Gaps4;
  key: Key4;
  link?: Link | null;
  note?: Note5;
  proxies: Proxies;
  rotation?: Rotation | null;
  say: Say4;
  sector: Sector1;
  state: State4;
  sub: Sub4;
  title: Title5;
}
export interface Proxy {
  day: Day;
  five_day: FiveDay;
  label: Label3;
  symbol: Symbol6;
}
export interface Rotation {
  detail: Detail;
  title: Title4;
}
export interface Sector2 {
  columns: Columns4;
  heading: Heading2;
  rows: Rows4;
}
export interface SectorRow {
  d1: D1;
  d20: D20;
  d5: D5;
  etf: Etf;
  sector: Sector3;
}
/**
 * 市场一行: one sentence and up to six figures.
 */
export interface MarketLine {
  data_times: DataTimes5;
  figures?: Figures1;
  gap_text?: GapText5;
  gaps: Gaps5;
  key: Key5;
  link?: Link | null;
  note?: Note6;
  say: Say5;
  sentence?: Sentence;
  state: State5;
  sub: Sub5;
  title: Title6;
}
export interface MarketFigure {
  figure: Figure;
  label: Label4;
}
export interface MarketWatchBlock {
  companies: Companies;
  data_times: DataTimes6;
  gap_text?: GapText6;
  gaps: Gaps6;
  indices: Indices;
  key: Key6;
  link?: Link | null;
  muted_note?: MutedNote;
  note?: Note7;
  say: Say6;
  state: State6;
  sub: Sub6;
  title: Title7;
}
export interface Companies1 {
  columns: Columns5;
  rows: Rows5;
}
export interface CompanyRow {
  day: Day1;
  news: News;
  symbol: Symbol7;
}
export interface Indices1 {
  columns: Columns6;
  rows: Rows6;
}
export interface IndexRow {
  close: Close;
  day: Day2;
  five_day: FiveDay1;
  name: Name;
  source_text?: SourceText;
}
/**
 * 持仓提醒: the groups to look at (``rows``) and, folded below, the calm rest as a table.
 */
export interface Reminders {
  data_times: DataTimes7;
  gap_text?: GapText7;
  gaps: Gaps7;
  key: Key7;
  link?: Link | null;
  note?: Note8;
  rest?: Rest;
  rest_columns?: RestColumns;
  rest_title?: RestTitle;
  rows?: Rows7;
  say: Say7;
  state: State7;
  sub: Sub7;
  title: Title8;
}
/**
 * One calm group of the folded rest table: five cells in the order of ``Reminders.rest_columns``
 * (标的, 组合, 户, 到期, 已赚); a value that is not stored is a ``Gap``, never 0 or empty.
 */
export interface RestRow {
  cells: Cells;
}
export interface Reminder {
  link?: Link | null;
  text: Text15;
  tone: Tone11;
}
export interface RisksBlock {
  data_times: DataTimes8;
  gap_text?: GapText8;
  gaps: Gaps8;
  items: Items1;
  key: Key8;
  link?: Link | null;
  note?: Note9;
  say: Say8;
  state: State8;
  sub: Sub8;
  title: Title10;
}
export interface RiskItem {
  detail: Detail1;
  icon_key: IconKey;
  kind: Kind2;
  level: Level;
  title: Title9;
}
/**
 * 系统状态, collapsible (``collapsed`` is the initial state).
 */
export interface SystemStatus {
  collapsed?: Collapsed;
  data_times: DataTimes9;
  gap_text?: GapText9;
  gaps: Gaps9;
  key: Key9;
  lines?: Lines;
  link?: Link | null;
  note?: Note10;
  say: Say9;
  state: State9;
  sub: Sub9;
  title: Title11;
}
/**
 * One line of a folded 系统状态 list (the page's, and the ``SystemStatus`` block's).
 */
export interface StatusLine {
  label: Label5;
  text: Text16;
}
/**
 * The four tiles of the board, in board order.
 */
export interface Tiles {
  data_times: DataTimes10;
  gap_text?: GapText10;
  gaps: Gaps10;
  key: Key10;
  link?: Link | null;
  note?: Note11;
  say: Say10;
  state: State10;
  sub: Sub10;
  tiles?: Tiles1;
  title: Title12;
}
export interface Tile {
  figure: Figure1;
  label: Label6;
  note?: Note12;
  tone: Tone12;
}
export interface Chip {
  text: Text17;
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
  data_time: DataTime;
  stale_after: StaleAfter;
  text: Text18;
}
