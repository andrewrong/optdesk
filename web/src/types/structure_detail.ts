// The structure detail page reads schema version 3 only; its payload type is the generated one.
import type { StructureDetail } from '@/generated/structure_detail.v3'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [3] as const
export type Payload = StructureDetail
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
