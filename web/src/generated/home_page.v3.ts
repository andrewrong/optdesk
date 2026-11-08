// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Schema = "home_page";
export type SchemaVersion = 3;
export type Id = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
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
export type Timestamp1 = string;
export type Id1 = string;
export type RunStatus = "ok" | "partial" | "failed";
export type Title = string;
export type Say = string;
export type GapLine = string;
/**
 * @maxItems 20
 */
export type Gaps =
  | []
  | [GapItem]
  | [GapItem, GapItem]
  | [GapItem, GapItem, GapItem]
  | [GapItem, GapItem, GapItem, GapItem]
  | [GapItem, GapItem, GapItem, GapItem, GapItem]
  | [GapItem, GapItem, GapItem, GapItem, GapItem, GapItem]
  | [GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem]
  | [GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem]
  | [GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem]
  | [GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem]
  | [GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem]
  | [GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem, GapItem]
  | [
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem
    ]
  | [
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem
    ]
  | [
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem
    ]
  | [
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem
    ]
  | [
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem
    ]
  | [
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem
    ]
  | [
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem
    ]
  | [
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem,
      GapItem
    ];
export type Card = string;
export type Text = string;
/**
 * @minItems 1
 * @maxItems 10
 */
export type Items =
  | [OverviewLine]
  | [OverviewLine, OverviewLine]
  | [OverviewLine, OverviewLine, OverviewLine]
  | [OverviewLine, OverviewLine, OverviewLine, OverviewLine]
  | [OverviewLine, OverviewLine, OverviewLine, OverviewLine, OverviewLine]
  | [OverviewLine, OverviewLine, OverviewLine, OverviewLine, OverviewLine, OverviewLine]
  | [OverviewLine, OverviewLine, OverviewLine, OverviewLine, OverviewLine, OverviewLine, OverviewLine]
  | [OverviewLine, OverviewLine, OverviewLine, OverviewLine, OverviewLine, OverviewLine, OverviewLine, OverviewLine]
  | [
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine
    ]
  | [
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine,
      OverviewLine
    ];
export type Href = string;
export type Label = string;
export type Text1 = string;
/**
 * @maxItems 10
 */
export type ReportLinks =
  | []
  | [BlockLink]
  | [BlockLink, BlockLink]
  | [BlockLink, BlockLink, BlockLink]
  | [BlockLink, BlockLink, BlockLink, BlockLink]
  | [BlockLink, BlockLink, BlockLink, BlockLink, BlockLink]
  | [BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink]
  | [BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink]
  | [BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink]
  | [BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink]
  | [BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink, BlockLink];
export type Href1 = string;
export type Text2 = string;
export type Id2 = string;
export type DataTimesText = string;
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
export type Text3 = string;
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
/**
 * An RFC 3339 time with any offset.
 */
export type DataTime = string;
/**
 * An RFC 3339 time with any offset.
 */
export type StaleAfter = string;
export type Text4 = string;
export type Subtitle = string;
export type Caption = UsdSignedFigure | Gap;
export type Text5 = string;
export type Tone = ("up" | "down" | "flat" | "none") | null;
export type Unit = "usd_signed";
export type Value = number;
export type Explain = string | null;
export type Figure = SharePercentFigure | Caption1 | Gap;
export type Text6 = string;
export type Unit1 = "share_percent";
export type Value1 = number;
export type Text7 = string;
export type Label1 = string;
export type Caption2 = PercentFigure | Gap;
export type Text8 = string;
export type Tone1 = ("up" | "down" | "flat" | "none") | null;
export type Unit2 = "percent";
export type Value2 = number;
export type Explain1 = string | null;
export type Figure1 = UsdSignedFigure | Gap;
export type Label2 = string;
export type Caption3 = PercentFigure | Caption1 | Gap;
export type Explain2 = string | null;
export type Figure2 = UsdSignedFigure | Gap;
export type Label3 = string;
export type Explain3 = string | null;
export type Figure3 = UsdFigure | Gap;
export type Text9 = string;
export type Unit3 = "USD";
export type Value3 = number;
export type Label4 = string;
export type Title1 = string;
export type Currency = "USD";
/**
 * @maxItems 20
 */
