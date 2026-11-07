// The household_risk page reads schema version 1 only; its payload type is the generated one.
import type { HouseholdRisk } from '@/generated/household_risk.v1'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [1] as const
export type Payload = HouseholdRisk
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
