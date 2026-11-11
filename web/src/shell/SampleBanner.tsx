import { FlaskConical } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'

// The one banner of a sample page (Decision 4 of docs/design/unified-site.md): a shell literal, never a payload field.
export function SampleBanner() {
  return (
    <Alert role="status">
      <FlaskConical aria-hidden />
      <AlertDescription>Sample data, not live data</AlertDescription>
    </Alert>
  )
}
