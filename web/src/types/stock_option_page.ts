// The stock_option_page reads schema version 1 only; its payload type is the generated one.
import type { StockOptionPage } from '@/generated/stock_option_page.v1'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [1] as const
export type Payload = StockOptionPage
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
