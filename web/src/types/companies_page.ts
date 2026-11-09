// TEMPORARY hand-written placeholder (not generated; web/src/generated stays the stub). Types copied from the old repository until the week1-schemas issue generates them.
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'
import type { CompaniesPage as PayloadV1 } from './companies_page.v1.types.ts'
import type { CompaniesPageV2 } from '@/generated/companies_page.v2'

export type { Gap }
export const SUPPORTED_VERSIONS = [1, 2] as const
export type Payload = PayloadV1 | CompaniesPageV2
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
