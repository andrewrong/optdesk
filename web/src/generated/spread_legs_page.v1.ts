// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Id = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp1 = string;
export type Id1 = string;
export type RunStatus = "ok" | "partial" | "failed";
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp2 = string;
export type IsStale = boolean;
export type Text = string;
export type Text1 = string;
/**
 * A figure as a number with its text, or a gap with its reason in words. Never a zero for a missing figure.
 */
export type Cell = {
  kind: "value" | "gap";
  text: Text1;
  value?: number;
  unit?: Text1;
  reason?: string;
} & {
  kind: "value" | "gap";
  text: Text1;
  value?: number;
  unit?: Text1;
  reason?: string;
} & {
  kind: "value" | "gap";
  text: Text1;
  value?: number;
  unit?: Text1;
  reason?: string;
};
/**
 * Every break-even price, ascending, or a gap with its reason in words. ``values`` is empty for a gap.
 */
export type BreakEven = {
  kind: "values" | "gap";
  text: Text1;
  /**
   * @maxItems 8
   */
  values:
    | []
    | [number]
    | [number, number]
    | [number, number, number]
    | [number, number, number, number]
    | [number, number, number, number, number]
    | [number, number, number, number, number, number]
    | [number, number, number, number, number, number, number]
    | [number, number, number, number, number, number, number, number];
  reason?: string;
};

/**
 * The spreads the account holds, each as one strategy with its legs, net credit, max loss, max gain and break-evens, then the units the matcher could not name. A render of text a producer wrote; nothing is computed here.
 */
export interface SpreadLegsPage {
  schema: "spread_legs_page";
  schema_version: 1;
  run_id: Id;
  build_id: Id;
  scope: Id;
  generated_at: Timestamp;
  /**
   * @maxItems 20
   */
  data_times:
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
  run_status: RunStatus;
  stale: StaleLabel;
  title: Text1;
  sentence: Text1;
  note: Text1 | null;
  /**
   * @minItems 2
   * @maxItems 2
   */
  groups: [Group, Group];
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: Timestamp1;
  source: Id1;
}
/**
 * The times and the label. The producer sets is_stale; the browser compares no clock.
 */
export interface StaleLabel {
  data_time: Timestamp2;
  is_stale: IsStale;
  text: Text;
}
/**
 * One card of the page: its title and its structures in order (the spreads first, the unidentified units second).
 */
export interface Group {
  title: Text1;
  /**
   * @maxItems 200
   */
  structures: Structure[];
}
/**
 * One structure with its legs. ``href`` is the detail route of the same structure. ``reason`` is the line of an unidentified unit, null otherwise.
 */
export interface Structure {
  symbol: Text1;
  href: string;
  kind_text: Text1;
  kind_gloss: Text1;
  expiry_text: Text1;
  /**
   * @minItems 1
   * @maxItems 8
   */
  legs:
    | [Leg]
    | [Leg, Leg]
    | [Leg, Leg, Leg]
    | [Leg, Leg, Leg, Leg]
    | [Leg, Leg, Leg, Leg, Leg]
    | [Leg, Leg, Leg, Leg, Leg, Leg]
    | [Leg, Leg, Leg, Leg, Leg, Leg, Leg]
    | [Leg, Leg, Leg, Leg, Leg, Leg, Leg, Leg];
  net_credit: Cell;
  max_loss: Cell;
  max_gain: Cell;
  break_even: BreakEven;
  reason: Text1 | null;
}
/**
 * One leg as ready texts.
 */
export interface Leg {
  side_text: Text1;
  strike: Text1;
  expiry_text: Text1;
  quantity: Text1;
  open_price: Text1;
}
