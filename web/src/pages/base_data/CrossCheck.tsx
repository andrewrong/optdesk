// One opened cross-check pair: verdict, the reason line of a run that did not finish, the counts, the items grouped by
// company in payload order, and the items as shadcn tables. Every text is a payload field; nothing is computed here.
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/base_data_page'
import { LevelBadge } from './LevelBadge'

type Pair = NonNullable<Payload['groups'][number]['rows'][number]['cross_check']>
type Item = Pair['items'][number]

// The items of one company, kept in the order the payload sent them.
function byCompany(items: readonly Item[]): { company: string; items: Item[] }[] {
  const groups: { company: string; items: Item[] }[] = []
  const at = new Map<string, { company: string; items: Item[] }>()
  for (const it of items) {
    const company = it.company ?? ''
    let g = at.get(company)
    if (!g) {
      g = { company, items: [] }
      at.set(company, g)
      groups.push(g)
    }
    g.items.push(it)
  }
  return groups
}

export function CrossCheck({ pair }: { pair: Pair }) {
  return (
    <section data-cross-check={pair.pair_id} className="space-y-2 text-sm">
      <p className="flex items-center gap-2">
        <LevelBadge level={pair.level} />
        <span className="font-mono text-xs">{pair.pair_id}</span>
      </p>
      {pair.reason_zh ? (
        <p data-reason className="text-muted-foreground">
          {pair.reason_zh}
        </p>
      ) : null}
      <p>
        compared {pair.n}, matched {pair.matched}, differences {pair.diff}, missing {pair.missing}
        {pair.skipped !== undefined ? `, skipped ${pair.skipped}` : ''}
      </p>
      <p>
        {pair.tolerance_text !== '' ? <span>{pair.tolerance_text}; </span> : null}
        source {pair.source_id}
        {pair.run_at ? `, run ${pair.run_at}` : ''}
      </p>
      {byCompany(pair.items).map((g) => (
        <div key={g.company} data-company={g.company} className="space-y-1">
          {g.company !== '' ? <p className="font-medium">{g.company}</p> : null}
          <Table>
            <TableHeader>
              <TableRow>
                {['key', 'ours', 'other', 'difference', 'kind', 'hint'].map((h) => (
                  <TableHead key={h}>{h}</TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {g.items.map((it) => (
                <TableRow key={it.key}>
                  <TableCell>{it.key}</TableCell>
                  <TableCell>{it.ours}</TableCell>
                  <TableCell>{it.other}</TableCell>
                  <TableCell>{it.diff}</TableCell>
                  <TableCell>{it.kind}</TableCell>
                  <TableCell className="whitespace-normal">{it.hint_zh}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      ))}
    </section>
  )
}
