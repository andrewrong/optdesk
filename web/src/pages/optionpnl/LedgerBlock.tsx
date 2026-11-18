// The ledger block of an account: a tab per view (by month, by company), a card per group, a lines table per
// book. Both views are in the payload; a tab only chooses which one is shown. Every text is a field of the
// payload; nothing is compared, summed, sorted or formatted here.
import { CircleAlert } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { Book, Group, Ledger } from '@/generated/option_pnl_page.v1'
import { LinesTable } from './BookCard'

function LedgerBook({ book }: { book: Book }) {
  return (
    <div className="space-y-3" data-ledger-book={book.id}>
      <p className="text-sm font-medium">
        {book.currency}
        <span className="ml-2 text-xs font-normal text-muted-foreground">{book.counts}</span>
      </p>
      <LinesTable lines={[book.headline]} />
      {book.left_out ? (
        <Alert data-ledger-left-out>
          <CircleAlert aria-hidden />
          <AlertDescription>{book.left_out}</AlertDescription>
        </Alert>
      ) : null}
      <LinesTable lines={book.lines} />
    </div>
  )
}

function GroupCard({ group }: { group: Group }) {
  return (
    <Card data-ledger-group={group.id}>
      <CardHeader>
        <CardTitle>{group.label}</CardTitle>
        <CardDescription>{group.summary}</CardDescription>
      </CardHeader>
      {group.books.length > 0 ? (
        <CardContent className="space-y-4">
          {group.books.map((book) => (
            <LedgerBook key={book.id} book={book} />
          ))}
        </CardContent>
      ) : null}
    </Card>
  )
}

export function LedgerBlock({ ledger }: { ledger?: Ledger | null }) {
  if (!ledger || ledger.views.length === 0) return null
  return (
    <section className="min-w-0 space-y-3" data-ledger>
      <div>
        <h2 className="text-lg font-semibold">{ledger.title}</h2>
        <p className="text-xs text-muted-foreground">{ledger.note}</p>
      </div>
      <Tabs defaultValue={ledger.views[0]!.id}>
        <TabsList>
          {ledger.views.map((view) => (
            <TabsTrigger key={view.id} value={view.id} data-tab={view.id}>
              {view.label}
            </TabsTrigger>
          ))}
        </TabsList>
        {ledger.views.map((view) => (
          <TabsContent key={view.id} value={view.id} className="space-y-4">
            {view.groups.map((group) => (
              <GroupCard key={group.id} group={group} />
            ))}
          </TabsContent>
        ))}
      </Tabs>
    </section>
  )
}
