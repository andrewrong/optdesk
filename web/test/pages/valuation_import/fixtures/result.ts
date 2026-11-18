// The result payload of a confirm: the preview fixture with the outcomes of each row (invented).
import type { Payload } from '@/types/valuation_import_page'
import { preview } from './preview'

export const result = (): Payload => ({
  ...preview(),
  view: 'result',
  result: [
    { key: 'US:ZZA', outcome: 'inserted', text: 'written' },
    { key: 'JP:9901', outcome: 'same', text: '' },
    { key: 'HK:0999', outcome: 'not_confirmed', text: 'not confirmed' },
    { key: 'US:ZZC', outcome: 'failed', text: 'the write failed' },
  ],
})
