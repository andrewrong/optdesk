// The rows of one tier: a seven-column table per tier, one accordion item per row for the expansion, and the grey
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
const widths = ['w-10', 'w-28', 'w-56', 'w-40', 'w-32', 'w-36', 'min-w-64']

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

export function TierIcon({ tier }: { tier: Tier }) {
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

function Cols() {
  return (
    <colgroup>
      {widths.map((w) => (
        <col key={w} className={w} />
      ))}
    </colgroup>
  )
}

export function TierHead({ columns }: { columns: TierColumn[] }) {
  return (
    <Table className="table-fixed">
      <Cols />
      <TableHeader>
        <TableRow>
          {columns.map((c, i) => (
            <TableHead key={c.id} data-col={c.id} className={cn(i === columns.length - 1 && 'min-w-64')}>
              <Hint text={c.hint}>{c.label}</Hint>
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
    </Table>
  )
}

function DataRow({ tier, row }: { tier: Tier; row: TierRow }) {
  return (
    <Accordion type="single" collapsible data-row={row.id}>
      <AccordionItem value={row.id}>
        <Table className="table-fixed">
          <Cols />
          <TableBody>
            <TableRow className={cn('border-l-4', borders[tier.id] ?? 'border-l-muted')}>
              <TableCell className="align-top">
                <AccordionTrigger
                  data-arrow
                  aria-label={row.contract.name}
                  className="w-fit flex-none p-1 no-underline **:data-[slot=accordion-trigger-icon]:hidden!"
                >
                  <ChevronRight className="size-4 transition-transform group-aria-expanded/accordion-trigger:rotate-90" />
                </AccordionTrigger>
              </TableCell>
              <TableCell className="align-top">
                <Badge variant="outline" className="gap-1">
                  <TierIcon tier={tier} />
                  {tier.word}
                </Badge>
              </TableCell>
              <TableCell data-cell="contract" className="align-top">
                <div className="font-medium">{row.contract.name}</div>
                <div className="text-xs text-muted-foreground">
                  {row.contract.strike} {row.contract.expiry} {row.contract.dte}
                </div>
              </TableCell>
              <PairCell id="margin" pair={row.margin} />
              <PairCell id="below_spot" pair={row.below_spot} />
              <PairCell id="premium" pair={row.premium} />
              <TableCell data-cell="badges" className="min-w-64 align-top">
                <div className="flex flex-wrap gap-1">
                  <Badges items={row.badges} />
                  <Badges items={row.notes} />
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
        <AccordionContent data-expand>
          <div className="grid gap-3 p-3 sm:grid-cols-2 lg:grid-cols-3">
            {row.fields.map((f) => (
              <FieldItem key={f.id} field={f} />
            ))}
          </div>
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  )
}

function GreyGroup({ tier }: { tier: Tier }) {
  return (
    <Accordion type="single" collapsible data-tier={tier.id}>
      <AccordionItem value={tier.id}>
        <AccordionTrigger>
          <span className="flex items-center gap-2">
            <TierIcon tier={tier} />
            <span>{tier.word}</span>
            <span>{tier.title}</span>
          </span>
        </AccordionTrigger>
        <AccordionContent>
          <p className="mb-2 text-xs text-muted-foreground">{tier.explain}</p>
          {tier.rows.map((row) => (
            <p key={row.id} data-row={row.id} className="border-l-4 border-l-muted py-1 pl-2 text-sm">
              {row.line}
            </p>
          ))}
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  )
}

export function TierSection({ tier, columns }: { tier: Tier; columns: TierColumn[] }) {
  if (tier.collapsed) return <GreyGroup tier={tier} />
  return (
    <section data-tier={tier.id} className="min-w-0 space-y-1">
      <h3 className="flex items-center gap-2 text-base font-semibold">
        <TierIcon tier={tier} />
        <span>{tier.word}</span>
        <span>{tier.title}</span>
      </h3>
      <p className="text-xs text-muted-foreground">{tier.explain}</p>
      <div className="overflow-x-auto">
        <div className="min-w-[60rem]">
          <TierHead columns={columns} />
          {tier.rows.map((row) => (
            <DataRow key={row.id} tier={tier} row={row} />
          ))}
        </div>
      </div>
    </section>
  )
}
