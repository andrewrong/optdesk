// The top block and the chosen-contract cards: every line is a field of the payload.
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { TONE_CLASS } from '@/components/data-table/data-table'
import type { Payload } from '@/types/stock_option_page'

export function TopBlock({ top }: { top: Payload['top'] }) {
  const lines = [top.quote, top.iv, top.earnings, top.trend, top.policy]
  const tone = top.day_tone && top.day_tone !== 'none' ? TONE_CLASS[top.day_tone] : undefined
  return (
    <Card data-block="top">
      <CardHeader>
        <CardTitle className="flex flex-wrap items-baseline gap-3">
          <span>{top.symbol}</span>
          {top.name ? <span className="text-muted-foreground">{top.name}</span> : null}
          {top.spot ? <span>{top.spot}</span> : null}
          {top.day_change ? <span className={tone}>{top.day_change}</span> : null}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-1 text-sm">
        {lines.map((line) => (line ? <p key={line}>{line}</p> : null))}
      </CardContent>
    </Card>
  )
}

export function ChosenCards({ cards }: { cards: Payload['cards'] }) {
  return (
    <div className="grid gap-4 md:grid-cols-2" data-block="cards">
      {cards.map((card) => (
        <Card key={card.key} data-chosen={card.chosen}>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              {card.title}
              {card.chosen ? <Badge variant="secondary">{card.yield_line}</Badge> : null}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            {card.chosen ? null : <p>{card.yield_line}</p>}
            <ul className="flex gap-4">
              {card.ladder.map((pair) => (
                <li key={pair.label}>
                  <span className="text-muted-foreground">{pair.label}</span> {pair.text}
                </li>
              ))}
            </ul>
            <p>{card.advice}</p>
            {card.amounts.map((amount) => (
              <p key={amount}>{amount}</p>
            ))}
            <p>{card.risk}</p>
            <p>{card.liquidity}</p>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
