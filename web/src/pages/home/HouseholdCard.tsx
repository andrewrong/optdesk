// The Household overview card of the home page (ADR 0034 step 3c, board ab1). Every visible string is a `text` field of the
// payload; this file formats and computes nothing. The gain or loss colour comes from the payload's `tone` only.
import { ChevronRight, Info } from 'lucide-react'
import { TONE_CLASS } from '@/components/data-table/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { validate, type Payload } from '@/types/household_summary'
import { cn } from '@/lib/utils'
import { page } from '@/shell/page'
import { CardNotes } from './CardNotes'
import { DataTimesFold } from './DataTimesFold'
import { StaleAlert } from './StaleAlert'
import { TitleLink } from './OverviewCard'

type Tile = Payload['card']['tiles'][keyof Payload['card']['tiles']]

// The writer sorts JSON keys, so the payload order is not the board order: the tiles are shown in this order.
const TILE_ORDER = [
  'net_worth',
  'day_change',
  'month_to_date',
  'cash',
] as const satisfies readonly (keyof Payload['card']['tiles'])[]

function toneClass(item: object): string {
  const tone = 'tone' in item ? item.tone : undefined
  return typeof tone === 'string' && tone in TONE_CLASS
    ? TONE_CLASS[tone as keyof typeof TONE_CLASS]
    : ''
}

function Gapped({
  item,
  className,
}: {
  item: Tile['figure'] | Tile['caption']
  className: string
}) {
  if ('reason' in item) {
    return (
      <Badge variant="warning" className="h-auto whitespace-normal text-left" data-gap>
        {item.text}
      </Badge>
    )
  }
  return (
    <div className={cn(className, toneClass(item))} data-figure>
      {item.text}
    </div>
  )
}

function TileCard({ id, tile }: { id: string; tile: Tile }) {
  return (
    <Card size="sm" data-tile={id}>
      <CardHeader>
        <CardDescription className="flex items-center gap-1">
          {tile.label}
          {tile.explain && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon-xs" aria-label={tile.label} data-explain>
                  <Info />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{tile.explain}</TooltipContent>
            </Tooltip>
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-1">
        <Gapped item={tile.figure} className="text-2xl font-semibold" />
        <Gapped item={tile.caption} className="text-xs text-muted-foreground" />
      </CardContent>
    </Card>
  )
}

export function HouseholdCard({
  payload,
  showGaps = true,
  titleHref,
}: {
  payload: Payload
  showGaps?: boolean
  titleHref?: string
}) {
  const { card } = payload
  return (
    <Card data-card="household-summary">
      <CardHeader>
        <CardTitle>
          <TitleLink href={titleHref}>{card.title}</TitleLink>
        </CardTitle>
        <CardDescription>{card.subtitle}</CardDescription>
        <CardAction>
          <Button asChild variant="link">
            <a href="/household">
              Household page
              <ChevronRight />
            </a>
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-3">
        <StaleAlert stale={card.stale} generatedAt={payload.generated_at} />
        <TooltipProvider>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {TILE_ORDER.map((id) => (
              <TileCard key={id} id={id} tile={card.tiles[id]} />
            ))}
          </div>
        </TooltipProvider>
        <CardNotes notes={card.notes} gaps={card.gaps} showGaps={showGaps} />
        <DataTimesFold text={card.data_times_text} />
      </CardContent>
    </Card>
  )
}

export const registration = {
  schema: 'household_summary',
  page: page<Payload>(validate, HouseholdCard),
}
