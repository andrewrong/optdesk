// The held_companies_summary card reads schema version 3 only; its payload type is the generated one.
import type { HeldCompaniesSummary } from '@/generated/held_companies_summary.v3'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [3] as const
export type Payload = HeldCompaniesSummary
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
