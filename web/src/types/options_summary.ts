// The options_summary page, schema v3: the payload type is generated (`@/generated/options_summary.v3`), the
// validator is the placeholder check until the Ajv validators land (docs/design/web-v2-types.md).
import type { OptionsSummaryV3 } from '@/generated/options_summary.v3'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [3] as const
export type Payload = OptionsSummaryV3
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
