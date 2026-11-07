// The held_companies_summary card reads schema version 2 only; its payload type is the generated one.
import type { HeldCompaniesSummary } from '@/generated/held_companies_summary.v2'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [2] as const
export type Payload = HeldCompaniesSummary
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
