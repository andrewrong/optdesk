import type { FigureOrGap, MarketPage } from '../../../../src/generated/market_page.v1'

const miss = (text: string): FigureOrGap => ({ kind: 'gap', reason: 'jp_slice_missing', text })
const fig = (text: string, value: number): FigureOrGap => ({ kind: 'value', value, text, unit: 'pct' })

// 120 sessions of the close and its 200-day line, as the schema's points.
const points = Array.from({ length: 120 }, (_, i) => ({
  label: `S${String(i + 1).padStart(3, '0')}`,
  close: 5000 + i,
  ma200: 4900 + i / 2,
}))

// Synthetic market_page fixture written from contracts/schemas/page.market_page.v1.json alone.
export const gap = {
  schema: 'market_page',
  schema_version: 1,
  run_id: 'run-demo-0001',
  build_id: 'build-demo-0001',
  scope: 'demo',
  generated_at: '2026-10-23T09:00:00+08:00',
  data_times: [{ data_time: '2026-10-23T08:30:00+08:00', source: 'demo_market' }],
  run_status: 'ok',
  stale: null,
  title: 'gap market page title',
  sentence: 'gap market page sentence',
  notes: ['gap market page note'],
  gaps: [],
  weather: {
    headline: 'gap weather title',
    sentence: 'gap weather sentence',
    chart: { aria_text: 'gap weather chart caption', series_names: ['gap SPX', 'gap 200-day line'], points },
  },
  indices: {
    title: 'gap index title',
    sentence: 'gap index sentence',
    columns: ['gap index', 'gap IV', 'gap RV20', 'gap IV minus RV20', 'gap 3y rank'],
    rows: [
      { name: 'gap SPX', iv: fig('gap SPX iv 18.2', 18.2), rv20: fig('gap SPX rv 14.1', 14.1), iv_minus_rv20: fig('gap SPX diff 4.1', 4.1), rank_3y: fig('gap SPX rank 61', 61) },
      { name: 'gap TOPX', iv: miss('gap TOPX missing iv: Japan data not connected'), rv20: miss('gap TOPX missing rv: Japan data not connected'), iv_minus_rv20: miss('gap TOPX missing difference: Japan data not connected'), rank_3y: miss('gap TOPX missing rank: Japan data not connected') },
      { name: 'gap N225', iv: miss('gap N225 missing iv: Japan data not connected'), rv20: miss('gap N225 missing rv: Japan data not connected'), iv_minus_rv20: miss('gap N225 missing difference: Japan data not connected'), rank_3y: miss('gap N225 missing rank: Japan data not connected') },
    ],
    note: 'gap index note',
  },
  term: { kind: 'gap', reason: 'no_data', text: 'gap term structure missing: no VIX term data' },
  breadth: {
    title: 'gap breadth title',
    rows: [
      { label: 'gap above 50-day line', figure: fig('gap 50d 63%', 63) },
      { label: 'gap above 200-day line', figure: fig('gap 200d 58%', 58) },
    ],
  },
} satisfies MarketPage
