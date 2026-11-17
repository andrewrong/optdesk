// A column header of a payload column: its label, then its gloss beside an Info icon. Both texts are the payload's.
import { Info } from 'lucide-react'

export function GlossHeader({ id, label, gloss }: { id: string; label: string; gloss: string }) {
  return (
    <div className="space-y-1">
      <div>{label}</div>
      {gloss !== '' ? (
        <div data-gloss={id} className="flex items-start gap-1 text-xs font-normal text-muted-foreground">
          <Info aria-hidden className="mt-0.5 size-3 shrink-0" />
          <span>{gloss}</span>
        </div>
      ) : null}
    </div>
  )
}