export type DataTimes1 =
  | []
  | [SourceTime1]
  | [SourceTime1, SourceTime1]
  | [SourceTime1, SourceTime1, SourceTime1]
  | [SourceTime1, SourceTime1, SourceTime1, SourceTime1]
  | [SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1]
  | [SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1]
  | [SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1]
  | [SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ];
/**
 * An RFC 3339 time with any offset.
 */
export type DataTime1 = string;
export type Id3 = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp2 = string;
export type RunStatus1 = "ok" | "partial" | "failed";
export type Schema1 = "household_summary";
export type SchemaVersion1 = 2;
/**
 * @minItems 5
 * @maxItems 5
 */
export type Columns = [Column, Column, Column, Column, Column];
export type Id4 = "company" | "price" | "earnings" | "day_change" | "five_day_change";
export type Label5 = string;
export type DataTimesText1 = string;
/**
 * @maxItems 20
 */
export type Gaps2 =
  | []
  | [Gap1]
  | [Gap1, Gap1]
  | [Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1
    ];
export type Kind = "gap";
export type Reason1 =
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
export type Text10 = string;
/**
 * @maxItems 20
 */
export type Notes1 =
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
export type Currency1 = "USD" | "JPY";
export type DayChange = PercentFigure1 | Gap1;
export type Kind1 = "value";
export type Text11 = string;
export type Tone2 = "up" | "down" | "flat" | "none";
export type Unit4 = "percent";
export type Value4 = number;
export type DaySharp = SharpMark | NoMark;
export type Kind2 = "sharp";
export type Text12 = string;
export type Kind3 = "none";
export type Earnings = EarningsCell | Gap1;
export type Date = string;
export type Kind4 = "value";
export type Text13 = string;
export type FiveDayChange = PercentFigure1 | Gap1;
export type FiveDaySharp = SharpMark | NoMark;
export type Price = PriceFigure | Gap1;
export type Kind5 = "value";
export type Text14 = string;
export type Unit5 = "price";
export type Value5 = number;
export type Symbol = string;
export type DataDay = string;
export type Hv30 = PercentFigure1 | Gap1;
export type Iv30 = PercentFigure1 | Gap1;
export type Ivp = PercentFigure1 | Gap1;
export type Label6 = string;
export type Reconciliation = string;
export type Source = string;
/**
 * @maxItems 30
 */
export type Rows = Row[];
export type Say1 = string;
export type SharpNote = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp3 = string;
export type IsStale = boolean;
export type Text15 = string;
export type Subtitle1 = string;
export type Title2 = string;
/**
 * @maxItems 20
 */
export type DataTimes2 =
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
export type Schema2 = "held_companies_summary";
export type SchemaVersion2 = 3;
export type DataTimesText2 = string;
export type Ease = Ease1 | Gap;
export type AriaText = string;
export type Text16 = string;
export type Tone3 = ("up" | "down" | "flat" | "none") | null;
export type Unit6 = "index_points";
export type Value6 = number;
export type Text17 = string;
export type Unit7 = "percent_of_track";
export type Value7 = number;
export type ReadingText = string;
/**
 * @minItems 5
 * @maxItems 5
 */
export type Ticks = [string, string, string, string, string];
export type WeekChange = IndexPointsFigure | Gap;
/**
 * @maxItems 8
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
  | [Event, Event, Event, Event, Event, Event, Event, Event];
export type Date1 = string;
export type NameText = string;
export type Previous = string | Gap;
export type WhenText = string;
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
/**
 * @maxItems 20
 */
export type Notes2 =
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
export type Say2 = string;
export type Title3 = string;
/**
 * @maxItems 20
 */
export type DataTimes3 =
  | []
  | [SourceTime1]
  | [SourceTime1, SourceTime1]
  | [SourceTime1, SourceTime1, SourceTime1]
  | [SourceTime1, SourceTime1, SourceTime1, SourceTime1]
  | [SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1]
  | [SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1]
  | [SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1]
  | [SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1, SourceTime1]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ]
  | [
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1,
      SourceTime1
    ];
export type RunStatus2 = "ok" | "partial" | "failed";
export type Schema3 = "macro_summary";
export type SchemaVersion3 = 2;
export type AriaText1 = string;
/**
 * @minItems 2
 * @maxItems 2
 */
