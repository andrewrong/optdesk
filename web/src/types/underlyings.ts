// The underlyings page payload: the generated v1 type of contracts/schemas/page.underlyings.v1.json. The structural
// check of a payload (an object with a supported `schema_version`) is the shared placeholder until the generated validators land.
import type { Underlyings } from '@/generated/underlyings.v1'
import { placeholderValidate } from './placeholder.ts'

export const SUPPORTED_VERSIONS = [1] as const
export type Payload = Underlyings

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
