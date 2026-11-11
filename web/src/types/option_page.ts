// TEMPORARY hand-written placeholder (not generated; web/src/generated stays the stub). Types copied from the old repository until the week1-schemas issue generates them.
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'
import type { OptionPage as PayloadV1 } from './option_page.v1.types.ts'
import type { OptionPage as PayloadV2 } from '@/generated/option_page.v2'

export type { Gap }
export const SUPPORTED_VERSIONS = [1, 2] as const
export type Payload = PayloadV1 | PayloadV2
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
