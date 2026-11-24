// The spread legs page type module: the payload type is the generated one, no field is declared here.
// TEMPORARY validator: it checks the document is an object of a supported version, as the other placeholders do.
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'
import type { SpreadLegsPage } from '@/generated/spread_legs_page.v1'

export type { Gap }
export const SUPPORTED_VERSIONS = [1] as const
export type Payload = SpreadLegsPage
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
