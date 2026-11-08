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
  title: 'ok 市场页标题',
  sentence: 'ok 市场页一句话',
  notes: ['ok 市场页备注'],
  gaps: [],
  weather: {
    headline: 'ok 天气标题',
    sentence: 'ok 天气一句话',
    chart: { aria_text: 'ok 天气图说明', series_names: ['ok SPX', 'ok 200日线'], points },
  },
  indices: {
    title: 'ok 指数标题',
    sentence: 'ok 指数一句话',
    columns: ['ok 指数', 'ok IV', 'ok RV20', 'ok IV减RV20', 'ok 三年分位'],
    rows: [
      { name: 'ok SPX', iv: fig('ok SPX iv 18.2', 18.2), rv20: fig('ok SPX rv 14.1', 14.1), iv_minus_rv20: fig('ok SPX diff 4.1', 4.1), rank_3y: fig('ok SPX rank 61', 61) },
      { name: 'ok NDX', iv: fig('ok NDX iv 21.0', 21), rv20: fig('ok NDX rv 17.5', 17.5), iv_minus_rv20: fig('ok NDX diff 3.5', 3.5), rank_3y: fig('ok NDX rank 55', 55) },
      { name: 'ok N225', iv: fig('ok N225 iv 20.3', 20.3), rv20: fig('ok N225 rv 16.0', 16), iv_minus_rv20: fig('ok N225 diff 4.3', 4.3), rank_3y: fig('ok N225 rank 48', 48) },
    ],
    note: 'ok 指数备注',
  },
  term: {
    title: 'ok 期限结构标题',
    sentence: 'ok 期限结构一句话',
    points: [
      { label: 'ok 1M', today: fig('ok 1M today 16.5', 16.5), five_days_ago: fig('ok 1M before 17.1', 17.1) },
      { label: 'ok 3M', today: fig('ok 3M today 18.0', 18), five_days_ago: fig('ok 3M before 18.4', 18.4) },
    ],
  },
  breadth: {
    title: 'ok 宽度标题',
    rows: [
      { label: 'ok 站上50日线', figure: fig('ok 50d 63%', 63) },
      { label: 'ok 站上200日线', figure: fig('ok 200d 58%', 58) },
    ],
  },
} satisfies MarketPage
