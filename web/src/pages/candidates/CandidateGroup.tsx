// One group of the option candidates page: the gap cause once and no table, or the main table with the folded
// blocked-and-unknown table below it. Every text is a payload field; nothing is compared, summed, sorted or formatted.
import { Info, TriangleAlert } from 'lucide-react'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { ReasonList } from '@/pages/candidates/ReasonList'
import type { Payload } from '@/types/option_candidates_page'

type Group = Payload['groups'][number]
type PayloadTable = NonNullable<Group['main']>

// A value cell goes to the data-table as sent: its badges (such as `Stale`) ride on the note line of the cell.
function GroupTable({ table, name }: { table: PayloadTable; name: string }) {
  const columns: Column[] = table.columns.map(({ id, label, align, hint }) => ({ id, label, align, hint }))
  const rows = table.rows.map((row) =>
    Object.fromEntries(
      Object.entries(row.cells).map(([id, cell]) => [
        id,
        'badges' in cell
          ? { text: cell.text, value: cell.value, tone: cell.tone, note: cell.badges.join(' · ') || null }
          : (cell as Cell),
      ]),
    ),
  )
  return (
    <div className="min-w-0" data-table={name}>
      <DataTable columns={columns} rows={rows} />
    </div>
  )
}

export function CandidateGroup({ group, staleText }: { group: Group; staleText: string | null }) {
  return (
    <div className="min-w-0 space-y-3" data-group={group.kind}>
      {group.stale && staleText ? (
        <Alert variant="destructive" data-group-stale>
          <TriangleAlert aria-hidden />
          <AlertDescription>{staleText}</AlertDescription>
        </Alert>
      ) : null}
      {group.gap ? (
        <Alert data-group-gap={group.gap.reason}>
          <Info aria-hidden />
          <AlertDescription>{group.gap.text}</AlertDescription>
        </Alert>
      ) : null}
      {group.main ? <GroupTable table={group.main} name="main" /> : null}
      {group.other ? (
        <Accordion type="single" collapsible data-other>
          <AccordionItem value="other">
            <AccordionTrigger>Not passed / unconfirmed ({group.other.rows.length})</AccordionTrigger>
            <AccordionContent className="space-y-3">
              <GroupTable table={group.other} name="other" />
              <ReasonList table={group.other} />
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      ) : null}
    </div>
  )
}
