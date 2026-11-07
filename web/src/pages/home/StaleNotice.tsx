// The stale notice of a v3 card: the producer sets `is_stale`, the browser compares no clock and shows the
// label text it was given.
import { Info } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'

export function StaleNotice({ text, isStale }: { text: string; isStale: boolean }) {
  return (
    <Alert variant={isStale ? 'destructive' : 'default'} data-stale={isStale}>
      <Info aria-hidden />
      <AlertDescription>{text}</AlertDescription>
    </Alert>
  )
}
