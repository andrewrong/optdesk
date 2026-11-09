// The option tracking page (`/options/tracking`): a header card, then one section per account. Every text is a
// field of the payload; nothing is compared, summed, sorted or formatted here.
import { Info } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/option_tracking_page'
import { AccountSection } from './AccountSection'

export function OptionTrackingView({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-6" data-page="option-tracking-page">
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">{payload.title}</CardTitle>
          <CardDescription>{payload.as_of}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          <p className="text-sm">{payload.mark_note}</p>
          {payload.stale ? (
            <Alert variant={payload.stale.is_stale ? 'destructive' : 'default'} data-stale={payload.stale.is_stale}>
              <Info aria-hidden />
              <AlertDescription>{payload.stale.text}</AlertDescription>
            </Alert>
          ) : null}
          {payload.gaps.map((gap) => (
            <Alert key={gap.reason} data-gap={gap.reason}>
              <AlertDescription>{gap.text}</AlertDescription>
            </Alert>
          ))}
        </CardContent>
      </Card>
      {payload.accounts.map((account) => (
        <AccountSection key={account.alias} account={account} />
      ))}
      {payload.notes.length > 0 ? (
        <ul className="space-y-1 text-xs text-muted-foreground">
          {payload.notes.map((note) => (
            <li key={note}>{note}</li>
          ))}
        </ul>
      ) : null}
    </div>
  )
}

export const registration = { schema: 'option_tracking_page', page: page<Payload>(validate, OptionTrackingView) }
