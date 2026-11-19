// The report payload: the invented fixture of web/fixtures/strategy-replay-page/report.json.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import type { Payload } from '@/types/strategy_replay_page'

export const report = (): Payload =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../../../fixtures/strategy-replay-page/report.json'), 'utf8'),
  )
