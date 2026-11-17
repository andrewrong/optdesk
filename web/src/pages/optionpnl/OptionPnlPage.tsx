// The options P&L page (`/options/pnl`): a header card, then per account its books and one card per lot.
// Every text is a field of the payload; nothing is compared, summed, sorted or formatted here.
import { CircleAlert, Info } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/option_pnl_page'
import { BookCard } from './BookCard'
import { LotCard } from './LotCard'

type Account = Payload['accounts'][number]

function AccountSection({ account }: { account: Account }) {
  return (
    <section className="min-w-0 space-y-4" data-account={account.alias}>
      <Card>
        <CardHeader>
          <CardTitle>{account.alias}</CardTitle>
          <CardDescription>{account.sample_note}</CardDescription>
        </CardHeader>
        {account.gap ? (
          <CardContent>
            <Alert data-gap={account.gap.reason}>
              <CircleAlert aria-hidden />
              <AlertDescription>{account.gap.text}</AlertDescription>
            </Alert>
          </CardContent>
        ) : null}
      </Card>
      {account.books.map((book) => (
        <BookCard key={book.id} book={book} />
      ))}
      {account.lots.map((lot) => (
        <LotCard key={lot.id} lot={lot} />
      ))}
    </section>
  )
}

export function OptionPnlView({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-6" data-page="option-pnl-page">
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">{payload.title}</CardTitle>
          <CardDescription>{payload.as_of}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          {payload.stale ? (
            <Alert variant={payload.stale.is_stale ? 'destructive' : 'default'} data-stale={payload.stale.is_stale}>
              <Info aria-hidden />
              <AlertDescription>{payload.stale.text}</AlertDescription>
            </Alert>
          ) : null}
          {payload.gaps.map((gap) => (
            <Alert key={gap.reason} data-gap={gap.reason}>
              <CircleAlert aria-hidden />
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

export const registration = { schema: 'option_pnl_page', page: page<Payload>(validate, OptionPnlView) }
