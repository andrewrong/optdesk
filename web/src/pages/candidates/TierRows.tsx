// One tier's rows: a seven-column table per tier, one accordion item per row for the expansion, and the grey
// groups as closed accordions. Every text is a payload field; nothing is computed, sorted or filtered here.
import { Check, ChevronRight, Minus, TriangleAlert } from 'lucide-react'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'
import type { Badge as BadgeT, Field, Pair, Tier, TierColumn, TierRow } from '@/generated/option_candidates_page.v1'

const icons = { check: Check, alert: TriangleAlert, minus: Minus }
const borders: Record<string, string> = { fits: 'border-l-success', check: 'border-l-warn' }
const tones = { warn: 'warn', info: 'secondary', none: 'outline' } as const
const titleTones: Record<string, string> = { fits: 'text-success', check: 'text-warn' }
const wordVariants: Record<string, 'success' | 'warn' | 'outline'> = { fits: 'success', check: 'warn' }
const widths = ['w-10', 'w-28', 'w-56', 'w-64', 'w-32', 'w-36', 'sm:min-w-64']
const numeric = new Set(['margin', 'below_spot', 'premium'])
// The fit layout: a wrapping flex row below lg (badges on their own line), one line from lg up; no fixed widths.
const fitCells: Record<string, string> = {
  arrow: 'min-w-0 shrink-0 lg:basis-10',
  tier: 'min-w-0 shrink-0 lg:basis-24',
  contract: 'min-w-0 flex-1 basis-40 lg:basis-56',
  margin: 'min-w-0 flex-1 basis-1/4 lg:basis-28',
  below_spot: 'min-w-0 flex-1 basis-1/4 lg:basis-28',
  premium: 'min-w-0 flex-1 basis-1/4 lg:basis-32',
  badges: 'min-w-0 basis-full lg:flex-1 lg:basis-0',
}
const fitRow = 'flex min-w-0 flex-wrap items-start gap-x-2 border-l-4 px-2 py-2 lg:flex-nowrap'

