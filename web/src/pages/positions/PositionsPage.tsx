// Issue #1205, board 6. Python supplies every cell, count, tone and sentence.
// The shared shadcn DataTable delegates user-started sorting to TanStack Table.
import { ChevronLeft } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { BarsSourceNote } from '@/components/BarsSourceNote'
import { ScrollHint } from '@/components/ScrollHint'
import { DataTable } from '@/components/data-table/data-table'
import { CardNotes } from '@/pages/home/CardNotes'
import { StaleAlert } from '@/pages/home/StaleAlert'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/positions_page'

export { validate as validatePositionsPage } from '@/types/positions_page'
export type { Payload as PositionsPagePayload } from '@/types/positions_page'

type View = Payload['views'][number]

type Table = View['stocks'] | View['options'] | NonNullable<View['trades']>

// The columns that hold words wrap in their cell; the figure columns keep their no-wrap.
const WRAP = new Set(['name', 'role', 'code', 'account', 'action', 'instrument'])

function PositionTable({
  table,
  name,
  structuresKey,
}: {
  table: Table
  name: string
  structuresKey?: string
}) {
  return (
    <Card className="min-w-0" data-table={name}>
      <CardHeader>
        <CardTitle>{table.title}</CardTitle>
      </CardHeader>
      <CardContent className="min-w-0">
        {table.gap ? (
          <Alert data-table-gap>
            <AlertDescription>{table.gap.text}</AlertDescription>
          </Alert>
        ) : (
          <>
            <ScrollHint />
            <DataTable
              columns={table.columns.map((c) => (WRAP.has(c.id) ? { ...c, wrap: true } : c))}
              rows={table.rows.map((row) => ({ ...row }))}
            />
            {structuresKey ? (
              <Button asChild variant="link" size="sm">
                <a href={`/options/structures?account=${encodeURIComponent(structuresKey)}`}>
                  组合结构
                </a>
              </Button>
            ) : null}
          </>
        )}
      </CardContent>
    </Card>
  )
}

export function PositionsPage({ payload }: { payload: Payload }) {
  // Only the v4 payload carries the account links; older payloads have none.
  const accounts = 'accounts' in payload ? payload.accounts : undefined
  return (
    <div className="min-w-0 space-y-4" data-page="positions-page">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <Button asChild variant="link">
          <a href="/household" aria-label="/household">
            <ChevronLeft />
          </a>
        </Button>
      </div>
      {accounts ? (
        <nav className="min-w-0 flex flex-wrap gap-2" data-positions-accounts>
          {accounts.map((account) => (
            <Button key={account.key} asChild variant="link" size="sm">
              <a
                href={account.href}
                data-account={account.key}
                aria-current={account.current ? 'page' : undefined}
                className={account.current ? 'font-semibold' : undefined}
              >
                {account.label}
              </a>
            </Button>
          ))}
        </nav>
      ) : null}
      <StaleAlert stale={payload.stale} generatedAt={payload.generated_at} />
      <BarsSourceNote payload={payload} />
      <Tabs defaultValue={payload.views[0].key} className="min-w-0">
        <TabsList className="group-data-horizontal/tabs:h-auto w-full flex-wrap">
          {payload.views.map((view) => (
            <TabsTrigger key={view.key} value={view.key}>
              {view.label}
            </TabsTrigger>
          ))}
        </TabsList>
        {payload.views.map((view) => (
          <TabsContent key={view.key} value={view.key} className="min-w-0 space-y-4">
            <p data-sentence>{view.sentence}</p>
            <PositionTable table={view.stocks} name="stocks" />
            <PositionTable table={view.options} name="options" structuresKey={view.key} />
            {view.trades ? <PositionTable table={view.trades} name="trades" /> : null}
          </TabsContent>
        ))}
      </Tabs>
      <CardNotes notes={payload.notes} gaps={payload.gaps} />
    </div>
  )
}

export const registration = {
  schema: 'positions_page',
  page: page<Payload>(validate, PositionsPage),
}
