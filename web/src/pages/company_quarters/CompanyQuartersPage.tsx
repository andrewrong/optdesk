// The company quarters page (docs/design/data-collection.md, Decisions 5 to 12): the chooser as links, the quarterly table,
// the notes and the gap lines. Every visible value is a payload field; this file computes nothing.
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/company_quarters_page'
import { QuartersTable } from './QuartersTable'

export function CompanyQuartersPage({ payload }: { payload: Payload }) {
  const company = payload.company
  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>
            <h1 className="text-xl font-semibold">Company quarters</h1>
          </CardTitle>
          {company !== null ? (
            <p data-company={company.key} className="text-sm text-muted-foreground">
              {company.symbol} {company.name} {company.issuer}
            </p>
          ) : null}
        </CardHeader>
        <CardContent className="space-y-3">
          {payload.gaps.map((g, i) => (
            <Alert key={`${i}-${g.reason}`} data-gap={g.reason}>
              <AlertDescription>{g.text}</AlertDescription>
            </Alert>
          ))}
          <ul data-choices className="flex flex-wrap gap-x-4 gap-y-1 text-sm">
            {payload.choices.map((c) => (
              <li key={c.key}>
                <a href={c.path} className="underline underline-offset-4">
                  {c.label}
                </a>
              </li>
            ))}
          </ul>
          {company !== null ? <QuartersTable company={company} columns={payload.columns} quarters={payload.quarters} /> : null}
          {payload.notes.map((n, i) => (
            <Alert key={`${i}-${n}`} data-note>
              <AlertDescription>{n}</AlertDescription>
            </Alert>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}

export const registration = { schema: 'company_quarters_page', page: page<Payload>(validate, CompanyQuartersPage) }
