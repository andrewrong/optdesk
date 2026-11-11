import type { SettingsPage } from '../../../../src/generated/settings_page.v2'

// Synthetic settings_page fixture written from contracts/schemas/page.settings_page.v2.json alone: editable, with a notice and history.
export const editable: SettingsPage = {
  schema: 'settings_page',
  schema_version: 2,
  run_id: 'run-demo-0001',
  build_id: 'build-demo-0001',
  scope: 'demo',
  as_of: '2026-11-10',
  generated_at: '2026-11-10T00:00:00Z',
  data_times: [],
  run_status: 'ok',
  title: 'demo Settings',
  form: { expires: '1790000000', mac: 'ab12cd34', post_path: '/owner-settings' },
  confirm: null,
  notice: { kind: 'saved', text: 'demo Saved 2 items' },
  params: [
    { key: 'min_dte', label: 'demo Shortest expiry', unit: 'days', value: '25', default: '21', min: '7', max: '120', kind: 'int' },
    { key: 'max_spread', label: 'demo Max spread', unit: 'USD', value: '', default: '0.10', min: '0.01', max: '0.30', kind: 'decimal' },
  ],
  rows: [
    { instrument_id: 9001, symbol: 'DEMOA', excluded: true },
    { instrument_id: 9002, symbol: 'DEMOB', excluded: false },
    { instrument_id: 9003, symbol: 'DEMOC', excluded: false },
  ],
  history: [
    { at: '2026-11-09 10:00', kind: 'param', name: 'Shortest expiry', key: 'min_dte', value: '30', instrument_id: null, excluded: null },
    { at: '2026-11-08 09:00', kind: 'exclusion', name: 'DEMOA', key: null, value: 'Excluded', instrument_id: 9001, excluded: true },
  ],
}
