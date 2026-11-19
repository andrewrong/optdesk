// The grouped short list table: eight columns, one header row per underlying that spans them, one row per short put
// in payload order. Every text is a payload field; nothing is computed, sorted or filtered here.
import { Fragment } from 'react'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import type { Badge as BadgeT, Pair, ShortGroup, ShortRow, TierColumn } from '@/generated/option_candidates_page.v1'

const tones = { warn: 'warn', info: 'secondary', none: 'outline' } as const

export function Hint({ text, children }: { text: string | null; children: React.ReactNode }) {
  if (text === null) return <>{children}</>
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span data-hint={text} tabIndex={0} className="cursor-help underline decoration-dotted underline-offset-2">
          {children}
        </span>
      </TooltipTrigger>
      <TooltipContent>{text}</TooltipContent>
    </Tooltip>
  )
}

function Tag({ badge, source }: { badge: BadgeT; source?: boolean }) {
  return (
    <Hint text={badge.gloss}>
      <Badge
        variant={tones[badge.tone]}
        data-badge={source ? undefined : badge.code}
        data-buy-source={source ? badge.code : undefined}
        className="h-auto whitespace-normal"
      >
        {badge.label}
      </Badge>
    </Hint>
  )
}

function PairCell({ id, pair }: { id: string; pair: Pair }) {
  return (
    <TableCell data-cell={id} className="align-top">
      {pair.gap ? (
        <span className="text-muted-foreground">{pair.gap.text}</span>
      ) : (
        <>
          <div>{pair.main}</div>
          {pair.sub !== null ? <div className="text-xs text-muted-foreground">{pair.sub}</div> : null}
        </>
      )}
    </TableCell>
  )
}

function BuyCell({ row }: { row: ShortRow }) {
  return (
    <TableCell data-cell="buy" className="align-top">
      {row.buy_link !== null ? (
        <a href={row.buy_link} className="text-primary underline underline-offset-2">
          {row.buy.gap ? row.buy.gap.text : row.buy.main}
        </a>
      ) : row.buy.gap ? (
        <span className="text-muted-foreground">{row.buy.gap.text}</span>
      ) : (
        <div>{row.buy.main}</div>
      )}
      {row.buy.gap === null && row.buy.sub !== null ? (
        <div className="text-xs text-muted-foreground">{row.buy.sub}</div>
      ) : null}
      {row.buy_source ? <Tag badge={row.buy_source} source /> : null}
    </TableCell>
  )
}

function ShortRowView({ row }: { row: ShortRow }) {
  return (
    <TableRow data-short-row={row.key}>
      <PairCell id="strike" pair={row.strike} />
      <PairCell id="expiry" pair={row.expiry} />
      <PairCell id="otm" pair={row.otm} />
      <PairCell id="delta" pair={row.delta} />
      <PairCell id="premium" pair={row.premium} />
      <PairCell id="annualized" pair={row.annualized} />
      <BuyCell row={row} />
      <TableCell data-cell="liquidity" className="align-top">
        <div className="flex flex-wrap gap-1">
          {row.badges.map((b) => (
            <Tag key={b.code} badge={b} />
          ))}
        </div>
      </TableCell>
    </TableRow>
  )
}

export function ShortListRows({ columns, groups }: { columns: TierColumn[]; groups: ShortGroup[] }) {
  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            {columns.map((c) => (
              <TableHead key={c.id} data-col={c.id}>
                <Hint text={c.hint}>{c.label}</Hint>
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {groups.map((g) => (
            <Fragment key={g.ticker}>
              <TableRow data-group={g.ticker} className="bg-muted/50">
                <TableCell colSpan={columns.length} className="font-semibold">
                  <span>{g.ticker}</span> <span className="font-normal text-muted-foreground">{g.spot}</span>{' '}
                  <span className="text-xs font-normal text-muted-foreground">{g.summary}</span>
                </TableCell>
              </TableRow>
              {g.rows.map((r) => (
                <ShortRowView key={r.key} row={r} />
              ))}
            </Fragment>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
