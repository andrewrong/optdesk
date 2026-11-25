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
  { id: 'dataset', label: '数据集' },
  { id: 'key', label: '键' },
  { id: 'market', label: '市场' },
  { id: 'status', label: '状态' },
  { id: 'error_code', label: '错误码' },
  { id: 'reason_code', label: '原因码' },
  { id: 'returned_n', label: '返回数', align: 'right' },
  { id: 'usable_n', label: '可用数', align: 'right' },
  { id: 'detail', label: '详情', wrap: true },
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
