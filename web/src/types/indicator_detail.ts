// The indicator_detail page reads schema version 1 only; its payload type is the generated one.
import type { IndicatorDetail } from '@/generated/indicator_detail.v1'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [1] as const
export type Payload = IndicatorDetail
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
