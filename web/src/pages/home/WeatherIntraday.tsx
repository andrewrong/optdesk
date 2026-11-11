// The intraday estimate section of the market weather card (issue #1224): the intraday gauge beside the close gauge. Every sentence
// and figure is a field of the `market_weather` payload, written by Python. The one comparison made here is the
// clock against `expires_at`: from then on the section is hidden and `expired_text` stands in its place. While the
// feature is on (`intraday` or `closed` in the payload) the page is read again every minute, so a new reading, the
// removal after the close and a closed market arrive without the owner touching the page.
import { Clock } from 'lucide-react'
import { motion } from 'motion/react'
import { useEffect } from 'react'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Payload } from '@/types/market_weather'
import { useClock } from './StaleAlert'

const REREAD_MS = 60_000

type Intraday = NonNullable<Payload['intraday']>

// The page is the only source of a new payload (no request is made from the browser, ADR 0034): reading it again
// is a reload of the address it was opened at, which the serve answers with the newest page.
export function useReread(active: boolean, reload: () => void = () => window.location.reload()) {
  useEffect(() => {
    if (!active) return
    const timer = setInterval(reload, REREAD_MS)
    return () => clearInterval(timer)
  }, [active, reload])
}

export function WeatherIntraday({
  intraday,
  closed,
  reload,
}: {
  intraday?: Payload['intraday']
  closed?: Payload['closed']
  reload?: () => void
}) {
  const now = useClock()
  useReread(intraday != null || closed != null, reload)
  if (intraday == null) {
    return closed ? (
      <p data-closed className="text-sm text-muted-foreground">
        {closed}
      </p>
    ) : null
  }
  if (now >= Date.parse(intraday.expires_at)) {
    return (
      <p data-intraday-expired className="text-sm text-muted-foreground">
        {intraday.expired_text}
      </p>
    )
  }
  return <Section intraday={intraday} />
}

function Section({ intraday }: { intraday: Intraday }) {
  const gauge = intraday.gauge
  return (
    <Card data-intraday={intraday.status} className="py-3">
      <CardHeader className="px-3">
        <CardTitle className="flex items-center gap-2 text-sm">
          <Clock aria-hidden className="size-4" />
          <span>{gauge?.label ?? intraday.title}</span>
          {gauge ? <span data-intraday-headline>{gauge.headline}</span> : null}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-2 px-3">
        <ul className="space-y-2">
          {intraday.rows.map((row) => (
            <li key={row.series} data-intraday-row={row.series} className="space-y-1">
              {/* A new figure fades in: the key is the value, so a changed value mounts again. */}
              <motion.div
                key={row.value}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.3 }}
                className="flex flex-wrap items-center gap-2"
              >
                <span>
                  {row.series} {row.value}
                </span>
                <Badge variant="secondary">{row.label}</Badge>
                <span className="text-xs text-muted-foreground">{row.as_of}</span>
                {row.detail ? (
                  <span className="text-xs" data-intraday-detail>
                    {row.detail}
                  </span>
                ) : null}
                {row.stale ? (
                  <Badge variant="destructive" data-intraday-stale>
                    {row.stale.text}
                  </Badge>
                ) : null}
              </motion.div>
            </li>
          ))}
        </ul>
        {gauge ? (
          <ul className="space-y-1" data-intraday-carried>
            <li className="text-sm">{gauge.ratio_text}</li>
            {gauge.carried.map((carried) => (
              <li key={carried.vote} className="flex items-center gap-2 text-sm">
                <span>{carried.vote}</span>
                <Badge variant="outline">
                  {carried.label} {carried.close_date.slice(5)}
                </Badge>
              </li>
            ))}
          </ul>
        ) : null}
        <p className="text-xs text-muted-foreground" data-intraday-as-of>
          {intraday.as_of}
        </p>
        {intraday.trend_line ? (
          <p className="text-xs text-muted-foreground" data-intraday-trend-line>
            {intraday.trend_line}
          </p>
        ) : null}
        {intraday.status_line ? (
          <p className="text-xs text-muted-foreground" data-intraday-status>
            {intraday.status_line}
          </p>
        ) : null}
        <p className="text-xs text-muted-foreground" data-intraday-footnote>
          {intraday.footnote}
        </p>
      </CardContent>
    </Card>
  )
}
