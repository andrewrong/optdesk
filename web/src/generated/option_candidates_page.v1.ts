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
 * One kind of candidate. gap is non-null exactly when main and other are both null.
 */
export type Group = (
  | {
      main?: null;
      other?: null;
      gap?: Gap;
      [k: string]: unknown;
    }
  | ((
      | {
          main?: Table;
          [k: string]: unknown;
        }
      | {
          other?: Table;
          [k: string]: unknown;
        }
    ) & {
      gap?: null;
      [k: string]: unknown;
    })
) & {
  kind: "short_put" | "covered_call" | "held_call_guard";
  title: Text1;
  /**
   * Date text of the rows of this group, or null.
   */
  day: Text1 | null;
  /**
   * True when day is before the build day.
   */
  stale: boolean;
  counts: Counts;
  main: null | Table;
  other: null | Table;
  gap: Gap | null;
  /**
   * Currency of the rows of this group as three upper-case letters (JPY for the Japanese group); null or absent for the US groups. Additive and optional.
   */
  currency?: string | null;
  /**
   * One-line note of the group, or null. Additive and optional.
   */
  note?: Text1 | null;
  /**
   * Counts of the rows by limit, or null. Additive and optional.
   */
  limit_counts?:
    | {
        limit: Text1;
        count: number;
        text: Text1;
      }[]
    | null;
  /**
   * For each Japanese name with no candidate row, its outcome (an open string), its Chinese text and its counts; additive and optional.
   */
  name_outcomes?: {
    name: Text1;
    outcome: string;
    text: Text1;
    counts: Text1;
  }[];
} & {
  kind: "short_put" | "covered_call" | "held_call_guard";
  title: Text1;
  /**
   * Date text of the rows of this group, or null.
   */
  day: Text1 | null;
  /**
   * True when day is before the build day.
   */
  stale: boolean;
  counts: Counts;
  main: null | Table;
  other: null | Table;
  gap: Gap | null;
  /**
   * Currency of the rows of this group as three upper-case letters (JPY for the Japanese group); null or absent for the US groups. Additive and optional.
   */
  currency?: string | null;
  /**
   * One-line note of the group, or null. Additive and optional.
   */
  note?: Text1 | null;
  /**
   * Counts of the rows by limit, or null. Additive and optional.
   */
  limit_counts?:
    | {
        limit: Text1;
        count: number;
        text: Text1;
      }[]
    | null;
  /**
   * For each Japanese name with no candidate row, its outcome (an open string), its Chinese text and its counts; additive and optional.
   */
  name_outcomes?: {
    name: Text1;
    outcome: string;
    text: Text1;
    counts: Text1;
  }[];
} & {
  kind: "short_put" | "covered_call" | "held_call_guard";
  title: Text1;
  /**
   * Date text of the rows of this group, or null.
   */
  day: Text1 | null;
  /**
   * True when day is before the build day.
   */
  stale: boolean;
  counts: Counts;
  main: null | Table;
  other: null | Table;
  gap: Gap | null;
  /**
   * Currency of the rows of this group as three upper-case letters (JPY for the Japanese group); null or absent for the US groups. Additive and optional.
   */
  currency?: string | null;
  /**
   * One-line note of the group, or null. Additive and optional.
   */
  note?: Text1 | null;
  /**
   * Counts of the rows by limit, or null. Additive and optional.
   */
  limit_counts?:
    | {
        limit: Text1;
        count: number;
        text: Text1;
      }[]
    | null;
  /**
   * For each Japanese name with no candidate row, its outcome (an open string), its Chinese text and its counts; additive and optional.
   */
  name_outcomes?: {
    name: Text1;
    outcome: string;
    text: Text1;
    counts: Text1;
  }[];
} & {
  kind: "short_put" | "covered_call" | "held_call_guard";
  title: Text1;
  /**
   * Date text of the rows of this group, or null.
   */
  day: Text1 | null;
  /**
   * True when day is before the build day.
   */
  stale: boolean;
  counts: Counts;
  main: null | Table;
  other: null | Table;
  gap: Gap | null;
  /**
   * Currency of the rows of this group as three upper-case letters (JPY for the Japanese group); null or absent for the US groups. Additive and optional.
   */
  currency?: string | null;
  /**
   * One-line note of the group, or null. Additive and optional.
   */
  note?: Text1 | null;
  /**
   * Counts of the rows by limit, or null. Additive and optional.
   */
  limit_counts?:
    | {
        limit: Text1;
        count: number;
        text: Text1;
      }[]
    | null;
  /**
   * For each Japanese name with no candidate row, its outcome (an open string), its Chinese text and its counts; additive and optional.
   */
  name_outcomes?: {
    name: Text1;
    outcome: string;
    text: Text1;
    counts: Text1;
  }[];
};
export type Cell = ValueCell | Gap;