export type EndLabels = [string, string];
export type BandHigh = number;
export type BandLow = number;
export type Close = number;
export type Label7 = string;
export type Ma200 = number;
/**
 * @maxItems 120
 */
export type Points = ChartPoint[];
/**
 * @minItems 3
 * @maxItems 3
 */
export type SeriesNames = [string, string, string];
export type DataTimesText3 = string;
/**
 * @maxItems 20
 */
export type Gaps4 =
  | []
  | [Gap1]
  | [Gap1, Gap1]
  | [Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1, Gap1]
  | [
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1,
      Gap1
    ];
/**
 * @minItems 4
 * @maxItems 4
 */
export type Columns1 = [string, string, string, string];
/**
 * @maxItems 10
 */
export type Rows1 =
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
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow];
export type Iv = VolPointsFigure | Gap1;
export type Kind6 = "value";
export type Source1 = string;
export type Text18 = string;
export type Tone4 = "up" | "down" | "flat" | "none";
export type Unit8 = "vol_points";
export type Value8 = number;
export type IvMinusRv20 = VolPointsFigure | Gap1;
export type Name = string;
export type Rv20 = VolPointsFigure | Gap1;
export type SourceText = string;
/**
 * @maxItems 20
 */
export type Notes3 =
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
export type Say3 = string;
export type Subtitle2 = string;
export type Title4 = string;
/**
 * @minItems 3
 * @maxItems 3
 */
export type VolIndices = [VolIndexItem, VolIndexItem, VolIndexItem];
export type Day = string;
export type Level = IndexLevelFigure | Gap1;
export type Kind7 = "value";
export type Text19 = string;
export type Unit9 = "index_points";
export type Value9 = number;
export type Name1 = string;
export type SourceText1 = string;
export type VolIndicesNote = string;
/**
 * @maxItems 20
 */
export type DataTimes4 =
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
export type Schema4 = "market_summary";
export type SchemaVersion4 = 3;
export type ClosedSeriesText = string;
/**
 * @maxItems 20
 */
export type Gaps5 =
  | []
  | [Gap2]
  | [Gap2, Gap2]
  | [Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2, Gap2]
  | [
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2,
      Gap2
    ];
export type Kind8 = "gap";
export type Reason2 = "missing_quote" | "no_data" | "unsupported_currency";
export type Text20 = string;
/**
 * @maxItems 20
 */
export type Notes4 =
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
/**
 * @minItems 6
 * @maxItems 6
 */
export type PremiumSeries6M = [SeriesPoint, SeriesPoint, SeriesPoint, SeriesPoint, SeriesPoint, SeriesPoint];
export type Closed = UsdSignedFigure1 | Gap2;
export type Kind9 = "value";
export type Text21 = string;
export type Tone5 = "up" | "down" | "flat" | "none";
export type Unit10 = "usd_signed";
export type Value10 = number;
export type Label8 = string;
/**
 * Calendar month as YYYY-MM.
 */
export type Month = string;
export type Premium = UsdSignedFigure1 | Gap2;
export type PremiumSeriesCaption = string;
export type PremiumSeriesText = string;
export type Say4 = string;
export type Subtitle3 = string;
export type Caption4 = string;
export type Figure4 = UsdFigure1 | Gap2;
export type Kind10 = "value";
export type Text22 = string;
export type Unit11 = "usd";
export type Value11 = number;
export type Label9 = string;
export type Caption5 = string;
export type Figure5 = UsdSignedFigure1 | Gap2;
export type Label10 = string;
export type Caption6 = string;
export type Figure6 = CountFigure | Gap2;
export type Kind11 = "value";
export type Text23 = string;
export type Unit12 = "structures";
export type Value12 = number;
export type Label11 = string;
export type Title5 = string;
export type Currency2 = "USD";
/**
 * @maxItems 20
 */
export type DataTimes5 =
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
export type Schema5 = "options_summary";
export type SchemaVersion5 = 4;

/**
 * Envelope (ADR 0034), the home texts and links, and the five summary pages as members.
 */
