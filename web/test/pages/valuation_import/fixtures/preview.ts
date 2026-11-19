// The preview payload: the invented fixture of web/fixtures/valuation-import-page/preview.json.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import type { Payload } from '@/types/valuation_import_page'

export const preview = (): Payload =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../../../fixtures/valuation-import-page/preview.json'), 'utf8'),
  )

export const empty = (): Payload => ({
  ...preview(),
  view: 'empty',
  file: null,
  r: null,
  margin: null,
  rows: [],
  not_in_file: [],
  unvalued: [],
  removed: [],
  counts: { insert: 0, same: 0, older: 0, refused: 0, needs_tick: 0 },
  flagged_note: null,
})
