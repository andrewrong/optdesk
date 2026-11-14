// TEMPORARY hand-written placeholder (not generated; web/src/generated stays the stub). Types copied from the old repository until the week1-schemas issue generates them.
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'
import type { MacroPage as PayloadV1 } from './macro_page.v1.types.ts'
import type { MacroPageV2 } from '@/generated/macro_page.v2'

export type { Gap }
export const SUPPORTED_VERSIONS = [1, 2] as const
export type { MacroPageV2 }
export type Payload = PayloadV1 | MacroPageV2
export type Result = Checked<Payload>

const checkVersion = placeholderValidate<Payload>(SUPPORTED_VERSIONS)

// The placeholder checks the version only; the two versions differ by exactly the liquidity fields, so a payload
// whose version and fields disagree (version 1 with them, version 2 without) is refused as an invalid payload.
export function validate(input: unknown): Result {
  const checked = checkVersion(input)
  if (!checked.ok) return checked
  const has = 'liquidity' in checked.payload && 'liquidity_title' in checked.payload
  const lacks = !('liquidity' in checked.payload) && !('liquidity_title' in checked.payload)
  if (checked.payload.schema_version === 2 ? has : lacks) return checked
  return {
    ok: false,
    gap: {
      reason: 'invalid_payload',
      text: 'The payload does not match its schema. Do not act on this card.',
    },
    errors: [],
  }
}
