// The 期权概览 card of the home page (options_summary v2). Every visible string is a `text`, `label` or `caption`
// field of the payload; this file formats and computes nothing. A Gap figure shows its text, never a zero.
import { Chart } from '@/charts/Chart'
import type { ChartSpec } from '@/charts/spec'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/options_summary'
import { CardNotes } from './CardNotes'
import { Info } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'

type Tiles = Payload['card']['tiles']
const TILE_KEYS = [
  'sold_count_month',
  'premium_month_usd',
  'open_premium_usd',
  'max_loss_usd',
] as const satisfies readonly (keyof Tiles)[]

type Tile = Tiles[(typeof TILE_KEYS)[number]]

function TileBlock({ tileKey, tile }: { tileKey: string; tile: Tile }) {
  const gap = 'reason' in tile.figure
  return (
    <div className="space-y-1" data-tile={tileKey}>
      <div className="text-xs text-muted-foreground" data-tile-label>
        {tile.label}
      </div>
      {gap ? (
        <Badge variant="secondary" className="h-auto whitespace-normal text-left" data-tile-gap>
          {tile.figure.text}
        </Badge>
      ) : (
        <div className="text-2xl font-semibold" data-tile-figure>
          {tile.figure.text}
        </div>
      )}
      {tile.caption ? (
        <div className="text-xs text-muted-foreground" data-tile-caption>
          {tile.caption}
        </div>
      ) : null}
    </div>
  )
}

export function OptionsSummaryCard({ payload }: { payload: Payload }) {
  const { card } = payload
  const spec: ChartSpec = {
    kind: 'bar',
    categories: card.premium_series_6m.map((point) => point.label),
    series: [
      {
        name: card.premium_series_text,
        points: card.premium_series_6m.map((point) =>
          'reason' in point.premium
            ? { value: null, text: point.premium.text }
            : { value: point.premium.value, text: point.premium.text },
        ),
      },
    ],
  }
  return (
    <Card data-card="options-summary">
      <CardHeader>
        <CardTitle>{card.title}</CardTitle>
        <CardDescription>{card.say ?? card.subtitle}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <Alert data-stale-label>
          <Info aria-hidden />
          <AlertDescription>{card.stale.text}</AlertDescription>
        </Alert>
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4" data-tiles>
          {TILE_KEYS.map((key) => (
            <TileBlock key={key} tileKey={key} tile={card.tiles[key]} />
          ))}
        </div>
        <div className="space-y-1" data-premium-series>
          <div className="text-sm font-medium" data-series-text>
            {card.premium_series_text}
          </div>
          <Chart spec={spec} ariaText={card.premium_series_text} name="options-premium-6m" />
          <div className="text-xs text-muted-foreground" data-series-caption>
            {card.premium_series_caption}
          </div>
        </div>
        <CardNotes notes={card.notes} gaps={card.gaps} />
      </CardContent>
    </Card>
  )
}

export const registration = {
  schema: 'options_summary',
  page: page(validate, OptionsSummaryCard),
}
