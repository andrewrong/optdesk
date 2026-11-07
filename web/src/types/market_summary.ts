// The market_summary card reads schema version 3 only; its payload type is the generated one.
import type { MarketSummaryV3 } from '@/generated/market_summary.v3'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [3] as const
export type Payload = MarketSummaryV3
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
