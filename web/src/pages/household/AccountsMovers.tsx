// The account, mover and scope parts of the household page. The names, prices, money and tags are Python's text,
// in the order sent; nothing is summed or sorted here.
import { Info, Users } from 'lucide-react'
import { TONE_CLASS } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Block } from './MoneySections'
import type { Accounts, MoverLine, Movers, Scope, Tag } from './householdTypes'

// A scope heading: the label and the icon are Python's words and Lucide's; nothing is counted here.
export function ScopeHeading({ scope }: { scope: Scope }) {
  return (
    <h3 className="flex items-center gap-2 text-sm font-medium" data-scope={scope.key}>
      <Users aria-hidden className="size-4" />
      {scope.label}
    </h3>
  )
}

export function WhoseBadge({ tag }: { tag: Pick<Tag, 'whose'> }) {
  return <Badge variant="secondary">{tag.whose}</Badge>
}

// The account table (issues #1588, #1680): the rows sit under their scope; a figure is the payload's cell text, a
// missing one the payload's own gap words.
export function AccountsSection({ block, scopes }: { block: Accounts; scopes: Scope[] }) {
  if (block.rows.length === 0) return null
  const shown = (figure: Accounts['rows'][number]['cash']) => figure.cell?.text ?? figure.gap ?? ''
  return (
    <Block name="accounts" title={block.title} meaning={block.meaning}>
      {scopes.map((scope) => {
        const rows = block.rows.filter((row) => row.scope === scope.key)
        if (rows.length === 0) return null
        return (
          <section key={scope.key} className="min-w-0 space-y-2" data-scope-rows={scope.key}>
            <ScopeHeading scope={scope} />
            <Table>
              <TableHeader>
                <TableRow>
                  {block.columns.map((head, i) => (
                    <TableHead key={head} className={i > 0 && i < 5 ? 'text-right' : undefined}>
                      {head}
                    </TableHead>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((row) => (
                  <TableRow key={row.name}>
                    <TableCell>
                      {row.name} <WhoseBadge tag={row} />
                    </TableCell>
                    <TableCell className="text-right">{shown(row.net_liquidation)}</TableCell>
                    <TableCell className="text-right">{shown(row.cash)}</TableCell>
                    <TableCell className="text-right">{shown(row.stock)}</TableCell>
                    <TableCell className="text-right">{shown(row.options)}</TableCell>
                    <TableCell>{row.data_day ?? ''}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </section>
        )
      })}
    </Block>
  )
}

// The tone is Python's word; a missing or none tone has no colour. The map is the Data Table's.
const toneClass = (tone: string | null | undefined) =>
  tone && tone !== 'none' ? TONE_CLASS[tone as keyof typeof TONE_CLASS] : undefined

// One of the two lists of What moved today (issue #1681): the names, prices, money and tags are Python's text, in the
// order sent; nothing is summed or sorted here.
function MoverTable({ title, heads, rows }: { title: string; heads: string[]; rows: MoverLine[] }) {
  if (rows.length === 0) return null
  const shown = (price: MoverLine['previous']) => price.text
  return (
    <div className="min-w-0 space-y-1" data-mover-list={title}>
      <h4 className="text-xs font-medium text-muted-foreground">{title}</h4>
      <Table>
        <TableHeader>
          <TableRow>
            {heads.map((head, i) => (
              <TableHead key={head} className={i > 0 ? 'text-right' : undefined}>
                {head}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={`${row.scope}-${row.account.label}-${row.name}`}>
              <TableCell>
                {row.name} <WhoseBadge tag={row.account} />
                <span className="ml-1 text-xs text-muted-foreground">{row.account.label}</span>
              </TableCell>
              <TableCell className="text-right">{shown(row.previous)}</TableCell>
              <TableCell className="text-right">{shown(row.current)}</TableCell>
              <TableCell className="text-right">
                {row.amount ? (
                  <span className={toneClass(row.amount.tone)}>{row.amount.text}</span>
                ) : (
                  (row.note ?? '')
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

export function MoversSection({ block, scopes }: { block: Movers; scopes: Scope[] }) {
  const heads = block.columns ?? []
  return (
    <Block name="movers" title={block.title} meaning={block.meaning} gap={block.gap}>
      {scopes.map((scope) => {
        const stocks = (block.stocks ?? []).filter((row) => row.scope === scope.key)
        const options = (block.options ?? []).filter((row) => row.scope === scope.key)
        const other = block.rows.filter((row) => row.scope === scope.key)
        const gap = block.scope_gaps?.find((g) => g.scope === scope.key)
        const total = scope.key === 'household' ? block.totals : []
        if (stocks.length + options.length + other.length + total.length === 0 && !gap) return null
        return (
          <section key={scope.key} className="min-w-0 space-y-2" data-scope-rows={scope.key}>
            <ScopeHeading scope={scope} />
            {gap ? (
              <Alert data-scope-gap data-reason={gap.gap.reason}>
                <Info aria-hidden />
                <AlertDescription>{gap.gap.text}</AlertDescription>
              </Alert>
            ) : null}
            <MoverTable title="Stocks" heads={heads} rows={stocks} />
            <MoverTable title="Options" heads={heads} rows={options} />
            {other.length + total.length > 0 ? (
              <Table>
                <TableBody>
                  {other.map((row) => (
                    <TableRow key={row.name}>
                      <TableCell>{row.name}</TableCell>
                      <TableCell className="text-right">{row.amount.text}</TableCell>
                    </TableRow>
                  ))}
                  {total.map((cell, i) => (
                    <TableRow key={`total-${i}`}>
                      <TableCell className="font-medium">{block.title}</TableCell>
                      <TableCell className="text-right">{cell.text}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : null}
          </section>
        )
      })}
    </Block>
  )
}
