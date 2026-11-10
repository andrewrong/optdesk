// The stale label of a home card (ADR 0034, Staleness). Python sends `stale_after`, `generated_at` and the label
// text; the browser only compares them with the device clock, on first display, on a timer and on
// `visibilitychange`. A clock more than 5 minutes behind `generated_at` is flagged; a clock ahead is not.
import { useEffect, useState } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { StaleNotice } from './StaleNotice'

const REFRESH_MS = 60_000
const SKEW_MS = 5 * 60_000

export function useClock(): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const read = () => setNow(Date.now())
    const timer = setInterval(read, REFRESH_MS)
    document.addEventListener('visibilitychange', read)
    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', read)
    }
  }, [])
  return now
}

export function StaleAlert({
  stale,
  generatedAt,
}: {
  stale: { text: string; stale_after: string }
  generatedAt: string
}) {
  const now = useClock()
  const isStale = now > Date.parse(stale.stale_after)
  const skewed = Date.parse(generatedAt) - now > SKEW_MS
  return (
    <>
      <StaleNotice text={stale.text} isStale={isStale} />
      {skewed ? (
        <Alert variant="destructive" data-clock-skew>
          <AlertDescription>
            本机时钟与数据生成时间相差超过 5 分钟，请核对设备时间。
          </AlertDescription>
        </Alert>
      ) : null}
    </>
  )
}
