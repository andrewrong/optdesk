import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'
import type { MacroRatesV3 } from '@/generated/macro_rates.v3'

export type { Gap }
export const SUPPORTED_VERSIONS = [3] as const
export type Payload = MacroRatesV3
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
