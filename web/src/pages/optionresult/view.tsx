// The option result page: read-only. Every string and figure is a field of the payload; nothing is computed here.
import { Info } from 'lucide-react'
import { DataTable, type Column } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/option_result_page'

type Section = Payload['sections'][number]
type Series = Section['months']

const PAIR_COLUMNS: Column[] = [
  { id: 'label', label: 'Item' },
  { id: 'value', label: 'Value', wrap: true },
]
const POINT_COLUMNS: Column[] = [
  { id: 'label', label: 'Point' },
  { id: 'text', label: 'Figure' },
]

function SeriesBlock({ series }: { series: Series }) {
  return (
    <div className="space-y-2" data-series={series.title}>
      <h4 className="text-sm font-medium">{series.title}</h4>
      {series.points.length === 0 ? (
        <p className="text-sm text-muted-foreground">{series.empty}</p>
      ) : (
        <>
          <DataTable
            columns={POINT_COLUMNS}
            rows={series.points.map((point) => ({
              label: { text: point.label, value: point.label },
              text: { text: point.text, value: point.value, tone: point.muted ? 'flat' : 'none' },
            }))}
          />
          <p className="text-xs text-muted-foreground">{series.summary}</p>
        </>
      )}
    </div>
  )
}

function SectionCard({ section }: { section: Section }) {
  return (
    <Card data-section={section.key}>
      <CardHeader>
        <CardTitle>{section.heading}</CardTitle>
        <CardDescription>{section.lead}</CardDescription>
        <Badge variant="secondary" className="w-fit h-auto whitespace-normal">
          {section.scope}
        </Badge>
      </CardHeader>
      <CardContent className="space-y-4">
        <DataTable
          columns={PAIR_COLUMNS}
          rows={section.figures.map((pair) => ({
            label: { text: pair.label },
            value: { text: pair.value },
          }))}
        />
        {section.figure_notes.map((note) => (
          <p key={note} className="text-xs text-muted-foreground">
            {note}
          </p>
        ))}
        <SeriesBlock series={section.months} />
        <SeriesBlock series={section.running} />
        <Separator />
        <h4 className="text-sm font-medium">{section.endings_title}</h4>
        {section.endings_lines.map((line) => (
          <p key={line} className="text-sm">
            {line}
          </p>
        ))}
        <SeriesBlock series={section.endings} />
        {section.assignment ? <p className="text-sm">{section.assignment}</p> : null}
        {section.protection ? <p className="text-sm">{section.protection}</p> : null}
        <Separator />
        <h4 className="text-sm font-medium">{section.books_title}</h4>
        <p className="text-xs text-muted-foreground">{section.books_note}</p>
        {section.books_lines.map((line) => (
          <p key={line} className="text-sm">
            {line}
          </p>
        ))}
      </CardContent>
    </Card>
  )
}

export function OptionResultView({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-6" data-page="option-result-page">
      <h1 className="text-2xl font-semibold">{payload.title}</h1>
      {payload.stale ? (
        <Alert data-stale>
          <Info aria-hidden />
          <AlertDescription>{payload.stale.text}</AlertDescription>
        </Alert>
      ) : null}
      {payload.nothing ? (
        <Alert data-nothing>
          <AlertDescription>{payload.nothing}</AlertDescription>
        </Alert>
      ) : null}
      {payload.facts.length > 0 ? (
        <Card>
          <CardContent className="grid gap-2 pt-6 sm:grid-cols-2">
            {payload.facts.map((fact) => (
              <div key={fact.label}>
                <div className="text-xs text-muted-foreground">{fact.label}</div>
                <div className="text-sm">{fact.value}</div>
              </div>
            ))}
          </CardContent>
        </Card>
      ) : null}
      {payload.sections.map((section) => (
        <SectionCard key={section.key} section={section} />
      ))}
      {payload.reliability.map((block) => (
        <Card key={block.key}>
          <CardHeader>
            <CardTitle>{block.heading}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            {block.lines.map((line) => (
              <p key={line} className="text-sm">
                {line}
              </p>
            ))}
          </CardContent>
        </Card>
      ))}
      {payload.notes.length > 0 ? (
        <>
          <Separator />
          <ul className="space-y-1 text-xs text-muted-foreground">
            {payload.notes.map((note) => (
              <li key={note}>{note}</li>
            ))}
          </ul>
        </>
      ) : null}
    </div>
  )
}

export const registration = {
  schema: 'option_result_page',
  page: page<Payload>(validate, OptionResultView),
}
