import type { SettingsPage } from '../../../../src/generated/settings_page.v2'
import { editable } from './editable'

// The confirm step: the rows to confirm and the form whose token carries the confirm=1 post.
export const confirmStep: SettingsPage = {
  ...editable,
  notice: null,
  confirm: {
    form: { expires: '1790000900', mac: 'ef56ab78', post_path: '/owner-settings' },
    rows: [
      { label: 'demo Shortest expiry', old: '25', new: '28' },
      { label: 'DEMOB Excluded', old: 'No', new: 'Yes' },
    ],
  },
}
