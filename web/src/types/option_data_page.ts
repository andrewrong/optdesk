import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'
import type { OptionDataPage } from '@/generated/option_data_page.v1'

export type { Gap }
export const SUPPORTED_VERSIONS = [1] as const
export type Payload = OptionDataPage
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
