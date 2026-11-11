// The market weather card of the home page (issue #1209, board v2.2 board 1): the three votes with verdict, source and
// observed date, the source-mismatch lines and the footnote. Every sentence and figure is a field of the
// `market_weather` payload; this file formats and compares nothing. The verdict badge word is the fixed label of
// the three verdict codes.
import { CircleHelp, TriangleAlert } from 'lucide-react'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/market_weather'
import { StaleAlert } from './StaleAlert'
import { WeatherIntraday } from './WeatherIntraday'

const VERDICT = {
  ok: { word: 'Good', variant: 'secondary' },
  bad: { word: 'Bad', variant: 'destructive' },
  unknown: { word: 'Unknown', variant: 'outline' },
} as const

const COLUMNS: Column[] = [
  { id: 'day', label: 'Date' },
  { id: 'ibkr', label: 'IBKR', align: 'right' },
  { id: 'fred', label: 'FRED', align: 'right' },
  { id: 'diff', label: 'Diff', align: 'right' },
]

export function WeatherCard({ payload }: { payload: Payload }) {
  const rows = payload.mismatches.map((line) => ({
    day: { text: line.day, value: line.day } as Cell,
    ibkr: { text: line.ibkr.text, value: line.ibkr.value } as Cell,
    fred: { text: line.fred.text, value: line.fred.value } as Cell,
    diff: { text: line.diff.text, value: line.diff.value } as Cell,
  }))
  return (
    <Card data-card="market-weather">
      <CardHeader>
        <CardTitle data-headline>{payload.headline}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <WeatherIntraday intraday={payload.intraday} closed={payload.closed} />
        {payload.stale ? (
          <StaleAlert stale={payload.stale} generatedAt={payload.stale.data_time} />
        ) : null}
        <TooltipProvider>
          <ul className="space-y-2">
            {payload.rows.map((row) => (
              <li key={row.kind} data-vote={row.kind} className="flex flex-wrap items-center gap-2">
                <Badge variant={VERDICT[row.verdict].variant}>
                  {row.verdict === 'bad' ? <TriangleAlert aria-hidden /> : null}
                  {row.verdict === 'unknown' ? <CircleHelp aria-hidden /> : null}
                  {VERDICT[row.verdict].word}
                </Badge>
                <span>{row.figure_text}</span>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="outline" data-source>
                      {row.source}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{row.first_ibkr_date ?? row.source}</TooltipContent>
                </Tooltip>
                <span className="text-xs text-muted-foreground" data-observed>
                  {row.observed}
                </span>
              </li>
            ))}
          </ul>
        </TooltipProvider>
        {rows.length > 0 ? (
          <div data-mismatches>
            <DataTable columns={COLUMNS} rows={rows} />
          </div>
        ) : null}
        <Accordion type="single" collapsible defaultValue="footnote">
          <AccordionItem value="footnote">
            <AccordionTrigger>Footnotes</AccordionTrigger>
            <AccordionContent data-footnote>{payload.footnote}</AccordionContent>
          </AccordionItem>
        </Accordion>
      </CardContent>
    </Card>
  )
}

export const registration = {
  schema: 'market_weather',
  page: page<Payload>(validate, WeatherCard),
}
