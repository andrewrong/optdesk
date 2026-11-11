import type { SettingsPage } from '../../../../src/generated/settings_page.v2'
import { editable } from './editable'

// The same page with the form switched off (`form` null) and the gap that says so.
export const readOnly: SettingsPage = {
  ...editable,
  form: null,
  notice: null,
  gap: { reason: 'not_built', text: 'demo Edit switch is off' },
}