function Hint({ text, children }: { text: string | null; children: React.ReactNode }) {
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

function Badges({ items }: { items: BadgeT[] }) {
  return (
    <>
      {items.map((b) => (
        <Hint key={b.code} text={b.gloss}>
          <Badge variant={tones[b.tone]} data-badge={b.code} className="h-auto whitespace-normal">
            {b.label}
          </Badge>
        </Hint>
      ))}
    </>
  )
}

function PairBody({ pair, wrap }: { pair: Pair; wrap?: boolean }) {
  return pair.gap ? (
    <span className="text-right text-muted-foreground">{pair.gap.text}</span>
  ) : (
    <>
      <div className="num text-right">{pair.main}</div>
      {pair.sub !== null ? (
        <div className={cn('num text-right text-xs text-muted-foreground', wrap ? 'whitespace-normal' : 'sm:whitespace-nowrap')}>{pair.sub}</div>
      ) : null}
    </>
  )
}

function PairCell({ id, pair }: { id: string; pair: Pair }) {
  return (
    <TableCell data-cell={id} className="text-right align-top max-sm:order-4 max-sm:basis-1/3 max-sm:whitespace-normal">
      <PairBody pair={pair} />
    </TableCell>
  )
}

function TierTitle({ tier }: { tier: Tier }) {
  return (
    <>
      <TierIcon tier={tier} />
      {tier.collapsed ? <Badge variant="outline">{tier.word}</Badge> : null}
      <span>{tier.title}</span>
    </>
  )
}

export function TierIcon({ tier }: { tier: Pick<Tier, 'icon' | 'word'> }) {
  const Icon = icons[tier.icon]
  return <Icon aria-label={tier.word} role="img" data-icon={tier.icon} className="size-4" />
}

function FieldItem({ field }: { field: Field }) {
  return (
    <div data-field={field.id} className="min-w-0">
      <div data-field-label className="text-xs text-muted-foreground">
        <Hint text={field.hint}>{field.label}</Hint>
      </div>
      <div className="flex flex-wrap items-center gap-1">
        {field.gap ? <span className="text-muted-foreground">{field.gap.text}</span> : <span>{field.value}</span>}
        <Badges items={field.badges} />
      </div>
    </div>
  )
}

function TierColgroup() {
  return (
    <colgroup>
      {widths.map((w) => (
        <col key={w} className={w} />
      ))}
    </colgroup>
  )
}

function ColLabel({ c, first }: { c: TierColumn; first: boolean }) {
  return (
    <>
      {first ? <span className="sr-only">{c.label}</span> : c.label}
      {c.hint ? (
        <span data-col-subtitle className="block text-xs font-normal whitespace-normal text-muted-foreground">
          {c.hint}
        </span>
      ) : null}
    </>
  )
}

export function TierHead({ columns, fit }: { columns: TierColumn[]; fit?: boolean }) {
  if (fit) {
    return (
      <div className={cn(fitRow, 'border-l-transparent text-sm font-medium text-muted-foreground')} data-col-head>
        {columns.map((c, i) => (
          <div key={c.id} data-col={c.id} className={cn(fitCells[c.id], numeric.has(c.id) && 'text-right')}>
            <ColLabel c={c} first={i === 0} />
          </div>
        ))}
      </div>
    )
  }
  return (
    <Table className="table-fixed">
      <TierColgroup />
      <TableHeader>
        <TableRow>
          {columns.map((c, i) => (
            <TableHead key={c.id} data-col={c.id} className={cn(i === columns.length - 1 && 'min-w-64', numeric.has(c.id) && 'text-right')}>
              {i === 0 ? <span className="sr-only">{c.label}</span> : c.label}
              {c.hint ? (
                <span data-col-subtitle className="block text-xs font-normal whitespace-normal text-muted-foreground">
                  {c.hint}
                </span>
              ) : null}
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
    </Table>
  )
}

// A row's own tier (the by_company layout) wins over the tier section it sits in.
const ownIcons = { fits: 'check', check: 'alert' } as const

function FitRow({ row, mark }: { row: TierRow; mark: Pick<Tier, 'id' | 'icon' | 'word'> }) {
  return (
    <div data-fit-row className={cn(fitRow, borders[mark.id] ?? 'border-l-muted')}>
      <div className={fitCells.arrow}>
        <AccordionTrigger
          data-arrow
          aria-label={row.contract.name}
          className="w-fit flex-none p-1 no-underline **:data-[slot=accordion-trigger-icon]:hidden!"
        >
          <ChevronRight className="size-4 transition-transform group-aria-expanded/accordion-trigger:rotate-90" />
        </AccordionTrigger>
      </div>
      <div className={fitCells.tier}>
        <Badge variant={wordVariants[mark.id] ?? 'outline'} className="gap-1">
          <TierIcon tier={mark} />
          {mark.word}
        </Badge>
      </div>
      <div data-cell="contract" className={fitCells.contract}>
        <div className="font-medium">{row.contract.name}</div>
        <div className="num text-xs text-muted-foreground">
          {row.contract.strike} {row.contract.expiry} {row.contract.dte}
        </div>
      </div>
      {(['margin', 'below_spot', 'premium'] as const).map((id) => (
        <div key={id} data-cell={id} className={cn(fitCells[id], 'text-right')}>
          <PairBody pair={row[id]} wrap />
        </div>
      ))}
      <div data-cell="badges" className={fitCells.badges}>
        <div className="flex flex-wrap gap-1">
          <Badges items={row.badges} />
          <Badges items={row.notes} />
        </div>
      </div>
    </div>
  )
}

export function DataRow({ tier, row, fit }: { tier: Tier | null; row: TierRow; fit?: boolean }) {
  const own = row.tier
  const mark = own ? { id: own.id, word: own.word, icon: ownIcons[own.id] } : tier
  if (!mark) return null
  const wide = !own && tier !== null && tier.collapsed
  return (
    <Accordion type="single" collapsible data-row={row.id}>
      <AccordionItem value={row.id}>
        {fit ? (
          <FitRow row={row} mark={mark} />
        ) : (
        <Table className="table-fixed max-sm:block">
          <TierColgroup />
          <TableBody className="max-sm:block">
            <TableRow className={cn('border-l-4 max-sm:flex max-sm:flex-wrap', borders[mark.id] ?? 'border-l-muted')}>
              <TableCell className="align-top max-sm:order-1 max-sm:w-10">
                <AccordionTrigger
                  data-arrow
                  aria-label={row.contract.name}
                  className="w-fit flex-none p-1 no-underline **:data-[slot=accordion-trigger-icon]:hidden!"
                >
                  <ChevronRight className="size-4 transition-transform group-aria-expanded/accordion-trigger:rotate-90" />
                </AccordionTrigger>
              </TableCell>
              <TableCell className="align-top max-sm:order-2">
                <Badge variant={wordVariants[mark.id] ?? 'outline'} className="gap-1">
                  <TierIcon tier={mark} />
                  {mark.word}
                </Badge>
              </TableCell>
              <TableCell data-cell="contract" className="align-top max-sm:order-3 max-sm:min-w-0 max-sm:flex-1 max-sm:whitespace-normal">
                <div className="font-medium">{row.contract.name}</div>
                <div className="num text-xs text-muted-foreground">
                  {row.contract.strike} {row.contract.expiry} {row.contract.dte}
                </div>
              </TableCell>
              <PairCell id="margin" pair={row.margin} />
              <PairCell id="below_spot" pair={row.below_spot} />
              <PairCell id="premium" pair={row.premium} />
              <TableCell data-cell="badges" className="align-top sm:min-w-64 max-sm:order-5 max-sm:basis-full">
                <div className="flex flex-wrap gap-1">
                  <Badges items={row.badges} />
                  <Badges items={row.notes} />
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
        )}
        <AccordionContent data-expand>
          {wide ? (
            <p data-line className="p-3 text-sm">
              {row.line}
            </p>
          ) : (
            <div className="grid gap-3 p-3 sm:grid-cols-2 lg:grid-cols-3">
              {row.fields.map((f) => (
                <FieldItem key={f.id} field={f} />
              ))}
            </div>
          )}
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  )
}

function TierTable({ tier, columns }: { tier: Tier; columns: TierColumn[] }) {
  if (tier.rows.length === 0) return null
  return (
    <div className="overflow-x-auto">
      <div className="sm:min-w-[66rem]">
        <div className="max-sm:hidden">
          <TierHead columns={columns} />
        </div>
        {tier.rows.map((row) => (
          <DataRow key={row.id} tier={tier} row={row} />
        ))}
      </div>
    </div>
  )
}

function GreyGroup({ tier, columns }: { tier: Tier; columns: TierColumn[] }) {
  return (
    <Accordion type="single" collapsible data-tier={tier.id}>
      <AccordionItem value={tier.id}>
        <AccordionTrigger className="text-muted-foreground">
          <span className="flex items-center gap-2">
            <TierTitle tier={tier} />
          </span>
        </AccordionTrigger>
        <AccordionContent>
          <p className="mb-2 text-xs text-muted-foreground">{tier.explain}</p>
          <TierTable tier={tier} columns={columns} />
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  )
}

export function TierSection({ tier, columns }: { tier: Tier; columns: TierColumn[] }) {
  if (tier.collapsed) return <GreyGroup tier={tier} columns={columns} />
  return (
    <section data-tier={tier.id} className="min-w-0 space-y-1">
      <h3 className={cn('flex items-center gap-2 text-base font-semibold', titleTones[tier.id])}>
        <TierTitle tier={tier} />
      </h3>
      {tier.rows.length === 0 ? <p className="text-xs text-muted-foreground">{tier.explain}</p> : null}
      <TierTable tier={tier} columns={columns} />
    </section>
  )
}
