// One card per alias: status, data day, notices, the "cash by date" table and the cushion text. A `no_capture` block
// shows its note and no table. Text as sent; nothing is compared, summed, sorted or formatted here.
import { Fragment } from 'react'
import { Info } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { CashPuts } from '@/pages/notices/CashPuts'
import { NoticeList } from '@/pages/notices/NoticeList'
import type { Payload } from '@/types/option_notices_page'

type Block = Payload['accounts'][number]

const STATUS_LABEL: Record<string, string> = { ok: 'Current', stale: 'Stale', no_capture: 'No capture' }

const CASH_GLOSS =
  "If every short put that expires on that day were assigned, this is the cash you would need to pay for the shares (strike × multiplier × quantity). 'Assigned' = the buyer of your put makes you buy the stock at the strike."

export function AliasBlock({ block }: { block: Block }) {
  return (
    <Card data-alias={block.alias} className="min-w-0">
      <CardHeader>
        <CardTitle className="flex flex-wrap items-center gap-2">
          <h2 className="text-lg font-semibold">{block.alias}</h2>
          <Badge variant="outline" data-status={block.status}>
            {STATUS_LABEL[block.status] ?? block.status}
          </Badge>
          {block.data_day ? <Badge variant="secondary">{block.data_day}</Badge> : null}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {block.note ? (
          <Alert data-note>
            <Info aria-hidden />
            <AlertDescription>{block.note}</AlertDescription>
          </Alert>
        ) : null}
        {typeof block.note_hint === 'string' ? (
          <p data-note-hint className="text-sm text-muted-foreground">
            {block.note_hint}
          </p>
        ) : null}
        {block.status === 'no_capture' ? null : (
          <>
            <NoticeList notices={block.notices} />
            <h3 data-cash-title className="text-base font-semibold">
              Cash due if assigned
            </h3>
            <p data-cash-gloss className="text-sm text-muted-foreground">
              {CASH_GLOSS}
            </p>
            {block.cash_by_date.length === 0 ? (
              <Alert data-cash-empty>
                <AlertDescription>No short put is open: no cash is due.</AlertDescription>
              </Alert>
            ) : (
              <Table data-cash>
                <TableHeader>
                  <TableRow>
                    <TableHead>Date</TableHead>
                    <TableHead>Currency</TableHead>
                    <TableHead>Amount</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {block.cash_by_date.map((row, index) => (
                    <Fragment key={`${index}-${row.day}-${row.currency}`}>
                      <TableRow>
                        <TableCell>{row.day}</TableCell>
                        <TableCell>{row.currency}</TableCell>
                        <TableCell>{row.amount_text}</TableCell>
                      </TableRow>
                      <CashPuts puts={row.puts} />
                    </Fragment>
                  ))}
                </TableBody>
              </Table>
            )}
            {block.cushion_text ? (
              <p className="text-sm" data-cushion>
                {block.cushion_text}
              </p>
            ) : null}
            {typeof block.cushion_hint === 'string' ? (
              <p data-cushion-hint className="text-sm text-muted-foreground">
                {block.cushion_hint}
              </p>
            ) : null}
          </>
        )}
      </CardContent>
    </Card>
  )
}