export interface HomePage {
  schema: Schema;
  schema_version: SchemaVersion;
  run_id: Id;
  build_id: Id;
  scope: Id;
  generated_at: Timestamp;
  data_times: DataTimes;
  run_status: RunStatus;
  title: Title;
  say: Say;
  gap_line: GapLine;
  overview: Overview;
  report_links: ReportLinks;
  household: HouseholdSummary;
  companies: HeldCompaniesSummary;
  macro: MacroSummary;
  market: MarketSummaryV3;
  options: OptionsSummaryV3;
  stale_after: Timestamp;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: Timestamp1;
  source: Id1;
}
/**
 * The top card: one line per card and the real gaps.
 */
export interface Overview {
  gaps: Gaps;
  items: Items;
}
/**
 * A real gap of a card, by the card's title.
 */
export interface GapItem {
  card: Card;
  text: Text;
}
/**
 * A card summary line: ``label`` links to ``href``, ``text`` is the card's own words.
 */
export interface OverviewLine {
  href: Href;
  label: Label;
  text: Text1;
}
/**
 * A link the home page shows: the path it goes to and its words.
 */
export interface BlockLink {
  href: Href1;
  text: Text2;
}
/**
 * Envelope (ADR 0034) and card.
 */
export interface HouseholdSummary {
  build_id: Id2;
  card: Card1;
  currency: Currency;
  data_times: DataTimes1;
  generated_at: Timestamp2;
  run_id: Id2;
  run_status: RunStatus1;
  schema: Schema1;
  schema_version: SchemaVersion1;
  scope: Id2;
  stale_after?: Timestamp2;
}
export interface Card1 {
  data_times_text: DataTimesText;
  gaps: Gaps1;
  notes: Notes;
  stale: StaleLabel;
  subtitle: Subtitle;
  tiles: Tiles;
  title: Title1;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  reason: Reason;
  text: Text3;
}
/**
 * The times and the label; the browser only compares its clock with stale_after.
 */
export interface StaleLabel {
  data_time: DataTime;
  stale_after: StaleAfter;
  text: Text4;
}
/**
 * The four tiles, under fixed keys.
 */
export interface Tiles {
  cash: CashTile;
  day_change: DayChangeTile;
  month_to_date: MonthToDateTile;
  net_worth: NetWorthTile;
}
export interface CashTile {
  caption: Caption;
  explain?: Explain;
  figure: Figure;
  label: Label1;
}
/**
 * A signed US-dollar amount; ``text`` starts with ``+`` or ``−`` (U+2212).
 */
export interface UsdSignedFigure {
  text: Text5;
  tone?: Tone;
  unit: Unit;
  value: Value;
}
/**
 * A share in whole percent, 0 to 100.
 */
export interface SharePercentFigure {
  text: Text6;
  unit: Unit1;
  value: Value1;
}
/**
 * A plain small line under a figure.
 */
export interface Caption1 {
  text: Text7;
}
export interface DayChangeTile {
  caption: Caption2;
  explain?: Explain1;
  figure: Figure1;
  label: Label2;
}
/**
 * A fraction times 100, signed (``+2.1%``).
 */
export interface PercentFigure {
  text: Text8;
  tone?: Tone1;
  unit: Unit2;
  value: Value2;
}
export interface MonthToDateTile {
  caption: Caption3;
  explain?: Explain2;
  figure: Figure2;
  label: Label3;
}
export interface NetWorthTile {
  caption: Caption1;
  explain?: Explain3;
  figure: Figure3;
  label: Label4;
}
/**
 * A displayed US-dollar amount (two decimals in ``value``, whole dollars in ``text``).
 */
