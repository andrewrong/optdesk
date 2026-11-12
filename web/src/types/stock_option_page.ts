// The stock_option_page reads schema versions 1, 2 and 3; its payload types are the generated ones.
import type { StockOptionPage as PayloadV1 } from '@/generated/stock_option_page.v1'
import type { StockOptionPage as PayloadV2 } from '@/generated/stock_option_page.v2'
import type { StockOptionPage as PayloadV3 } from '@/generated/stock_option_page.v3'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [1, 2, 3] as const
export type Payload = PayloadV1 | PayloadV2 | PayloadV3
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
