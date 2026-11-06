import { CircleCheck } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { pages } from '@/shell/pages'
import { TopNav } from '@/shell/TopNav'

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
// An unknown schema (or none) shows the hello page.
export function App({ payload }: { payload?: unknown }) {
  const schema =
    typeof payload === 'object' && payload !== null && 'schema' in payload
      ? payload.schema
      : undefined
  const entry =
    typeof schema === 'string' && Object.hasOwn(pages, schema) ? pages[schema] : undefined
  if (entry !== undefined) {
    const { validate, component: Component } = entry
    const result = validate(payload)
    if (!result.ok) throw new Error(result.gap.text)
    return (
      <>
        <TopNav />
        <main className="p-4">
          <Component payload={result.payload as never} />
        </main>
      </>
    )
  }
  return <Hello />
}
