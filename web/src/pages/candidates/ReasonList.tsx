// The reasons of the rows of a blocked-and-unknown table (Decision 7): every text is a payload field. `unknown` is drawn
// differently from `block` (outline badge and a question icon against a destructive badge and a stop icon).
import { CircleHelp, OctagonX } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import type { Payload } from '@/types/option_candidates_page'

type Group = Payload['groups'][number]
type Table = NonNullable<Group['other']>

export function ReasonList({ table }: { table: Table }) {
  const first = table.columns[0]?.id
  return (
    <ul className="space-y-2" data-reasons>
      {table.rows.map((row) => (
        <li key={row.id} data-reason-row={row.id} className="text-sm">
          <div className="font-medium">{(first !== undefined && row.cells[first]?.text) || row.id}</div>
          <ul className="space-y-1 pl-4">
            {row.reasons.map((reason, index) => {
              const unknown = reason.status === 'unknown'
              const Icon = unknown ? CircleHelp : OctagonX
              return (
                <li
                  key={`${index}-${reason.gate}-${reason.code}`}
                  data-reason={reason.status}
                  className="flex flex-wrap items-center gap-2"
                >
                  <Badge variant={unknown ? 'outline' : 'destructive'}>
                    <Icon aria-hidden />
                    {reason.status}
                  </Badge>
                  <Badge variant="secondary">{reason.gate}</Badge>
                  <span>{reason.text}</span>
                </li>
              )
            })}
          </ul>
        </li>
      ))}
    </ul>
  )
}
