// The stale notice of a v3 card: the producer sets `is_stale`, the browser compares no clock and shows the
// label text it was given. Not stale: one grey small line. Stale: the destructive Alert with its icon.
import { Info } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'

export function StaleNotice({ text, isStale }: { text: string; isStale: boolean }) {
  if (!isStale) {
    return (
      <p className="text-sm text-muted-foreground" data-stale={false}>
        {text}
      </p>
    )
  }
  return (
    <Alert variant="destructive" data-stale={true}>
      <Info aria-hidden />
      <AlertDescription>{text}</AlertDescription>
    </Alert>
  )
}
