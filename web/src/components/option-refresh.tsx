// The refresh button of the option pages (docs/design/option-open-pull.md Decision 20): a shadcn Button with the
// RefreshCw icon in the page header, disabled while a call runs and when `refresh.enabled` is false. The notice of
// the payload sits under it. The web computes nothing and keeps no timer, and the browser makes no network request
// by script (eslint `[network]`): the button is a plain HTML form that posts the symbols of the payload to
// `/options/refresh`, and the page that comes back carries the new JSON with its new notice.
import { Info, RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import type { Refresh } from '@/generated/option_page.v2'

export function OptionRefresh({ refresh }: { refresh: Refresh | undefined }) {
  const [running, setRunning] = useState(false)
  if (!refresh) return null
  return (
    <div className="space-y-2" data-refresh>
      <form method="post" action="/options/refresh" onSubmit={() => setRunning(true)}>
        {refresh.symbols.map((symbol) => (
          <input key={symbol} type="hidden" name="symbols" value={symbol} />
        ))}
        <Button
          type="submit"
          variant="outline"
          size="sm"
          disabled={!refresh.enabled || running}
          data-refresh-button
        >
          <RefreshCw aria-hidden className={running ? 'animate-spin' : undefined} />
          Refresh
        </Button>
      </form>
      {refresh.notice ? (
        <Alert data-refresh-notice>
          <Info aria-hidden />
          <AlertDescription>{refresh.notice}</AlertDescription>
        </Alert>
      ) : null}
    </div>
  )
}
