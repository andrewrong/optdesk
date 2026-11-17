// The valuation pages (docs/design/valuation-structure.md, entry 8): the table of every name, the page of one
// company and the entry form, told apart by `view`. Every figure is a payload text; nothing is computed here.
import { TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/valuation_page'
import { CompanyView } from './CompanyView'
import { ReviewForm } from './ReviewForm'
import { ValuationTable } from './ValuationTable'

// The H1 names the view, not the nav label.
function title(payload: Payload): string {
  switch (payload.view) {
    case 'table':
      return 'Valuation table'
    case 'company':
      return payload.company?.name ?? 'Company'
    case 'form':
      return 'Enter a valuation'
  }
}

function Body({ payload }: { payload: Payload }) {
  switch (payload.view) {
    case 'table':
      return (
        <Card>
          <CardHeader>
            <CardTitle>Companies{payload.unvalued_count > 0 ? `, not entered: ${payload.unvalued_count}` : ''}</CardTitle>
          </CardHeader>
          <CardContent>
            <ValuationTable rows={payload.rows ?? []} />
          </CardContent>
        </Card>
      )
    case 'company':
      return <CompanyView payload={payload} />
    case 'form':
      return payload.form ? <ReviewForm payload={payload} form={payload.form} /> : null
  }
}

export function ValuationPage({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-6" data-page="valuation">
      <header>
        <h1 className="text-2xl font-semibold">{title(payload)}</h1>
        <p className="mt-2 flex gap-4 text-sm">
          <a href="/valuation" aria-current="page" className="font-medium">
            Table
          </a>
          <a href="/valuation-params" className="underline">
            Parameters
          </a>
        </p>
      </header>
      {payload.error !== '' ? (
        <Alert variant="destructive">
          <TriangleAlert />
          <AlertDescription>{payload.error}</AlertDescription>
        </Alert>
      ) : null}
      {payload.saved !== '' ? (
        <Alert>
          <AlertDescription>{payload.saved}</AlertDescription>
        </Alert>
      ) : null}
      {payload.gaps.map((gap, at) => (
        <Alert key={at}>
          <TriangleAlert />
          <AlertDescription>{gap.text}</AlertDescription>
        </Alert>
      ))}
      <Body payload={payload} />
    </div>
  )
}

export const registration = {
  schema: 'valuation_page',
  page: page<Payload>(validate, ValuationPage),
}
