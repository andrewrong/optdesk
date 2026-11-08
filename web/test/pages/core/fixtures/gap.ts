// Synthetic core page payload with a missing figure and its cause.
import type { CorePage } from '../../../../src/generated/core_page.v1'
import { ok } from './ok'

export const gap = {
  ...ok,
  run_status: 'partial',
  gaps: [{ kind: 'gap', reason: 'no_data', text: '估值数据不足：CCC 缺少三年财报' }],
} satisfies CorePage
