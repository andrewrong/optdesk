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
  title: 'gap 市场页标题',
  sentence: 'gap 市场页一句话',
  notes: ['gap 市场页备注'],
  gaps: [],
  weather: {
    headline: 'gap 天气标题',
    sentence: 'gap 天气一句话',
    chart: { aria_text: 'gap 天气图说明', series_names: ['gap SPX', 'gap 200日线'], points },
  },
  indices: {
    title: 'gap 指数标题',
    sentence: 'gap 指数一句话',
    columns: ['gap 指数', 'gap IV', 'gap RV20', 'gap IV减RV20', 'gap 三年分位'],
    rows: [
      { name: 'gap SPX', iv: fig('gap SPX iv 18.2', 18.2), rv20: fig('gap SPX rv 14.1', 14.1), iv_minus_rv20: fig('gap SPX diff 4.1', 4.1), rank_3y: fig('gap SPX rank 61', 61) },
      { name: 'gap TOPX', iv: miss('gap TOPX 缺 iv：日本数据未接入'), rv20: miss('gap TOPX 缺 rv：日本数据未接入'), iv_minus_rv20: miss('gap TOPX 缺差值：日本数据未接入'), rank_3y: miss('gap TOPX 缺分位：日本数据未接入') },
      { name: 'gap N225', iv: miss('gap N225 缺 iv：日本数据未接入'), rv20: miss('gap N225 缺 rv：日本数据未接入'), iv_minus_rv20: miss('gap N225 缺差值：日本数据未接入'), rank_3y: miss('gap N225 缺分位：日本数据未接入') },
    ],
    note: 'gap 指数备注',
  },
  term: { kind: 'gap', reason: 'no_data', text: 'gap 期限结构缺：没有VIX期限数据' },
  breadth: {
    title: 'gap 宽度标题',
    rows: [
      { label: 'gap 站上50日线', figure: fig('gap 50d 63%', 63) },
      { label: 'gap 站上200日线', figure: fig('gap 200d 58%', 58) },
    ],
  },
} satisfies MarketPage
