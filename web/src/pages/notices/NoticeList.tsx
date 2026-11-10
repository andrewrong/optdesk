// The notices of one alias, in payload order. Every visible string is a payload field; no sort control, nothing computed.
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/option_notices_page'

type Notice = Payload['accounts'][number]['notices'][number]

const SEVERITY_VARIANT = { info: 'secondary', warn: 'warning', act: 'destructive' } as const

function NoticeItem({ notice }: { notice: Notice }) {
  return (
    <li data-notice={notice.id} className="space-y-1">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant={SEVERITY_VARIANT[notice.severity]} data-severity={notice.severity}>
          {notice.severity}
        </Badge>
        <span className="font-medium" data-notice-title>
          {notice.title}
        </span>
        <span className="text-xs text-muted-foreground">{notice.contract}</span>
        {notice.expiry ? <span className="text-xs text-muted-foreground">{notice.expiry}</span> : null}
      </div>
      {notice.facts.length > 0 ? (
        <Table data-facts>
          <TableBody>
            {notice.facts.map((fact, index) => (
              <TableRow key={`${index}-${fact.label}`}>
                <TableCell className="text-muted-foreground">{fact.label}</TableCell>
                <TableCell>{fact.text}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      ) : null}
      {notice.reasons.map((reason, index) => (
        <p key={`${index}-${reason}`} className="text-xs text-muted-foreground">
          {reason}
        </p>
      ))}
    </li>
  )
}

export function NoticeList({ notices }: { notices: Notice[] }) {
  return (
    <ul className="space-y-3" data-notices>
      {notices.map((notice) => (
        <NoticeItem key={notice.id} notice={notice} />
      ))}
    </ul>
  )
}
