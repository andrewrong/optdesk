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
import type { CompaniesPageV2 } from '@/generated/companies_page.v2'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/companies_page'
import { CardNotes } from '../home/CardNotes'
import { StaleAlert } from '../home/StaleAlert'
export { validate as validateCompaniesPage } from '@/types/companies_page'
export type { Payload as CompaniesPagePayload } from '@/types/companies_page'

type Head = { numeric?: boolean; text: string }
type Cells = {
  text: string
  tone?: 'up' | 'down' | 'flat' | 'none'
  price_state?: string
  hand_price?: string
}[]
type Gap = { reason: string; text: string }
const isGap = (v: object): v is Gap => 'reason' in v

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
  Object.fromEntries(
    cells.map((cell, i) => [
      `c${i}`,
      {
        text: cell.text,
        tone: cell.tone,
        // The price-line label is shown in a Badge; the own price is shown under the band. Both are Python's text.
        source: cell.price_state,
        note: cell.hand_price ? `hand price: ${cell.hand_price}` : undefined,
      },
    ]),
  )
const figureCell = (v: { text: string } | Gap | string) =>
  typeof v === 'string' ? { text: v } : v

function V2Blocks({ payload }: { payload: CompaniesPageV2 }) {
  const { core, diagnosis, questions } = payload
  return (
    <>
      <Section name="core" title="Core">
        {isGap(core) ? (
          <p data-gap>{core.text}</p>
        ) : (
          <>
            <p className="text-xs text-muted-foreground">{core.basis_note}</p>
            <p>
              {core.low_text} – {core.high_text} · {core.treasury.text}
              {' · '}
              {core.jp_rate.text}
            </p>
            <DataTable
              columns={[
                { id: 'symbol', label: 'Company' },
                { id: 'price', label: 'Cost', align: 'right' },
                { id: 'assumption', label: 'Case' },
                { id: 'ret', label: 'Return', align: 'right' },
              ]}
              rows={core.rows.flatMap((row) => {
                const head = { symbol: { text: row.symbol }, price: { text: row.price_text } }
                if (isGap(row.returns)) {
                  return [{ ...head, assumption: { text: '' }, ret: row.returns }]
                }
                return row.returns.map((r) => ({
                  ...head,
                  assumption: { text: r.assumption },
                  ret: { text: r.text },
                }))
              })}
            />
            {core.rows.some((r) => r.flag) ? (
              <ul className="list-disc pl-4">
                {core.rows.flatMap((r) =>
                  r.flag ? [<li key={r.symbol}>{`${r.symbol}: ${r.flag}`}</li>] : [],
                )}
              </ul>
            ) : null}
            <ul className="list-disc pl-4">
              {core.legend.map((line, i) => (
                <li key={`${i}-${line}`}>{line}</li>
              ))}
            </ul>
            <p className="text-xs text-muted-foreground">{core.snapshot_text}</p>
          </>
        )}
      </Section>
      <Section name="diagnosis" title="Diagnosis">
        {isGap(diagnosis) ? (
          <p data-gap>{diagnosis.text}</p>
        ) : (
          <>
            <p className="text-xs text-muted-foreground">{diagnosis.basis_note}</p>
            {diagnosis.groups.map((group) => {
              const columns = group.columns.map((label, i) => ({ id: `c${i}`, label }))
              const rows = group.rows.map((r) => {
                if (isGap(r)) return { c0: r }
                const cells = [r.symbol, r.latest_fy, r.latest, r.rank_text]
                return Object.fromEntries(
                  cells.map((text, i) => [
                    `c${i}`,
                    { text, note: i === 0 ? `${r.extra} · ${r.usual}` : undefined },
                  ]),
                )
              })
              return (
                <div key={group.key} data-group={group.key} className="space-y-2">
                  <h3 className="font-medium">{group.title}</h3>
                  <DataTable columns={columns} rows={rows} />
                  <ul className="list-disc pl-4">
                    {group.legend.map((line, i) => (
                      <li key={`${i}-${line}`}>{line}</li>
                    ))}
                  </ul>
                </div>
              )
            })}
          </>
        )}
      </Section>
      <Section name="questions" title="Questions">
        {isGap(questions) ? (
          <p data-gap>{questions.text}</p>
        ) : (
          <>
            <p className="text-xs text-muted-foreground">{questions.basis_note}</p>
            <DataTable
              columns={['Company', 'Q1', 'Q2', 'Q3', 'Q4', 'Q5'].map((label, i) => ({
                id: `c${i}`,
                label,
              }))}
              rows={questions.rows.map((row) => ({
                c0: { text: row.symbol },
                c1: figureCell(row.q1),
                c2: figureCell(row.q2),
                c3: figureCell(row.q3),
                c4: figureCell(row.q4),
                c5: figureCell(row.fifth),
              }))}
            />
          </>
        )}
      </Section>
    </>
  )
}

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
      {'core' in payload ? <V2Blocks payload={payload} /> : null}
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

export const registration = {
  schema: 'companies_page',
  page: page<Payload>(validate, CompaniesPage),
}
