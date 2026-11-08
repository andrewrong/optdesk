// Board 14: the price lines the owner keeps. Every text is a field of the payload; every form is a plain post to the
// constant path with the hidden `expires` and `mac` of the payload (the precedent of settings_page). Nothing is
// computed, compared or formatted here, and the view fetches nothing.
import { Info } from 'lucide-react'
import type { ReactNode } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { CardNotes } from '@/pages/home/CardNotes'
import { StaleAlert } from '@/pages/home/StaleAlert'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/price_lines_page'

// The native controls wear the border, ring and size classes of button.tsx.
const FIELD =
  'h-8 rounded-lg border border-border bg-background px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:border-input dark:bg-input/30'

type Market = Payload['rows'][number]['market']

function Tokens({ form }: { form: Payload['form'] }) {
  return (
    <>
      <input type="hidden" name="expires" value={form.expires} />
      <input type="hidden" name="mac" value={form.mac} />
    </>
  )
}

function MarketSelect({ markets, value }: { markets: readonly Market[]; value?: Market }) {
  return (
    <select name="market" defaultValue={value} className={FIELD} aria-label="market">
      {markets.map((market) => (
        <option key={market} value={market}>
          {market}
        </option>
      ))}
    </select>
  )
}

function Fields({
  markets,
  symbol,
  market,
  price,
  note,
}: {
  markets: readonly Market[]
  symbol?: string
  market?: Market
  price?: string
  note?: string
}) {
  return (
    <>
      <input name="symbol" defaultValue={symbol} className={FIELD} aria-label="code" />
      <MarketSelect markets={markets} value={market} />
      <input name="price" defaultValue={price} className={FIELD} aria-label="price" />
      <input name="note" defaultValue={note} className={FIELD} aria-label="note" />
    </>
  )
}

function Message({ children, destructive, ...rest }: { children: ReactNode; destructive?: boolean } & Record<string, boolean | string>) {
  return (
    <Alert variant={destructive ? 'destructive' : 'success'} {...rest}>
      <Info aria-hidden />
      <AlertDescription>{children}</AlertDescription>
    </Alert>
  )
}

function SavedRow({ row, form }: { row: Payload['rows'][number]; form: Payload['form'] }) {
  return (
    <TableRow data-row>
      <TableCell>{row.symbol}</TableCell>
      <TableCell>{row.market}</TableCell>
      <TableCell>{row.price !== null ? <span data-price>{row.price}</span> : null}</TableCell>
      <TableCell>{row.note}</TableCell>
      <TableCell>
        <form method="post" action={form.post_path} className="flex flex-wrap gap-2">
          <Tokens form={form} />
          <input type="hidden" name="symbol" value={row.symbol} />
          <input type="hidden" name="market" value={row.market} />
          <input name="price" defaultValue={row.price ?? ''} className={FIELD} aria-label="price" />
          <input name="note" defaultValue={row.note ?? ''} className={FIELD} aria-label="note" />
          <Button type="submit" name="action" value="save">
            保存
          </Button>
          <Button type="submit" name="action" value="delete" variant="destructive">
            删除
          </Button>
        </form>
      </TableCell>
    </TableRow>
  )
}

export function PriceLinesPage({ payload }: { payload: Payload }) {
  const { form, add } = payload
  return (
    <div className="min-w-0 space-y-6" data-page="price-lines-page">
      <header>
        <h1 className="text-xl font-semibold">{payload.title}</h1>
        <p className="text-sm text-muted-foreground">{payload.sentence}</p>
        <p className="text-sm text-muted-foreground">{payload.intro}</p>
      </header>
      {payload.stale && payload.stale_after ? (
        <StaleAlert
          stale={{ text: payload.stale.text, stale_after: payload.stale_after }}
          generatedAt={payload.generated_at}
        />
      ) : payload.stale ? (
        <Alert variant={payload.stale.is_stale ? 'destructive' : 'default'} data-stale={payload.stale.is_stale}>
          <AlertDescription>{payload.stale.text}</AlertDescription>
        </Alert>
      ) : null}
      <CardNotes notes={payload.notes} gaps={payload.gaps} />
      {payload.saved !== null ? <Message data-saved>{payload.saved}</Message> : null}
      {payload.error !== null ? (
        <Message destructive data-error>
          {payload.error}
        </Message>
      ) : null}
      <Card>
        <CardHeader>
          <CardTitle>价格线</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>代码</TableHead>
                <TableHead>市场</TableHead>
                <TableHead>价格</TableHead>
                <TableHead>备注</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {payload.rows.map((row) => (
                <SavedRow key={`${row.market}-${row.symbol}`} row={row} form={form} />
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>新增</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <p className="text-xs text-muted-foreground">{add.hint}</p>
          <form method="post" action={form.post_path} className="flex flex-wrap gap-2" data-add>
            <Tokens form={form} />
            <Fields markets={add.markets} />
            <Button type="submit" name="action" value="save">
              保存
            </Button>
          </form>
        </CardContent>
      </Card>
      {payload.unset.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>未设价格线的持仓</CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2" data-unset>
              {payload.unset.map((row) => (
                <li key={`${row.market}-${row.symbol}`}>
                  <form method="post" action={form.post_path} className="flex flex-wrap items-center gap-2">
                    <Tokens form={form} />
                    <Fields markets={add.markets} symbol={row.symbol} market={row.market} />
                    <Button type="submit" name="action" value="save">
                      保存
                    </Button>
                  </form>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      ) : null}
      <p className="text-xs text-muted-foreground">{payload.footnote}</p>
    </div>
  )
}

export const registration = {
  schema: 'price_lines_page',
  page: page<Payload>(validate, PriceLinesPage),
}
