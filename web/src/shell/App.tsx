import { CircleCheck } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { pages } from '@/shell/pages'
import { SampleBanner } from '@/shell/SampleBanner'
import { TopNav } from '@/shell/TopNav'
import { sampleOf } from '@/types/sample_page'

// The hello page of the web shell (ADR 0034 step 1b): a dev page, not a report.
function Hello() {
  return (
    <>
      <TopNav />
      <main className="p-4">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <CircleCheck aria-hidden="true" />
              <span>option-trader web shell, dev page, not a report</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant="secondary">_dev/hello</Badge>
          </CardContent>
        </Card>
      </main>
    </>
  )
}

// Dispatch on `payload.schema` through the page registry. The payload is validated whole before anything
// renders; one that does not validate throws, and the error boundary puts the fail-closed notice back.
// A `sample_page` payload shows the named page from the carried sample data under one banner.
// An unknown schema (or none) shows the hello page.
function schemaOf(payload: unknown): unknown {
  return typeof payload === 'object' && payload !== null && 'schema' in payload
    ? payload.schema
    : undefined
}

export function App({ payload }: { payload?: unknown }) {
  const sample = schemaOf(payload) === 'sample_page'
  const shown = sample ? sampleOf(payload) : payload
  const schema = schemaOf(shown)
  const entry =
    typeof schema === 'string' && Object.hasOwn(pages, schema) ? pages[schema] : undefined
  if (entry !== undefined) {
    const { validate, component: Component } = entry
    const result = validate(shown)
    if (!result.ok) throw new Error(result.gap.text)
    return (
      <>
        <TopNav />
        {sample ? <SampleBanner /> : null}
        <main className="p-4">
          <Component payload={result.payload as never} />
        </main>
      </>
    )
  }
  if (sample) throw new Error('sample_page: no page is registered for the sample')
  return <Hello />
}
