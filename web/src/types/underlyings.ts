// The underlyings page payload: the generated v2 type of contracts/schemas/page.underlyings.v2.json. The structural
// check of a payload (an object with a supported `schema_version`) is the shared placeholder until the generated validators land.
import type { Underlyings } from '@/generated/underlyings.v2'
import { placeholderValidate } from './placeholder.ts'

export const SUPPORTED_VERSIONS = [2] as const
export type Payload = Underlyings

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
