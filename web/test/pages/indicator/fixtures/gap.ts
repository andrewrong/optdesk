// Synthetic fixture written from contracts/schemas/page.indicator_detail.v2.json alone.
import type { IndicatorDetail } from '../../../../src/generated/indicator_detail.v2'

export const gap = {
  schema: 'indicator_detail',
  schema_version: 2,
  run_id: 'run-1',
  build_id: 'build-1',
  scope: 'macro',
  generated_at: '2026-11-06T20:00:00+08:00',
  data_times: [{ source: 'fred', data_time: '2026-11-06T19:00:00+08:00' }],
  run_status: 'partial',
  slug: 'unemployment',
  title: '失业率详情',
  sentence: '失业率小幅上升，为今年最高。',
  latest: { reason: 'no_data', text: '这个指标没有存储的数据点。' },
  revision: {
    previous: { value: 4.1, text: '上一版本 4.1%', unit: 'series_value' },
    vintage: { day: '2026-10-01', text: '10 月 1 日版本' },
    step: { value: 0.1, text: '上修 0.1 个百分点', unit: 'series_change' },
    text: '10 月版本后由 4.1% 上修。',
  },
  reference_line: { label: '长期均值 5.0%', value: { value: 5, text: '5.0%', unit: 'series_value' } },
  series_title: '失业率，%',
  explanation_title: '怎么读这个指标',
  meta_line: '来源 FRED · 数据截至 2026-10-01 · 及时',
  recent: { reason: 'no_data', text: '最近 12 期暂无数据' },
  points: [
    { observed_on: '2026-08-01', value: 4.0, text: '4.0%', vintage_on: '2026-09-01', released: { day: '2026-09-05', text: '9 月 5 日公布' } },
    { observed_on: '2026-09-01', value: 4.1, text: '4.1%', vintage_on: '2026-10-01', released: { day: '2026-10-03', text: '10 月 3 日公布' } },
    { observed_on: '2026-10-01', value: 4.2, text: '4.2%', vintage_on: '2026-11-01', released: { reason: 'no_data', text: '公布日未记录' } },
  ],
  explanation: ['失业率统计正在找工作的人。', '上升 0.1 个百分点属于正常波动。'],
  gaps: [{ reason: 'no_reader', text: '这个指标还没有读取器。' }],
  stale: { data_time: '2026-11-06T19:00:00+08:00', stale_after: '2026-11-08T20:00:00+08:00', text: '11 月 8 日前有效' },
} satisfies IndicatorDetail
