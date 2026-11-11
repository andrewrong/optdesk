import type { FigureOrGap, MarketPage } from '../../../../src/generated/market_page.v1'

const fig = (text: string, value: number): FigureOrGap => ({ kind: 'value', value, text, unit: 'pct' })

// 120 sessions of the close and its 200-day line, as the schema's points.
const points = Array.from({ length: 120 }, (_, i) => ({
  label: `S${String(i + 1).padStart(3, '0')}`,
  close: 5000 + i,
  ma200: 4900 + i / 2,
}))

// Synthetic market_page fixture written from contracts/schemas/page.market_page.v1.json alone.
export const stale = {
  schema: 'market_page',
  schema_version: 1,
  run_id: 'run-demo-0001',
  build_id: 'build-demo-0001',
  scope: 'demo',
  generated_at: '2026-10-23T09:00:00+08:00',
  data_times: [{ data_time: '2026-10-23T08:30:00+08:00', source: 'demo_market' }],
  run_status: 'ok',
  stale: { data_time: '2026-10-23T08:30:00+08:00', is_stale: true, text: 'stale data time label' },
  title: 'stale market page title',
  sentence: 'stale market page sentence',
  notes: ['stale market page note'],
  gaps: [],
  weather: {
    headline: 'stale weather title',
    sentence: 'stale weather sentence',
    chart: { aria_text: 'stale weather chart caption', series_names: ['stale SPX', 'stale 200-day line'], points },
  },
  indices: {
    title: 'stale index title',
    sentence: 'stale index sentence',
    columns: ['stale index', 'stale IV', 'stale RV20', 'stale IV minus RV20', 'stale 3y rank'],
    rows: [
      { name: 'stale SPX', iv: fig('stale SPX iv 18.2', 18.2), rv20: fig('stale SPX rv 14.1', 14.1), iv_minus_rv20: fig('stale SPX diff 4.1', 4.1), rank_3y: fig('stale SPX rank 61', 61) },
      { name: 'stale NDX', iv: fig('stale NDX iv 21.0', 21), rv20: fig('stale NDX rv 17.5', 17.5), iv_minus_rv20: fig('stale NDX diff 3.5', 3.5), rank_3y: fig('stale NDX rank 55', 55) },
      { name: 'stale N225', iv: fig('stale N225 iv 20.3', 20.3), rv20: fig('stale N225 rv 16.0', 16), iv_minus_rv20: fig('stale N225 diff 4.3', 4.3), rank_3y: fig('stale N225 rank 48', 48) },
    ],
    note: 'stale index note',
  },
  term: {
    title: 'stale term structure title',
    sentence: 'stale term structure sentence',
    points: [
      { label: 'stale 1M', today: fig('stale 1M today 16.5', 16.5), five_days_ago: fig('stale 1M before 17.1', 17.1) },
      { label: 'stale 3M', today: fig('stale 3M today 18.0', 18), five_days_ago: fig('stale 3M before 18.4', 18.4) },
    ],
  },
  breadth: {
    title: 'stale breadth title',
    rows: [
      { label: 'stale above 50-day line', figure: fig('stale 50d 63%', 63) },
      { label: 'stale above 200-day line', figure: fig('stale 200d 58%', 58) },
    ],
  },
} satisfies MarketPage
