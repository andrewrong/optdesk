// The settings page payload: the generated v2 type of contracts/schemas/page.settings_page.v2.json (`@/generated/settings_page.v2`).
// The structural check of a payload (an object with a supported `schema_version`) is the shared placeholder until the generated
// validators land.
import type { SettingsPage } from '@/generated/settings_page.v2'
import { placeholderValidate, type Gap, type Result as Checked } from './placeholder.ts'

export type { Gap }
export const SUPPORTED_VERSIONS = [2] as const
export type Payload = SettingsPage
export type Result = Checked<Payload>

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
