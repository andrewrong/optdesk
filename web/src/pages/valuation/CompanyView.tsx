// The company view: the profile, the current review, the lines with the zone and every true flag, the price band
// and the history. Every value is a payload text.
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Payload } from '@/types/valuation_page'
import { PriceBand } from './PriceBand'
import { DetailView, ReviewHistory } from './ReviewHistory'

type Lines = NonNullable<Payload['lines']>
type Flags = NonNullable<Lines['flags']>

const FLAG_TEXT: Record<keyof Flags, string> = {
  at_heavy: 'At heavy point',
  in_buy: 'In buy zone',
  in_fair: 'In fair zone',
  at_trim: 'At trim line',
  at_clear: 'At clear line',
  at_core: 'At core line',
}

const NOTE_TEXT: Record<Lines['zone_note'], string> = {
  '': '',
  no_close: 'No close price is stored, so there is no zone.',
  no_zone_market: 'This market has no price source, so there is no zone.',
}

// Every flag that is true is a mark of its own; none is dropped for another.
export function FlagBadges({ flags }: { flags: Flags | null }) {
  if (flags === null) return null
  return (
    <>
      {(Object.keys(FLAG_TEXT) as (keyof Flags)[])
        .filter((key) => flags[key])
        .map((key) => (
          <Badge key={key} variant="outline" data-flag={key}>
            {FLAG_TEXT[key]}
          </Badge>
        ))}
    </>
  )
}

export function LinesView({ lines }: { lines: Lines }) {
  const prices: [string, string][] = [
    ['C', lines.c],
    ['R', lines.r],
    ['Heavy', lines.heavy],
    ['Buy zone', `${lines.buy_low} - ${lines.buy_high}`],
    ['Fair zone', `${lines.fair_low} - ${lines.fair_high}`],
    ['Trim', lines.trim],
    ['Clear', lines.clear],
    ['Core', lines.core],
    ['M', lines.m],
    ['W', lines.w],
    ['H', lines.h],
    ['S', lines.s],
  ]
  return (
    <div data-lines className="space-y-2">
      <dl className="grid grid-cols-2 gap-x-6 gap-y-1 sm:grid-cols-4">
        {prices.map(([label, value]) => (
          <div key={label}>
            <dt className="text-sm text-muted-foreground">{label}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
      <div className="flex flex-wrap items-center gap-1">
        {lines.zone !== '' ? <Badge data-zone={lines.zone}>{lines.zone}</Badge> : null}
        <FlagBadges flags={lines.flags} />
      </div>
      {lines.zone === '' && lines.zone_note !== '' ? <p className="text-sm">{NOTE_TEXT[lines.zone_note]}</p> : null}
    </div>
  )
}

export function CompanyView({ payload }: { payload: Payload }) {
  const { company, current, lines, band, close, history } = payload
  if (company === null) return null
  return (
    <div className="space-y-6" data-view="company">
      <Card>
        <CardHeader>
          <CardTitle>
            {company.name} <span className="text-muted-foreground">{company.company_key}</span>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <p>
            {company.symbol} {company.currency}, R market {company.r_market}, pool {company.pool} {company.pool_date}
          </p>
          {company.note !== '' ? <p>{company.note}</p> : null}
          {close ? (
            <p>
              Close {close.value} on {close.day}
            </p>
          ) : null}
          {payload.form ? (
            <Button asChild size="sm">
              <a href={`/valuation-form?company=${encodeURIComponent(company.company_key)}`}>Enter a valuation</a>
            </Button>
          ) : null}
        </CardContent>
      </Card>
      {lines ? (
        <Card>
          <CardHeader>
            <CardTitle>Lines</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <LinesView lines={lines} />
            {band ? <PriceBand band={band} /> : null}
          </CardContent>
        </Card>
      ) : null}
      {current ? (
        <Card>
          <CardHeader>
            <CardTitle>
              Current valuation: V {current.v}, P {current.p}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <p className="text-sm">
              {current.type} {current.version_date} {current.status} {current.trigger} {current.change_memo}
            </p>
            <DetailView detail={current.detail} />
          </CardContent>
        </Card>
      ) : null}
      {history ? (
        <Card>
          <CardHeader>
            <CardTitle>History</CardTitle>
          </CardHeader>
          <CardContent>
            <ReviewHistory history={history} />
          </CardContent>
        </Card>
      ) : null}
    </div>
  )
}