export interface UsdFigure {
  text: Text9;
  unit: Unit3;
  value: Value3;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime1 {
  data_time: DataTime1;
  source: Id3;
}
/**
 * Envelope (ADR 0034) and card. No envelope currency: the names carry their own.
 */
export interface HeldCompaniesSummary {
  build_id: Id;
  card: Card2;
  data_times: DataTimes2;
  generated_at: Timestamp;
  run_id: Id;
  run_status: RunStatus;
  schema: Schema2;
  schema_version: SchemaVersion2;
  scope: Id;
  stale_after: Timestamp;
}
export interface Card2 {
  columns: Columns;
  data_times_text: DataTimesText1;
  gaps: Gaps2;
  notes: Notes1;
  rows: Rows;
  say: Say1;
  sharp_note: SharpNote;
  stale: StaleLabel1;
  subtitle: Subtitle1;
  title: Title2;
}
export interface Column {
  id: Id4;
  label: Label5;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap1 {
  kind: Kind;
  reason: Reason1;
  text: Text10;
}
export interface Row {
  currency: Currency1;
  day_change: DayChange;
  day_sharp: DaySharp;
  earnings: Earnings;
  five_day_change: FiveDayChange;
  five_day_sharp: FiveDaySharp;
  price: Price;
  symbol: Symbol;
  volatility: VolatilityLine;
}
/**
 * A fraction times 100, signed (``+2.1%``).
 */
export interface PercentFigure1 {
  kind: Kind1;
  text: Text11;
  tone?: Tone2;
  unit: Unit4;
  value: Value4;
}
/**
 * A sharp move mark: ``text`` is the label shown beside the change.
 */
export interface SharpMark {
  kind: Kind2;
  text: Text12;
}
/**
 * No mark: the move is not sharp.
 */
export interface NoMark {
  kind: Kind3;
}
/**
 * A results date: ``date`` is the ISO day, ``text`` what is shown (``10-28`` or ``已公布 10-20``).
 */
export interface EarningsCell {
  date: Date;
  kind: Kind4;
  text: Text13;
}
/**
 * A quote price: ``value`` is the number, ``text`` what is shown.
 */
export interface PriceFigure {
  kind: Kind5;
  text: Text14;
  unit: Unit5;
  value: Value5;
}
/**
 * The tool's own HV30, IV30 and IV percentile of a name (ADR 0033 change 9). ``source`` is the short
 * label ("own"), ``label`` names the conventions and the data day; the figures are percents without a
 * sign or tone, a part that is not an ``ok`` reading of the day is a ``Gap``, and one of an earlier day
 * carries that day in its ``text``. ``reconciliation`` is the status the readings carry against IBKR's
 * series; no IBKR value is shown.
 */
export interface VolatilityLine {
  data_day: DataDay;
  hv30: Hv30;
  iv30: Iv30;
  ivp: Ivp;
  label: Label6;
  reconciliation: Reconciliation;
  source: Source;
}
/**
 * The times and the label. The producer sets is_stale; the browser compares no clock.
 */
export interface StaleLabel1 {
  data_time: Timestamp3;
  is_stale: IsStale;
  text: Text15;
}
/**
 * Envelope (ADR 0034) and card.
 */
export interface MacroSummary {
  build_id: Id2;
  card: Card3;
  data_times: DataTimes3;
  generated_at: Timestamp2;
  run_id: Id2;
  run_status: RunStatus2;
  schema: Schema3;
  schema_version: SchemaVersion3;
  scope: Id2;
  stale_after?: Timestamp2;
}
export interface Card3 {
  data_times_text: DataTimesText2;
  ease: Ease;
  events: Events;
  gaps: Gaps3;
  notes: Notes2;
  say: Say2;
  stale: StaleLabel;
  title: Title3;
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
  text: Text16;
  tone?: Tone3;
  unit: Unit6;
  value: Value6;
}
/**
 * A marker position on its track, 0 to 100.
 */
export interface PercentOfTrackFigure {
  text: Text17;
  unit: Unit7;
  value: Value7;
}
/**
 * The ends of the neutral band on the track, ``percent_of_track``.
 */
export interface NeutralBand {
  high: PercentOfTrackFigure;
  low: PercentOfTrackFigure;
}
export interface Event {
  date: Date1;
  name_text: NameText;
  previous: Previous;
  when_text: WhenText;
}
/**
 * Envelope (ADR 0034) and card of the v3 line: tagged unions, a stale flag set by the producer.
 */
export interface MarketSummaryV3 {
  build_id: Id;
  card: Card4;
  data_times: DataTimes4;
  generated_at: Timestamp;
  run_id: Id;
  run_status: RunStatus;
  schema: Schema4;
  schema_version: SchemaVersion4;
  scope: Id;
  stale_after: Timestamp;
}
export interface Card4 {
  chart: Chart;
  data_times_text: DataTimesText3;
  gaps: Gaps4;
  index_table: IndexTable;
  notes: Notes3;
  say: Say3;
  stale: StaleLabel1;
  subtitle: Subtitle2;
  title: Title4;
  vol_indices?: VolIndices;
  vol_indices_note?: VolIndicesNote;
}
export interface Chart {
  aria_text: AriaText1;
  end_labels: EndLabels;
  points: Points;
  series_names: SeriesNames;
}
/**
 * One day. A level of a missing day is left out.
 */
export interface ChartPoint {
  band_high?: BandHigh;
  band_low?: BandLow;
  close?: Close;
  label: Label7;
  ma200?: Ma200;
}
export interface IndexTable {
  columns: Columns1;
  rows: Rows1;
}
export interface IndexRow {
  iv: Iv;
  iv_minus_rv20: IvMinusRv20;
  name: Name;
  rv20: Rv20;
  source_text?: SourceText;
}
/**
 * Vol points (GLOSSARY "Vol point"), signed (``+2.1``).
 */
export interface VolPointsFigure {
  kind: Kind6;
  source?: Source1;
  text: Text18;
  tone?: Tone4;
  unit: Unit8;
  value: Value8;
}
/**
 * One Cboe volatility index (VIX, VXN, RVX): its level in index points as Cboe publishes it
 * (18.20 is 18.20 points, not a fraction and not a percent), the data day ``mm-dd`` and the
 * source series. Never part of the IV cell of the index table: a Cboe index is a methodology of
 * its own.
 */
export interface VolIndexItem {
  day: Day;
  level: Level;
  name: Name1;
  source_text?: SourceText1;
}
/**
 * The level of a published market index in index points as its publisher states it (a Cboe
 * volatility index: 18.20 is 18.20 points). Not the 0 to 100 ease scale of ``IndexPointsFigure``:
 * the level has no upper bound in practice, the cap here only rejects a corrupt value.
 */
export interface IndexLevelFigure {
  kind: Kind7;
  text: Text19;
  unit: Unit9;
  value: Value9;
}
/**
 * Envelope (ADR 0034) and card of the v3 line: tagged unions, a stale flag set by the producer.
 */
export interface OptionsSummaryV3 {
  build_id: Id;
  card: Card5;
  currency: Currency2;
  data_times: DataTimes5;
  generated_at: Timestamp;
  run_id: Id;
  run_status: RunStatus;
  schema: Schema5;
  schema_version: SchemaVersion5;
  scope: Id;
  stale_after: Timestamp;
}
export interface Card5 {
  closed_series_text: ClosedSeriesText;
  gaps: Gaps5;
  notes: Notes4;
  premium_series_6m: PremiumSeries6M;
  premium_series_caption: PremiumSeriesCaption;
  premium_series_text: PremiumSeriesText;
  say?: Say4;
  stale: StaleLabel1;
  subtitle: Subtitle3;
  tiles: Tiles1;
  title: Title5;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap2 {
  kind: Kind8;
  reason: Reason2;
  text: Text20;
}
export interface SeriesPoint {
  closed: Closed;
  label: Label8;
  month: Month;
  premium: Premium;
}
/**
 * A signed US-dollar amount; ``text`` starts with ``+`` or ``−`` (U+2212).
 */
export interface UsdSignedFigure1 {
  kind: Kind9;
  text: Text21;
  tone?: Tone5;
  unit: Unit10;
  value: Value10;
}
/**
 * The four tiles, under fixed keys.
 */
export interface Tiles1 {
  max_loss_usd: UsdTile;
  open_premium_usd: UsdTile;
  premium_month_usd: SignedUsdTile;
  sold_count_month: CountTile;
}
export interface UsdTile {
  caption?: Caption4;
  figure: Figure4;
  label: Label9;
}
/**
 * A displayed US-dollar amount (two decimals in ``value``, whole dollars in ``text``).
 */
export interface UsdFigure1 {
  kind: Kind10;
  text: Text22;
  unit: Unit11;
  value: Value11;
}
export interface SignedUsdTile {
  caption?: Caption5;
  figure: Figure5;
  label: Label10;
}
export interface CountTile {
  caption?: Caption6;
  figure: Figure6;
  label: Label11;
}
/**
 * A displayed count of option structures (a short put, a spread: one structure each).
 */
export interface CountFigure {
  kind: Kind11;
  text: Text23;
  unit: Unit12;
  value: Value12;
}
