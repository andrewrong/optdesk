// The valuation_import_page payload: the generated v1 type of contracts/schemas/page.valuation_import_page.v1.json. The structural check of a
// payload (an object with a supported `schema_version`) is the shared placeholder until the generated validators land.
import type { ValuationImportPage } from '@/generated/valuation_import_page.v1'
import { placeholderValidate } from './placeholder.ts'

export const SUPPORTED_VERSIONS = [1] as const
export type Payload = ValuationImportPage

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
