// The PE and PB block of the companies page (docs/design/company-pepb.md, Decision 7). Every sentence and figure
// is Python's text; the history points are drawn as given, and nothing is sorted, summed or rounded here.
import { Info } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Chart } from '@/charts/Chart'
import type { Gap, PepbBlock as Block, PepbLine, PepbRow } from '@/generated/companies_page.v2'

const isGap = (v: object): v is Gap => 'reason' in v

function GapLine({ gap }: { gap: Gap }) {
  return (
    <Alert data-gap>
      <AlertDescription>{gap.text}</AlertDescription>
    </Alert>
  )
}

function Line({ label, line }: { label: string; line: PepbLine }) {
  const spec = {
    kind: 'line' as const,
    categories: line.history.map((p) => p.day),
    series: [{ name: label, points: line.history.map((p) => ({ value: p.value, text: p.text })) }],
  }
  return (
    <div className="min-w-0 space-y-2" data-line={label}>
      <h4 className="font-medium">{label}</h4>
      <p>{line.text}</p>
      <p>
        {!isGap(line.percentile)
          ? `percentile ${line.percentile.text}`
          : line.percentile.text}
      </p>
      <ul className="list-disc pl-4">
        {line.at_percentile.map((row) => (
          <li key={row.percentile}>{row.text}</li>
        ))}
      </ul>
      <p className="text-xs text-muted-foreground">
        shares {line.shares}, filed {line.shares_filed}
      </p>
      {line.history.length > 0 ? (
        <Chart spec={spec} ariaText={`${label} history, ${line.window_from} to ${line.day}`} name={`pepb-${label}`} />
      ) : null}
    </div>
  )
}

function Ratio({ label, value }: { label: string; value: PepbLine | Gap }) {
  return isGap(value) ? <GapLine gap={value} /> : <Line label={label} line={value} />
}

function Company({ row }: { row: PepbRow }) {
  return (
    <Card className="min-w-0" data-company={row.symbol}>
      <CardHeader>
        <CardTitle>{row.symbol}</CardTitle>
      </CardHeader>
      <CardContent className="min-w-0 space-y-3">
        <Ratio label="PE" value={row.pe} />
        <Ratio label="PB" value={row.pb} />
        <Alert className="text-muted-foreground" data-forward>
          <Info aria-hidden />
          <AlertDescription>{row.forward.text}</AlertDescription>
        </Alert>
      </CardContent>
    </Card>
  )
}

export function PepbBlock({ pepb }: { pepb: Block | Gap }) {
  return (
    <Card className="min-w-0" data-block="pepb">
      <CardHeader>
        <CardTitle>PE and PB</CardTitle>
      </CardHeader>
      <CardContent className="min-w-0 space-y-3">
        {isGap(pepb) ? (
          <GapLine gap={pepb} />
        ) : (
          <>
            <p className="text-xs text-muted-foreground">{pepb.basis_note}</p>
            {pepb.rows.map((row) => (
              <Company key={row.symbol} row={row} />
            ))}
          </>
        )}
      </CardContent>
    </Card>
  )
}
