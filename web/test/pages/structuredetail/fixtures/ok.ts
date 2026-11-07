// Synthetic fixture written from page.structure_detail.v3.json alone.
import type { StructureDetail } from '../../../../src/generated/structure_detail.v3'

export const ok = {
  schema: 'structure_detail',
  schema_version: 3,
  build_id: 'synthetic-build-1',
  run_id: 'synthetic-run-1',
  run_status: 'ok',
  generated_at: '2030-01-02T10:00:00Z',
  data_times: [{ data_time: '2030-01-02T09:55:00Z', source: 'synthetic quotes' }],
  scope: 'synthetic-scope',
  title: 'Synthetic bull call spread',
  sentence: 'Sentence: the spread gains if the underlying ends above the long strike.',
  subline: 'Subline: two legs, one expiry, opened on a synthetic day.',
  stale: {
    data_time: '2030-01-02T09:55:00Z',
    stale_after: '2030-01-02T10:30:00Z',
    text: 'Stale label: quotes are from the synthetic morning run.',
  },
  tiles: {
    pnl: { label: 'Tile label pnl', caption: 'Tile caption pnl', figure: { text: 'Tile figure pnl', unit: 'usd', value: 120 } },
    max_gain: { label: 'Tile label gain', caption: 'Tile caption gain', figure: { text: 'Tile figure gain', unit: 'usd', value: 500 } },
    max_loss: { label: 'Tile label loss', caption: 'Tile caption loss', figure: { text: 'Tile figure loss', unit: 'usd', value: -300 } },
    break_even: { label: 'Tile label even', caption: 'Tile caption even', figure: { text: 'Tile figure even', unit: 'usd', value: 103 } },
  },
  legs_title: 'Legs title',
  legs: [
    {
      label: 'Leg label long',
      side_text: 'Leg side buy',
      right_text: 'Leg right call',
      expiry_text: 'Leg expiry one',
      quantity: 1,
      strike: { text: 'Leg strike long', unit: 'usd', value: 100 },
      price: { text: 'Leg price long', unit: 'usd', value: 5 },
      mark: { text: 'Leg mark long', unit: 'usd', value: 6 },
      pnl: { text: 'Leg pnl long', unit: 'usd', value: 100 },
    },
    {
      label: 'Leg label short',
      side_text: 'Leg side sell',
      right_text: 'Leg right call short',
      expiry_text: 'Leg expiry two',
      quantity: -1,
      strike: { text: 'Leg strike short', unit: 'usd', value: 106 },
      price: { text: 'Leg price short', unit: 'usd', value: 2 },
      mark: { text: 'Leg mark short', unit: 'usd', value: 1.9 },
      pnl: { text: 'Leg pnl short', unit: 'usd', value: 20 },
    },
  ],
  legs_total: { label: 'Total label', pnl: { text: 'Total pnl figure', unit: 'usd', value: 120 } },
  greeks_title: 'Greeks title',
  greeks: {
    model_text: 'Greeks model text',
    rows: [
      {
        label: 'Greek row long',
        cells: [
          { text: 'Greek delta long', unit: 'greek', value: 0.5 },
          { text: 'Greek gamma long', unit: 'greek', value: 0.02 },
          { text: 'Greek theta long', unit: 'greek', value: -0.04 },
          { text: 'Greek vega long', unit: 'greek', value: 0.11 },
        ],
      },
      {
        label: 'Greek row total',
        cells: [
          { text: 'Greek delta total', unit: 'greek', value: 0.2 },
          { text: 'Greek gamma total', unit: 'greek', value: 0.01 },
          { text: 'Greek theta total', unit: 'greek', value: -0.01 },
          { text: 'Greek vega total', unit: 'greek', value: 0.05 },
        ],
      },
    ],
  },
  fills_title: 'Fills title',
  fills: [
    {
      time_text: 'Fill time one',
      action_text: 'Fill action buy',
      leg_text: 'Fill leg long',
      quantity: 1,
      price: { text: 'Fill price one', unit: 'usd', value: 5 },
      fee: { text: 'Fill fee one', unit: 'usd', value: 1 },
    },
  ],
  payoff_title: 'Payoff title',
  payoff: {
    aria_text: 'Payoff aria text',
    price_unit: 'usd',
    pnl_unit: 'usd',
    points: [
      { price: 95, pnl: -300 },
      { price: 103, pnl: 0 },
      { price: 110, pnl: 500 },
    ],
    spot: { text: 'Payoff spot figure', unit: 'usd', value: 101 },
    break_even: { text: 'Payoff even figure', unit: 'usd', value: 103 },
    max_gain: { text: 'Payoff gain figure', unit: 'usd', value: 500 },
    max_loss: { text: 'Payoff loss figure', unit: 'usd', value: -300 },
    band: { low: 96, high: 106, text: 'Payoff band text' },
  },
  levels_title: 'Levels title',
  key_levels: [
    {
      name: 'Level name support',
      note: 'Level note support',
      price: { text: 'Level price support', unit: 'usd', value: 98 },
      distance: 'Level distance support',
    },
  ],
  timeline_title: 'Timeline title',
  timeline: [
    {
      name: 'Stage name open',
      date_text: 'Stage date open',
      days_text: 'Stage days open',
      elapsed: { text: 'Stage elapsed open', unit: 'share_percent', value: 10 },
    },
  ],
  gaps: [],
} satisfies StructureDetail
