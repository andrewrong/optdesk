// The company data page (docs/design/data-collection.md, Decisions 8 to 12): the grid of companies, the legend and the
// link to data health. Read-only: no form and no button. Every visible value is a payload field; nothing is computed here.
import { Info } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/company_data_page'
import { CompanyTable } from './CompanyTable'

export function CompanyDataPage({ payload }: { payload: Payload }) {
  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>
            <h1 className="text-xl font-semibold">Company data</h1>
          </CardTitle>
          <a href={payload.data_health_path} className="text-sm underline underline-offset-4">
            Data health
          </a>
        </CardHeader>
        <CardContent className="space-y-3">
          {payload.gaps.map((g, i) => (
            <Alert key={`${i}-${g.reason}`} data-gap={g.reason}>
              <AlertDescription>{g.text}</AlertDescription>
            </Alert>
          ))}
          <CompanyTable columns={payload.columns} companies={payload.companies} />
        </CardContent>
      </Card>
      {payload.glossary.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Legend</CardTitle>
          </CardHeader>
          <CardContent>
            <dl data-legend className="space-y-2 text-sm">
              {payload.glossary.map((g) => (
                <div key={g.term} data-term={g.term} className="flex items-start gap-2">
                  <Info aria-hidden className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                  <div>
                    <dt className="inline font-medium">{g.term}</dt> <dd className="inline">{g.text}</dd>
                  </div>
                </div>
              ))}
            </dl>
          </CardContent>
        </Card>
      ) : null}
    </div>
  )
}

export const registration = { schema: 'company_data_page', page: page<Payload>(validate, CompanyDataPage) }
