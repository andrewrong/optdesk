// TEMPORARY hand-written placeholder (not generated; web/src/generated stays the stub). Types copied from the old repository until the week1-schemas issue generates them.
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'
import type { PositionsPage as PayloadV1 } from './positions_page.v1.types.ts'
import type { PositionsPage as PayloadV2 } from './positions_page.v2.types.ts'
import type { PositionsPage as PayloadV3 } from './positions_page.v3.types.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [1, 2, 3] as const
export type Payload = PayloadV1 | PayloadV2 | PayloadV3
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
