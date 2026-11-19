// The route-off payload: view form, form null, the notice form_off.
import type { Payload } from '@/types/strategy_replay_page'
import { report } from './report'

export const off = (): Payload => ({
  ...report(),
  view: 'form',
  form: null,
  report: null,
  notices: [{ key: 'form_off', text: '回放未开启：缺少所有者令牌设置，设置后重启 ot serve' }],
})
