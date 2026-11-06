// TEMPORARY hand-written placeholder (not generated; web/src/generated stays the stub). Types copied from the old repository until the week1-schemas issue generates them.
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'
import type { StructuresList as PayloadV1 } from './structures_list.v1.types.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [1] as const
export type Payload = PayloadV1
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
