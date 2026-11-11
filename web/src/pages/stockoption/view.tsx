// The stock option page: a read-only view of page.stock_option_page.v1. Every string and figure is a field of
// the payload; this file computes and formats nothing. The matrix toggle between yield and premium is view
// state inside MatrixTable: both texts are in each cell of the payload.
import { Info, TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { OptionQuoteList } from '@/components/option-quote-cell'
import { OptionRefresh } from '@/components/option-refresh'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/stock_option_page'
import { ChosenCards, TopBlock } from './Cards'
import { HeldBlock, PriceMaps } from './Maps'
import { ContractsTable, MatrixTable } from './Tables'

export function StockOptionView({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-6" data-page="stock-option-page">
      <header className="space-y-2">
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <OptionRefresh refresh={'refresh' in payload ? payload.refresh : undefined} />
      </header>
      {payload.stale ? (
        <Alert variant="destructive" data-stale>
          <TriangleAlert aria-hidden />
          <AlertDescription>{payload.stale.text}</AlertDescription>
        </Alert>
      ) : null}
      {payload.shared_gap ? (
        <Alert data-gap>
          <Info aria-hidden />
          <AlertDescription>{payload.shared_gap}</AlertDescription>
        </Alert>
      ) : null}
      <TopBlock top={payload.top} />
      {payload.no_card ? <p data-no-card>{payload.no_card}</p> : <ChosenCards cards={payload.cards} />}
      <MatrixTable matrix={payload.matrix} />
      <PriceMaps maps={payload.maps} />
      <ContractsTable contracts={payload.contracts} />
      {'refresh' in payload ? (
        <OptionQuoteList
          items={payload.contracts.rows.map((line, index) => ({
            id: String(index),
            label: line.expiry_text,
            quote: line.quote,
          }))}
        />
      ) : null}
      <HeldBlock held={payload.held} />
      {payload.notice ? (
        <Alert data-notice>
          <Info aria-hidden />
          <AlertDescription>{payload.notice}</AlertDescription>
        </Alert>
      ) : null}
      <footer className="text-xs text-muted-foreground">{payload.footer}</footer>
    </div>
  )
}

export const registration = {
  schema: 'stock_option_page',
  page: page<Payload>(validate, StockOptionView),
}
