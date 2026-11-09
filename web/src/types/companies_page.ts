// TEMPORARY hand-written placeholder (not generated; web/src/generated stays the stub). Types copied from the old repository until the week1-schemas issue generates them.
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'
import type { CompaniesPage as PayloadV1 } from './companies_page.v1.types.ts'
import type { CompaniesPageV2 } from '@/generated/companies_page.v2'

export type { Gap }
export const SUPPORTED_VERSIONS = [1, 2] as const
export type Payload = PayloadV1 | CompaniesPageV2
export type Result = Checked<Payload>

const checkVersion = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
const V2_BLOCKS = ['core', 'diagnosis', 'questions'] as const

// A version 2 document carries the three v2 blocks (each the block or a Gap); one without them is not v2.
export const validate = (input: unknown): Result => {
  const checked = checkVersion(input)
  if (!checked.ok || (checked.payload as { schema_version: number }).schema_version !== 2) {
    return checked
  }
  const doc = checked.payload as unknown as Record<string, unknown>
  if (V2_BLOCKS.every((key) => typeof doc[key] === 'object' && doc[key] !== null)) return checked
  return {
    ok: false,
    gap: {
      reason: 'invalid_payload',
      text: 'The payload does not match its schema. Do not act on this card.',
    },
    errors: [],
  }
}
