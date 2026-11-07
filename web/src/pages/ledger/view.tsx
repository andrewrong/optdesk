// Read-only view of page.ledger_page.v1: every string and figure is a field of the payload.
import { DataTable } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { StaleAlert } from '@/pages/home/StaleAlert'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/ledger_page'
import { columnsOf, figure, rowOf, withDetail } from './cells'
import { SectionTables } from './SectionTables'

export function View({ payload }: { payload: Payload }) {
  const { header, sections } = payload
  return (
    <div className="min-w-0 space-y-6" data-page="ledger-page">
      <header>
        <h1 className="text-xl font-semibold">{payload.title}</h1>
        <p className="text-sm text-muted-foreground">{header.title}</p>
        <p className="text-sm text-muted-foreground">{header.through}</p>
        {header.fx_note ? <p className="text-xs text-muted-foreground">{header.fx_note}</p> : null}
      </header>
      <StaleAlert stale={payload.stale} generatedAt={payload.generated_at} />
      <Card>
        <CardContent>
          <ul className="space-y-1" data-overview>
            {payload.overview.map((row) => (
              <li key={row.key} className="flex justify-between gap-4">
                <span>{row.title}</span>
                <span>
                  {row.total ? <>{figure(row.total).text} · </> : null}
                  {row.count}
                </span>
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>
      {payload.nothing_closed ? (
        <Alert data-nothing-closed>
          <AlertDescription>{payload.nothing_closed}</AlertDescription>
        </Alert>
      ) : null}
      {sections[0] ? (
        <Tabs defaultValue={sections[0].key}>
          <TabsList>
            {sections.map((section) => (
              <TabsTrigger key={section.key} value={section.key}>
                {section.title}
              </TabsTrigger>
            ))}
          </TabsList>
          {sections.map((section) => (
            <TabsContent
              key={section.key}
              value={section.key}
              forceMount
              className="data-[state=inactive]:hidden"
            >
              <SectionTables section={section} />
            </TabsContent>
          ))}
        </Tabs>
      ) : null}
      {payload.shares.map((share) => (
        <Card key={share.key} data-shares={share.key}>
          <CardHeader>
            <CardTitle>{share.title}</CardTitle>
            {share.lead ? <CardDescription>{share.lead}</CardDescription> : null}
          </CardHeader>
          <CardContent className="space-y-3">
            {share.note ? <p className="text-sm">{share.note}</p> : null}
            {share.rows.length > 0 ? (
              <DataTable
                columns={columnsOf([], 3)}
                rows={share.rows.map((row) => rowOf(row.cells.map(figure)))}
              />
            ) : null}
            {share.sales.length > 0 ? (
              <DataTable
                columns={columnsOf([], 4)}
                rows={share.sales.map((sale) => rowOf(withDetail(sale.cells, sale.detail)))}
              />
            ) : null}
          </CardContent>
        </Card>
      ))}
      {payload.notes.map((note) => (
        <p key={note.term} className="text-xs text-muted-foreground">
          <b>{note.term}</b> {note.text}
        </p>
      ))}
      {payload.footer.map((line, index) => (
        <p key={`${index}-${line}`} className="text-xs text-muted-foreground">
          {line}
        </p>
      ))}
      {payload.gaps.length > 0 ? (
        <Alert data-gaps>
          <AlertDescription>
            <ul className="list-disc pl-4">
              {payload.gaps.map((gap, index) => (
                <li key={`${index}-${gap.text}`}>{gap.text}</li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      ) : null}
    </div>
  )
}

export const registration = { schema: 'ledger_page', page: page(validate, View) }
