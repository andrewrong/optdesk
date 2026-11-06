// The options_summary page, schema v2: the payload type is generated (`@/generated`), the validator is the
// placeholder check until the Ajv validators land (docs/design/web-v2-types.md).
import type { OptionsSummary } from '@/generated'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [2] as const
export type Payload = OptionsSummary
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
