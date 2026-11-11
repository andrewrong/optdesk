// The money blocks of the household page: the block frame, the money table, the income months and the allocation
// table. Python supplies every figure; nothing here adds up or formats.
import { Info } from 'lucide-react'
import { motion } from 'motion/react'
import { DataTable, type Column } from '@/components/data-table/data-table'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/household_page'
import type { Cell, Gap, MoneyBlock, Shown } from './householdTypes'

const figure = (cell: Cell) => ({
  text: cell.text,
  value: cell.value,
  tone: cell.tone,
  approx_usd: cell.approx_usd,
})

export function Block({
  name,
  title,
  meaning,
  result,
  gap,
  children,
}: {
  name: string
  title: string
  meaning: string
  result?: string | null
  gap?: Gap | null
  children?: React.ReactNode
}) {
  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.2 }}
      className="min-w-0"
      data-block={name}
    >
      <Card className="min-w-0">
        <CardHeader>
          <CardTitle>{title}</CardTitle>
        </CardHeader>
        <CardContent className="min-w-0 space-y-3">
          {result ? (
            <p className="text-sm font-medium" data-result>
              {result}
            </p>
          ) : null}
          <p className="text-sm text-muted-foreground">{meaning}</p>
          {gap ? (
            <Alert data-block-gap data-reason={gap.reason}>
              <Info aria-hidden />
              <AlertDescription>{gap.text}</AlertDescription>
            </Alert>
          ) : null}
          {children}
        </CardContent>
      </Card>
    </motion.div>
  )
}

const label = (text: string) => ({ text })
const share = (item: Shown) => (item ? { text: item.text, tone: item.tone } : { text: '' })
// A total row is named by the payload's own total label when the block has one, else by the block's title; the
// figure under it is the payload's own total.
const totalRows = (
  block: {
    title: string
    totals: Cell[]
    approx_usd_total?: { text: string } | null
    total_labels?: string[] | null
    approx_usd_label?: string | null
  },
  blank: string[],
) => {
  const empty = Object.fromEntries(blank.map((id) => [id, label('')]))
  return [
    ...block.totals.map((cell, i) => ({
      name: label(block.total_labels?.[i] ?? block.title),
      amount: figure(cell),
      ...empty,
    })),
    ...(block.approx_usd_total
      ? [
          {
            name: label(block.approx_usd_label ?? block.title),
            amount: { text: block.approx_usd_total.text },
            ...empty,
          },
        ]
      : []),
  ]
}

function MoneyTable({ block }: { block: MoneyBlock }) {
  if (block.rows.length === 0) return null
  const hasShare = block.rows.some((row) => row.share)
  // Column heads and the due column exist only where Python sent `columns` (the obligations block).
  const heads = block.columns
  const columns: Column[] = [
    { id: 'name', label: heads?.[0] ?? block.heads?.[0] ?? '' },
    ...(heads
      ? [
          { id: 'due', label: heads[1] },
          { id: 'open_price', label: heads[2], align: 'right' as const },
        ]
      : []),
    { id: 'amount', label: heads?.[3] ?? block.heads?.[1] ?? '', align: 'right' },
    ...(hasShare ? [{ id: 'share', label: block.heads?.[2] ?? '', align: 'right' as const }] : []),
  ]
  const rows = block.rows.map((row) => ({
    name: { text: row.name, ...(row.note ? { note: row.note } : {}) },
    ...(heads
      ? {
          due: label(row.due ?? ''),
          open_price: label(row.open_price ? row.open_price.text : ''),
        }
      : {}),
    amount: figure(row.amount),
    share: share(row.share),
  }))
  const blank = [...(heads ? ['due', 'open_price'] : []), 'share']
  return <DataTable columns={columns} rows={[...rows, ...totalRows(block, blank)]} />
}

// Monthly income: a month with items opens to them (Python's item text, the account tag, the amount) and to Python's own
// sentence of what is in it; a month without items cannot be opened. Nothing is added up here.
export function IncomeMonths({ block }: { block: MoneyBlock }) {
  if (block.rows.length === 0) return null
  const totals = totalRows(block, [])
  return (
    <>
      <Accordion type="multiple" data-income-months>
        {block.rows.map((row) =>
          row.items ? (
            <AccordionItem key={row.name} value={row.name} data-month={row.name}>
              <AccordionTrigger>
                <span className="flex flex-1 justify-between gap-4 pr-2">
                  <span>{row.name}</span>
                  <span className="tabular-nums">{row.amount.text}</span>
                </span>
              </AccordionTrigger>
              <AccordionContent>
                <p className="mb-2 text-sm text-muted-foreground">{row.summary}</p>
                {row.items.length > 0 ? (
                  <Table>
                    <TableBody>
                      {row.items.map((item, i) => (
                        <TableRow key={`${item.part}-${i}`}>
                          <TableCell>{item.part}</TableCell>
                          <TableCell>{item.item}</TableCell>
                          <TableCell>
                            <Badge variant="secondary">{item.account.label}</Badge>
                          </TableCell>
                          <TableCell className="text-right tabular-nums">
                            {item.amount.text}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                ) : null}
              </AccordionContent>
            </AccordionItem>
          ) : (
            <div
              key={row.name}
              className="flex justify-between gap-4 py-2.5 text-sm"
              data-month={row.name}
            >
              <span>{row.name}</span>
              <span className="tabular-nums">{row.amount.text}</span>
            </div>
          ),
        )}
      </Accordion>
      {totals.length > 0 ? (
        <DataTable
          columns={[
            { id: 'name', label: '' },
            { id: 'amount', label: '', align: 'right' },
          ]}
          rows={totals}
        />
      ) : null}
    </>
  )
}

export function MoneySection({ name, block }: { name: string; block: MoneyBlock }) {
  return (
    <Block
      name={name}
      title={block.title}
      meaning={block.meaning}
      result={block.result}
      gap={block.gap}
    >
      <MoneyTable block={block} />
    </Block>
  )
}

export function AllocationSection({ block }: { block: Payload['allocation'] }) {
  const columns: Column[] = [
    { id: 'name', label: block.heads?.[0] ?? '' },
    { id: 'amount', label: block.heads?.[1] ?? '', align: 'right' },
    { id: 'share', label: block.heads?.[2] ?? '', align: 'right' },
    { id: 'target', label: block.heads?.[3] ?? '', align: 'right' },
    { id: 'difference', label: block.heads?.[4] ?? '', align: 'right' },
  ]
  const rows = block.rows.map((row) => ({
    name: label(row.name),
    amount: figure(row.amount),
    share: share(row.share),
    target: share(row.target),
    difference: share(row.difference),
  }))
  return (
    <Block name="allocation" title={block.title} meaning={block.meaning} gap={block.gap}>
      {block.rows.length > 0 ? (
        <DataTable
          columns={columns}
          rows={[...rows, ...totalRows(block, ['share', 'target', 'difference'])]}
        />
      ) : null}
    </Block>
  )
}
