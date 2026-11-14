// The page type module of market_state_page, in the form of companies_page.ts: the payload type is the generated one.
// TEMPORARY validator: it checks the document is an object of a supported version, as the other placeholders do.
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'
import type { MarketStatePage as PayloadV1 } from '@/generated/market_state_page.v1'

export type { Gap }
export const SUPPORTED_VERSIONS = [1] as const
export type Payload = PayloadV1
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
