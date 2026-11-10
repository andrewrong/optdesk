// The option notices page (Decision 10 of docs/design/option-notices.md): a header card with the data day and the
// stale label, one card per alias in payload order, the page gap cause once. Every visible string is a payload field;
// this file counts, sorts, compares and words nothing.
import { TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { AliasBlock } from '@/pages/notices/AliasBlock'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/option_notices_page'

export { validate as validateOptionNoticesPage } from '@/types/option_notices_page'
export type { Payload as OptionNoticesPayload } from '@/types/option_notices_page'

export function OptionNoticesPage({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-4" data-page="option-notices">
      <Card data-header>
        <CardHeader>
          <CardTitle>
            <h1 className="text-2xl font-semibold">{payload.title}</h1>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <Badge data-day>{payload.day}</Badge>
          {payload.stale ? (
            <Alert variant={payload.stale.is_stale ? 'destructive' : 'default'} data-stale>
              <TriangleAlert aria-hidden />
              <AlertDescription>{payload.stale.text}</AlertDescription>
            </Alert>
          ) : null}
          {payload.gaps.map((gap, index) => (
            <Alert key={`${index}-${gap.reason}`} data-page-gap={gap.reason}>
              <AlertDescription>{gap.text}</AlertDescription>
            </Alert>
          ))}
        </CardContent>
      </Card>
      {payload.accounts.map((block) => (
        <AliasBlock key={block.alias} block={block} />
      ))}
      {payload.notes.map((note, index) => (
        <p key={`${index}-${note}`} className="text-xs text-muted-foreground" data-footnote>
          {note}
        </p>
      ))}
    </div>
  )
}

export const registration = { schema: 'option_notices_page', page: page<Payload>(validate, OptionNoticesPage) }
