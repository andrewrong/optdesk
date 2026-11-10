// One card per alias: status, data day, notices, the "cash by date" table and the cushion text. A `no_capture` block
// shows its note and no table. Text as sent; nothing is compared, summed, sorted or formatted here.
import { Info } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { NoticeList } from '@/pages/notices/NoticeList'
import type { Payload } from '@/types/option_notices_page'

type Block = Payload['accounts'][number]

export function AliasBlock({ block }: { block: Block }) {
  return (
    <Card data-alias={block.alias} className="min-w-0">
      <CardHeader>
        <CardTitle className="flex flex-wrap items-center gap-2">
          <h2 className="text-lg font-semibold">{block.alias}</h2>
          <Badge variant="outline" data-status={block.status}>
            {block.status}
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
        {block.status === 'no_capture' ? null : (
          <>
            <NoticeList notices={block.notices} />
            {block.cash_by_date.length > 0 ? (
              <Table data-cash>
                <TableHeader>
                  <TableRow>
                    <TableHead>日期</TableHead>
                    <TableHead>币种</TableHead>
                    <TableHead>金额</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {block.cash_by_date.map((row, index) => (
                    <TableRow key={`${index}-${row.day}-${row.currency}`}>
                      <TableCell>{row.day}</TableCell>
                      <TableCell>{row.currency}</TableCell>
                      <TableCell>{row.amount_text}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : null}
            {block.cushion_text ? (
              <p className="text-sm" data-cushion>
                {block.cushion_text}
              </p>
            ) : null}
          </>
        )}
      </CardContent>
    </Card>
  )
}
