// Same-morning visibility (docs/design/option-open-gap-jp.md#Same-morning-visibility, Decision 11). The rows are the
// producer's, in the producer's order: the view ranks nothing, sums nothing and formats nothing.
import type { ComponentProps } from 'react'
import { DataTable, type Column } from '@/components/data-table/data-table'
import type { Row } from '@/generated/ops_today_page.v1'
import { CardNotes } from '@/pages/home/CardNotes'
import { StaleAlert } from '@/pages/home/StaleAlert'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/ops_today_page'

const COLUMNS: Column[] = [
  { id: 'dataset', label: 'Dataset' },
  { id: 'key', label: 'Key' },
  { id: 'market', label: 'Market' },
  { id: 'status', label: 'Status' },
  { id: 'error_code', label: 'Error code' },
  { id: 'reason_code', label: 'Reason code' },
  { id: 'returned_n', label: 'Returned', align: 'right' },
  { id: 'usable_n', label: 'Usable', align: 'right' },
  { id: 'detail', label: 'Detail', wrap: true },
]

// A null text is shown empty; the producer's words are not replaced.
const cell = (text: string | null, badge = false) => ({ text: text ?? '', badge: badge && text !== null })

type TableRow = ComponentProps<typeof DataTable>['rows'][number]

const toTableRow = (row: Row): TableRow => ({
  dataset: cell(row.dataset),
  key: cell(row.key),
  market: cell(row.market),
  status: cell(row.status, true),
  error_code: cell(row.error_code),
  reason_code: cell(row.reason_code, true),
  returned_n: cell(`${row.returned_n}`),
  usable_n: cell(`${row.usable_n}`),
  detail: cell(row.detail),
})

export function OpsTodayPage({ payload }: { payload: Payload }) {
  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">{payload.title}</h1>
      {payload.stale ? (
        <StaleAlert
          stale={{ text: payload.stale.text, stale_after: payload.stale_after ?? '' }}
          generatedAt={payload.generated_at}
        />
      ) : null}
      <CardNotes notes={payload.notes} gaps={payload.gaps} />
      <DataTable columns={COLUMNS} rows={payload.today.rows.map(toTableRow)} />
    </div>
  )
}

export const registration = { schema: 'ops_today_page', page: page<Payload>(validate, OpsTodayPage) }
