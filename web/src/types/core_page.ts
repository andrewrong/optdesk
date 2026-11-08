// The core page payload: the generated v1 type of contracts/schemas/page.core_page.v1.json. The structural
// check of a payload (an object with a supported `schema_version`) is the shared placeholder until the generated validators land.
import type { CorePage } from '@/generated/core_page.v1'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [1] as const
export type Payload = CorePage
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
