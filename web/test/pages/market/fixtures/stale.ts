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
  stale: { data_time: '2026-10-23T08:30:00+08:00', is_stale: true, text: 'stale 数据时间标签' },
  title: 'stale 市场页标题',
  sentence: 'stale 市场页一句话',
  notes: ['stale 市场页备注'],
  gaps: [],
  weather: {
    headline: 'stale 天气标题',
    sentence: 'stale 天气一句话',
    chart: { aria_text: 'stale 天气图说明', series_names: ['stale SPX', 'stale 200日线'], points },
  },
  indices: {
    title: 'stale 指数标题',
    sentence: 'stale 指数一句话',
    columns: ['stale 指数', 'stale IV', 'stale RV20', 'stale IV减RV20', 'stale 三年分位'],
    rows: [
      { name: 'stale SPX', iv: fig('stale SPX iv 18.2', 18.2), rv20: fig('stale SPX rv 14.1', 14.1), iv_minus_rv20: fig('stale SPX diff 4.1', 4.1), rank_3y: fig('stale SPX rank 61', 61) },
      { name: 'stale NDX', iv: fig('stale NDX iv 21.0', 21), rv20: fig('stale NDX rv 17.5', 17.5), iv_minus_rv20: fig('stale NDX diff 3.5', 3.5), rank_3y: fig('stale NDX rank 55', 55) },
      { name: 'stale N225', iv: fig('stale N225 iv 20.3', 20.3), rv20: fig('stale N225 rv 16.0', 16), iv_minus_rv20: fig('stale N225 diff 4.3', 4.3), rank_3y: fig('stale N225 rank 48', 48) },
    ],
    note: 'stale 指数备注',
  },
  term: {
    title: 'stale 期限结构标题',
    sentence: 'stale 期限结构一句话',
    points: [
      { label: 'stale 1M', today: fig('stale 1M today 16.5', 16.5), five_days_ago: fig('stale 1M before 17.1', 17.1) },
      { label: 'stale 3M', today: fig('stale 3M today 18.0', 18), five_days_ago: fig('stale 3M before 18.4', 18.4) },
    ],
  },
  breadth: {
    title: 'stale 宽度标题',
    rows: [
      { label: 'stale 站上50日线', figure: fig('stale 50d 63%', 63) },
      { label: 'stale 站上200日线', figure: fig('stale 200d 58%', 58) },
    ],
  },
} satisfies MarketPage
