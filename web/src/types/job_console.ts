// The job console page payload: the generated v1 type of contracts/schemas/page.job_console.v1.json. The structural
// check of a payload (an object with a supported `schema_version`) is the shared placeholder until the generated validators land.
import type { JobConsole } from '@/generated/job_console.v1'
import { placeholderValidate } from './placeholder.ts'

export const SUPPORTED_VERSIONS = [1] as const
export type Payload = JobConsole

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
