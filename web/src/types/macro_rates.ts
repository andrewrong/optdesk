import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'
import type { MacroRates } from '@/generated/macro_rates.v2'

export type { Gap }
export const SUPPORTED_VERSIONS = [2] as const
export type Payload = MacroRates
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
