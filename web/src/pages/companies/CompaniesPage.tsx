// The companies page (ADR 0034 step 5, issue #1643). Every figure, sentence and sort of the lines is supplied by
// Python; the browser shows the texts of the payload and nothing else.
import { Info } from 'lucide-react'
import { DataTable, type Column } from '@/components/data-table/data-table'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Payload } from '@/types/companies_page'
import { CardNotes } from '../home/CardNotes'
import { StaleAlert } from '../home/StaleAlert'
export { validate as validateCompaniesPage } from '@/types/companies_page'
export type { Payload as CompaniesPagePayload } from '@/types/companies_page'

type Head = { numeric?: boolean; text: string }
type Cells = { text: string; tone?: 'up' | 'down' | 'flat' | 'none' }[]

function Section({
  name,
  title,
  children,
}: {
  name: string
  title: string
  children: React.ReactNode
}) {
  return (
    <Card className="min-w-0" data-block={name}>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent className="min-w-0 space-y-3">{children}</CardContent>
    </Card>
  )
}

const columnsOf = (heads: Head[]): Column[] =>
  heads.map((head, i) => ({
    id: `c${i}`,
    label: head.text,
    align: head.numeric ? 'right' : 'left',
  }))
const rowOf = (cells: Cells) =>
  Object.fromEntries(cells.map((cell, i) => [`c${i}`, { text: cell.text, tone: cell.tone }]))

export function CompaniesPage({ payload }: { payload: Payload }) {
  const { changes, glance, holdings, watch_list: watch } = payload
  return (
    <div className="min-w-0 space-y-6" data-page="companies">
      <header>
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        {payload.header.line ? (
          <p className="mt-2 text-muted-foreground">{payload.header.line}</p>
        ) : null}
      </header>
      <StaleAlert stale={payload.stale} generatedAt={payload.generated_at} />
      <CardNotes notes={[]} gaps={payload.gaps} />
      {changes ? (
        <Section name="changes" title={changes.title}>
          {changes.note ? <p className="text-xs text-muted-foreground">{changes.note}</p> : null}
          {changes.sentence ? <p>{changes.sentence}</p> : null}
          {changes.lines.length > 0 ? (
            <ul className="list-disc pl-4">
              {changes.lines.map((line, i) => (
                <li key={`${i}-${line}`}>{line}</li>
              ))}
            </ul>
          ) : null}
        </Section>
      ) : null}
      {glance ? (
        <Section name="glance" title={glance.title}>
          {glance.note ? <p className="text-xs text-muted-foreground">{glance.note}</p> : null}
          <dl className="grid grid-cols-2 gap-3 md:grid-cols-4">
            {glance.tiles.map((tile, i) => (
              <div key={`${i}-${tile.label}`} data-tile>
                <dt className="text-xs text-muted-foreground">{tile.label}</dt>
                <dd className="text-xl font-semibold">{tile.text}</dd>
              </div>
            ))}
          </dl>
        </Section>
      ) : null}
      {holdings ? (
        <Section name="holdings" title={payload.header.title}>
          <DataTable
            columns={columnsOf(holdings.columns)}
            rows={holdings.rows.map((row) => rowOf(row.cells))}
          />
          {holdings.volatility_note ? (
            <p className="text-xs text-muted-foreground" data-volatility-note>
              {holdings.volatility_note}
            </p>
          ) : null}
          <Accordion type="multiple" data-moat>
            {holdings.rows.map((row, i) => (
              <AccordionItem key={`${i}-${row.cells[0].text}`} value={`moat-${i}`}>
                <AccordionTrigger>{row.cells[0].text}</AccordionTrigger>
                <AccordionContent>
                  <ul className="list-disc pl-4">
                    {row.moat.map((line, j) => (
                      <li key={`${j}-${line}`}>{line}</li>
                    ))}
                  </ul>
                </AccordionContent>
              </AccordionItem>
            ))}
          </Accordion>
          {payload.unvalued ? <p data-unvalued>{payload.unvalued}</p> : null}
        </Section>
      ) : null}
      <Section name="watch_list" title={watch.title}>
        {watch.sentence ? <p>{watch.sentence}</p> : null}
        {watch.rows.length > 0 ? (
          <DataTable
            columns={columnsOf(watch.columns)}
            rows={watch.rows.map((row) => rowOf(row.cells))}
          />
        ) : null}
      </Section>
      {payload.notes.length > 0 ? (
        <Card className="min-w-0" data-block="notes">
          <CardContent className="flex min-w-0 gap-2 pt-4">
            <Info aria-hidden className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <div className="min-w-0 space-y-1">
              <CardNotes notes={payload.notes} gaps={[]} showGaps={false} />
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  )
}
