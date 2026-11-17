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

// Static labels of the codes the profile and the review print; an unknown code prints as given.
const CURRENCY_TEXT: Record<string, string> = { USD: 'US dollar', JPY: 'Japanese yen', HKD: 'Hong Kong dollar', CNY: 'Chinese yuan' }
const R_MARKET_TEXT: Record<string, string> = { US: 'United States', JP: 'Japan', HKCN: 'Hong Kong and China' }
const POOL_TEXT: Record<string, string> = { in_pool: 'in pool', watch: 'watch', removed: 'removed' }
export const TYPE_TEXT: Record<string, string> = {
  first: 'First valuation',
  post_earnings: 'After earnings',
  event: 'Event',
  backfill: 'Backfill',
}
const label = (map: Record<string, string>, code: string) => map[code] ?? code

// One gloss per term, static text; the source of each is on its own line (docs/design/valuation-structure.md).
export const GLOSS: Record<string, string> = {
  C: 'C: fair price, V x R', // line 75
  R: 'R: the multiple of the valuation that sets the fair price (C = V x R)', // line 75
  Heavy: 'Heavy: the heavy point, C x (1 - H)', // line 99
  Trim: 'Trim: C x (1 + S)', // line 78
  Clear: 'Clear: V x 1.2', // line 78
  Core: 'Core: V x 1.5', // line 78
  M: 'M: safety margin', // line 61
  W: 'W: fair half width', // line 61
  H: 'H: heavy discount', // line 61
  S: 'S: trim premium', // line 61
  V: "V: the owner's valuation of one share", // line 9
  P: 'P: predictability, 1 to 5', // lines 9 and 99
  'Version date': 'Version date: the date this valuation is valid from', // issue #2654 item 4, no line of the doc
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
        {prices.map(([name, value]) => (
          <div key={name}>
            <dt className="text-sm text-muted-foreground">{name}</dt>
            <dd>{value}</dd>
            {GLOSS[name] ? <dd className="text-xs text-muted-foreground">{GLOSS[name]}</dd> : null}
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
          <p data-profile>
            {company.symbol}, {label(CURRENCY_TEXT, company.currency)}, R market: {label(R_MARKET_TEXT, company.r_market)}, pool:{' '}
            {label(POOL_TEXT, company.pool)}, {company.pool_date}
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
              {label(TYPE_TEXT, current.type)} {current.version_date} {current.status} {current.trigger} {current.change_memo}
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
