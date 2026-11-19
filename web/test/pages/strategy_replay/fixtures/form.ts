// The form payload: the report fixture before any post (view form, no report).
import type { Payload } from '@/types/strategy_replay_page'
import { report } from './report'

export const form = (): Payload => ({ ...report(), view: 'form', report: null })
