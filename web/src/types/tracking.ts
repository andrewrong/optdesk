// The tracking page reads schema version 2 (grouped by rule version); its payload type is the generated one.
import type { Tracking } from '@/generated/tracking.v2'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [2] as const
export type Payload = Tracking
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
