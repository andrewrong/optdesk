// One account of the option tracking page: the open table, and the recent and check tables folded in accordions.
// Every text is a field of the payload; nothing is compared, summed, sorted or formatted here.
import { CircleAlert } from 'lucide-react'
import { DataTable, type Cell, type Column, type FigureCell } from '@/components/data-table/data-table'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { Cell as PayloadCell } from '@/generated/option_tracking_page.v1'
import type { Payload } from '@/types/option_tracking_page'

type Account = Payload['accounts'][number]
type Table = {
  columns: { id: string; label: string; align: 'left' | 'right'; hint: string | null }[]
  rows: { id: string; cells: Record<string, PayloadCell> }[]
}

// The guard badge text that is not settled or that blocks gets its own tone from the shared status classes of the
// data table; any other badge is neutral. This is a style choice on text Python wrote, not a verdict of the page.
const BADGE_STATUS: Record<string, FigureCell['source_status']> = {
  Unconfirmed: 'late',
  Blocked: 'missing',
}

// A cell as the shared data table reads it: the first badge rides in the source Badge, the rest in the note.
function cellOf(cell: PayloadCell): Cell {
  if ('kind' in cell) return { reason: cell.reason, text: cell.text }
  const [first, ...rest] = cell.badges
  return {
    text: cell.text,
    value: cell.value,
    tone: cell.tone,
    ...(first === undefined
      ? {}
      : { source: first, source_status: BADGE_STATUS[first] }),
    ...(rest.length > 0 ? { note: rest.join(' ') } : {}),
  }
}

function PayloadTable({ name, table }: { name: string; table: Table }) {
  const columns: Column[] = table.columns.map(({ id, label, align, hint }) => ({ id, label, align, hint }))
  return (
    <div data-table={name} className="min-w-0 overflow-x-auto">
      <DataTable
        columns={columns}
        rows={table.rows.map((row) =>
          Object.fromEntries(Object.entries(row.cells).map(([id, cell]) => [id, cellOf(cell)])),
        )}
      />
    </div>
  )
}

function Fold({ value, title, table }: { value: string; title: string; table: Table }) {
  return (
    <AccordionItem value={value}>
      <AccordionTrigger>{title}</AccordionTrigger>
      <AccordionContent>
        <PayloadTable name={value} table={table} />
      </AccordionContent>
    </AccordionItem>
  )
}

export function AccountSection({ account }: { account: Account }) {
  return (
    <Card data-account={account.alias}>
      <CardHeader>
        <CardTitle>{account.title}</CardTitle>
        <CardDescription className="flex flex-wrap gap-2">
          {account.mark_day ? <Badge variant="secondary">{account.mark_day}</Badge> : null}
          {account.guard_day ? <Badge variant="secondary">{account.guard_day}</Badge> : null}
          <Badge variant="outline" data-left-out>
            Long legs left out {account.long_legs_left_out}
          </Badge>
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {account.gap ? (
          <Alert data-gap={account.gap.reason}>
            <CircleAlert aria-hidden />
            <AlertDescription>{account.gap.text}</AlertDescription>
          </Alert>
        ) : (
          <>
            {account.open ? <PayloadTable name="open" table={account.open} /> : null}
            <Accordion type="multiple">
              {account.recent ? <Fold value="recent" title="Closed in the last 30 days" table={account.recent} /> : null}
              {account.recon ? <Fold value="recon" title="Flex reconciled with the account" table={account.recon} /> : null}
            </Accordion>
          </>
        )}
      </CardContent>
    </Card>
  )
}
