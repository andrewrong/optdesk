// The company data page payload: the generated v1 type of contracts/schemas/page.company_data_page.v1.json. The check of a
// payload is the shared placeholder plus a refusal of an unknown field, until the generated validators land.
import type { CompanyDataPage as PayloadV1 } from '@/generated/company_data_page.v1'
import type { CompanyDataPage as PayloadV2 } from '@/generated/company_data_page.v2'
import { arr, obj, strictValidate } from './strict_keys.ts'

export const SUPPORTED_VERSIONS = [1, 2] as const
export type Payload = PayloadV1 | PayloadV2

export const validate = strictValidate<Payload>(
  SUPPORTED_VERSIONS,
  obj({
    schema: null,
    schema_version: null,
    run_id: null,
    built_at: null,
    data_health_path: null,
    columns: arr(obj({ key: null, label: null, gloss: null })),
    companies: arr(
      obj({
        key: null,
        symbol: null,
        name: null,
        market: null,
        issuer: null,
        fiscal_year_end: null,
        price_to: null,
        bar_count: null,
        statements: null,
        latest_quarter: null,
        eps_quarters: null,
        next_report: null,
        missing: arr(obj({ code: null, text: null })),
        quarters_path: null,
      }),
    ),
    bars_source: null,
    bars_source_note: null,
    glossary: arr(obj({ term: null, text: null })),
    gaps: arr(obj({ kind: null, reason: null, text: null })),
  }),
)
