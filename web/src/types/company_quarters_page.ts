// The company quarters page payload: the generated v1 type of contracts/schemas/page.company_quarters_page.v1.json. The check
// of a payload is the shared placeholder plus a refusal of an unknown field, until the generated validators land.
import type { CompanyQuartersPage } from '@/generated/company_quarters_page.v1'
import { arr, map, obj, strictValidate } from './strict_keys.ts'

export const SUPPORTED_VERSIONS = [1] as const
export type Payload = CompanyQuartersPage

const gap = obj({ kind: null, reason: null, text: null })
const column = obj({ key: null, label: null, gloss: null })
const cell = obj({ text: null, missing: null, derived: null, restated: null, concept: null, why_text: null })

export const validate = strictValidate<Payload>(
  SUPPORTED_VERSIONS,
  obj({
    schema: null,
    schema_version: null,
    run_id: null,
    built_at: null,
    company: obj({ key: null, symbol: null, name: null, issuer: null, fiscal_year_end: null }),
    choices: arr(obj({ key: null, label: null, path: null })),
    columns: arr(column),
    quarters: arr(obj({ label: null, period_end: null, filing_date: null, cells: map(cell) })),
    notes: null,
    gaps: arr(gap),
  }),
)