/**
 * The option candidates page: the stored short puts, covered calls and guard rows of the last run. A render of rows a producer writes; nothing is computed here.
 */
export interface OptionCandidatesPage {
  schema: "option_candidates_page";
  schema_version: 1;
  run_id: Id;
  build_id: Id;
  scope: Id;
  generated_at: Timestamp;
  data_times: SourceTime[];
  run_status: RunStatus;
  stale: StaleLabel | null;
  title: Text1;
  notes: Text1[];
  gaps: Gap[];
  /**
   * Date text of the option chain day, or null.
   */
  chain_day: Text1 | null;
  /**
   * Time the run was computed, text in the owner zone, or null.
   */
  computed_at: Text1 | null;
  rule_version: Text1 | null;
  estimate_note: string;
  /**
   * One to four groups in the order short_put, the yen short_put when present, covered_call, held_call_guard.
   *
   * @minItems 1
   * @maxItems 4
   */
  groups: [Group] | [Group, Group] | [Group, Group, Group] | [Group, Group, Group, Group];
  stale_after?: Timestamp;
  /**
   * The grouped short put list, or null. Additive and optional; every figure is text.
   */
  short_put_list?: ShortList | null;
  /**
   * The tier view of the stored short puts, or null. Additive and optional; every figure is text.
   */
  short_put_tiers?: TierView | null;
  /**
   * The yen tier view of the stored yen short puts, or null. Built from the JP group; additive and optional; every figure is text.
   */
  short_put_tiers_jpy?: TierView | null;
  /**
   * The yen grouped short put list, or null. Built from the JP group; additive and optional; every figure is text.
   */
  short_put_list_jpy?: ShortList | null;
  subtitle?: Text1;
  /**
   * How many names came from the open chain, from the last close, and with no candidate; additive and optional. none is left out when the stored candidates cannot give it.
   */
  source_counts?: {
    open: number;
    eod_fallback: number;
    none?: number;
  };
  /**
   * The words of the page head, in Chinese; additive and optional.
   */
  head_labels?: {
    data_day: Text1;
    run_status: Text1;
    run_status_text: Text1;
    rule_version: Text1;
  } | null;
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
 * A named data gap, used in place of a value that cannot be shown. Never a zero. The reasons of common.v2 plus no_rows (a group without rows in its lookback), no_run (no stored run) and unreadable (a stored value that cannot be shown).
 */
export interface Gap {
  kind: "gap";
  reason:
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
    | "stale_input"
    | "no_rows"
    | "no_run"
    | "unreadable";
  text: Text1;
}
export interface Table {
  columns: Column[];
  rows: Row[];
}
export interface Column {
  id: Id;
  label: Text1;
  align: "left" | "right";
  hint: Text1 | null;
}
export interface Row {
  id: string;
  /**
   * Column id to Cell.
   */
  cells: {
    [k: string]: Cell;
  };
  /**
   * Only on other rows; empty on main rows.
   */
  reasons: Reason[];
}
export interface ValueCell {
  text: string;
  value: number | null;
  badges: Text1[];
  tone: "up" | "down" | "flat" | "none";
}
/**
 * Why a row is blocked or unknown. gate and code are patterns, not enums, so a new gate needs no schema change.
 */
export interface Reason {
  gate: string;
  status: "block" | "unknown";
  code: string;
  text: Text1;
}
/**
 * The stored row counts of the group.
 */
export interface Counts {
  ok: number;
  blocked: number;
  unknown: number;
}
/**
 * The grouped short put list, or null. Additive and optional; every figure is text.
 */
export interface ShortList {
  run_day: Text1;
  counts: Text1;
  caveat: Text1 | null;
  filters: Filter[];
  dropped: Text1 | null;
  dropped_items: DroppedItem[];
  columns: TierColumn[];
  groups: ShortGroup[];
  folded_title: Text1;
}
export interface Filter {
  id: Text1;
  label: Text1;
  value: Text1;
  hint: Text1;
}
export interface DroppedItem {
  code: Text1;
  label: Text1;
  count: Text1;
  hint: Text1;
}
export interface TierColumn {
  id: Text1;
  label: Text1;
  hint: Text1 | null;
}
export interface ShortGroup {
  ticker: Text1;
  spot: Text1;
  summary: Text1;
  rows: ShortRow[];
}
export interface ShortRow {
  key: Text1;
  strike: Pair;
  expiry: Pair;
  otm: Pair;
  delta: Pair;
  premium: Pair;
  annualized: Pair;
  buy: Pair;
  buy_source: Badge | null;
  buy_link: Text1 | null;
  badges: Badge[];
}
/**
 * A Pair with a gap has the gap text in main and sub null.
 */
export interface Pair {
  main: Text1;
  sub: Text1 | null;
  gap: Gap | null;
}
export interface Badge {
  code: Text1;
  label: Text1;
  gloss: Text1;
  tone: "warn" | "info" | "none";
}
/**
 * The short puts sorted into tiers, from docs/design/candidates-tiers.md#Interface.
 */
export interface TierView {
  chain_day: Text1;
  companies: Company[];
  common: Mark[];
  kept: Text1 | null;
  kept_note?: Text1 | null;
  stored_title?: Text1 | null;
  idle_note?: Text1 | null;
  buy_hint?: Text1 | null;
  spot_hint?: Text1 | null;
  legend: LegendItem[];
  columns: TierColumn[];
  tiers: Tier[];
  strip_labels?: {
    spot: Text1;
    buy_price: Text1;
    next_earnings: Text1;
    contracts: Text1;
    fits: Text1;
    check: Text1;
    grey: Text1;
    grey_hint: Text1;
  };
  card_note?: Text1 | null;
  heads?: {
    companies: Text1;
    companies_note: Text1;
    tiers: Text1;
  };
  by_company?: ByCompany;
}
export interface Company {
  key: Text1;
  ticker: Text1;
  spot: Text1;
  spot_day: Text1;
  buy_price: Text1 | null;
  buy_price_note: Text1 | null;
  next_earnings: Text1;
  counts: Text1;
  valuation?: ValueLine | null;
  buy_source?: Badge | null;
  tier?: {
    id: "fits" | "check" | "grey";
    word: Text1;
    glyph: Text1;
  };
  count_parts?: {
    fits: Text1;
    check: Text1;
    grey: Text1;
  };
  spot_text?: Text1 | null;
  spot_note?: Text1 | null;
  price_note?: Text1 | null;
  earnings_text?: Text1;
  source_badge?: Badge | null;
}
/**
 * The valuation line and the gate buy price of a strip, from docs/design/candidates-valuation-line.md#Interface.
 */
export interface ValueLine {
  label: Text1;
  text: Text1;
  hint: Text1 | null;
  reason:
    | ("no_profile" | "ambiguous" | "removed" | "no_review" | "no_tables" | "currency" | "bad_input" | "read_failed")
    | null;
  gate_label: Text1;
  gate_text: Text1;
  gate_hint: Text1 | null;
  line_label?: Text1;
  line_text?: Text1;
  run_state?: "used" | "not_used" | "none";
  note?: Text1 | null;
}
/**
 * An info mark said once above the table.
 */
export interface Mark {
  code: Text1;
  label: Text1;
  gloss: Text1;
}
/**
 * One line of the legend; text is Chinese with the current values.
 */
export interface LegendItem {
  tier: Text1;
  title: Text1;
  text: Text1;
}
export interface Tier {
  id:
    | "fits"
    | "check"
    | "grey_no_buy_price"
    | "grey_no_spot"
    | "grey_in_the_money"
    | "grey_no_bid"
    | "grey_above_buy_price"
    | "grey_bad_input";
  word: Text1;
  icon: "check" | "alert" | "minus";
  title: Text1;
  explain: Text1;
  collapsed: boolean;
  rows: TierRow[];
  /**
   * Optional: one entry per company of the tier, in the order of its first row, with its row count and row ids.
   */
  companies?: {
    company: Text1;
    count: number;
    row_ids: string[];
  }[];
}
/**
 * line is the one-line text of a grey row.
 */
export interface TierRow {
  id: Text1;
  company: Text1;
  contract: Contract;
  margin: Pair;
  below_spot: Pair;
  premium: Pair;
  badges: Badge[];
  notes: Badge[];
  fields: Field[];
  line: Text1 | null;
  tier?: {
    id: "fits" | "check";
    word: Text1;
    glyph: Text1;
  };
}
export interface Contract {
  name: Text1;
  strike: Text1;
  expiry: Text1;
  dte: Text1;
}
export interface Field {
  id: Text1;
  label: Text1;
  hint: Text1 | null;
  value: Text1 | null;
  gap: Gap | null;
  badges: Badge[];
}
/**
 * The company-block layout of the tier view; absent means the flat tier bands. Blocks are in the order to draw.
 */
export interface ByCompany {
  heading_note: Text1;
  legend: LegendItem[];
  columns: TierColumn[];
  blocks: CompanyBlock[];
}
/**
 * One company; row_ids are TierRow.id values of rows that stay in tiers[].rows.
 */
export interface CompanyBlock {
  company: Text1;
  verdict: Verdict;
  verdict_text: Text1;
  best_yield: Text1 | null;
  row_ids: Text1[];
  folded: Folded | null;
}
export interface Verdict {
  id: "sellable" | "out_band" | "above_buy" | "no_buy_price" | "no_quote" | "blocked";
  word: Text1;
  glyph: Text1;
  tone: "success" | "warn" | "muted";
}
export interface Folded {
  title: Text1;
  summary: Text1;
  reasons: {
    code: Text1;
    label: Text1;
    count: Text1;
  }[];
  row_ids: Text1[];
}
