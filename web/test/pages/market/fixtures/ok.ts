import type { FigureOrGap, MarketPage } from '../../../../src/generated/market_page.v1'

const fig = (text: string, value: number): FigureOrGap => ({ kind: 'value', value, text, unit: 'pct' })

// 120 sessions of the close and its 200-day line, as the schema's points.
const points = Array.from({ length: 120 }, (_, i) => ({
  label: `S${String(i + 1).padStart(3, '0')}`,
  close: 5000 + i,
  ma200: 4900 + i / 2,
}))

// Synthetic market_page fixture written from contracts/schemas/page.market_page.v1.json alone.
export const ok = {
  schema: 'market_page',
  schema_version: 1,
  run_id: 'run-demo-0001',
  build_id: 'build-demo-0001',
  scope: 'demo',
  generated_at: '2026-10-23T09:00:00+08:00',
  data_times: [{ data_time: '2026-10-23T08:30:00+08:00', source: 'demo_market' }],
  run_status: 'ok',
  stale: null,
  title: 'ok market page title',
  sentence: 'ok market page sentence',
  notes: ['ok market page note'],
  gaps: [],
  weather: {
    headline: 'ok weather title',
    sentence: 'ok weather sentence',
    chart: { aria_text: 'ok weather chart caption', series_names: ['ok SPX', 'ok 200-day line'], points },
  },
  indices: {
    title: 'ok index title',
    sentence: 'ok index sentence',
    columns: ['ok index', 'ok IV', 'ok RV20', 'ok IV minus RV20', 'ok 3y rank'],
    rows: [
      { name: 'ok SPX', iv: fig('ok SPX iv 18.2', 18.2), rv20: fig('ok SPX rv 14.1', 14.1), iv_minus_rv20: fig('ok SPX diff 4.1', 4.1), rank_3y: fig('ok SPX rank 61', 61) },
      { name: 'ok NDX', iv: fig('ok NDX iv 21.0', 21), rv20: fig('ok NDX rv 17.5', 17.5), iv_minus_rv20: fig('ok NDX diff 3.5', 3.5), rank_3y: fig('ok NDX rank 55', 55) },
      { name: 'ok N225', iv: fig('ok N225 iv 20.3', 20.3), rv20: fig('ok N225 rv 16.0', 16), iv_minus_rv20: fig('ok N225 diff 4.3', 4.3), rank_3y: fig('ok N225 rank 48', 48) },
    ],
    note: 'ok index note',
  },
  term: {
    title: 'ok term structure title',
    sentence: 'ok term structure sentence',
    points: [
      { label: 'ok 1M', today: fig('ok 1M today 16.5', 16.5), five_days_ago: fig('ok 1M before 17.1', 17.1) },
      { label: 'ok 3M', today: fig('ok 3M today 18.0', 18), five_days_ago: fig('ok 3M before 18.4', 18.4) },
    ],
  },
  breadth: {
    title: 'ok breadth title',
    rows: [
      { label: 'ok above 50-day line', figure: fig('ok 50d 63%', 63) },
      { label: 'ok above 200-day line', figure: fig('ok 200d 58%', 58) },
    ],
  },
} satisfies MarketPage
