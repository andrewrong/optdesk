// The outcome of each Japanese name with no candidate row. Every text is a payload field; the page counts and classifies
// nothing: only `failed` is drawn in the error colour, chosen by the outcome string.
import type { NameOutcome } from '@/types/option_candidates_page'

export function NameOutcomes({ outcomes }: { outcomes: NameOutcome[] }) {
  return (
    <ul className="space-y-1 text-sm" data-name-outcomes>
      {outcomes.map((item) => (
        <li
          key={item.name}
          className={item.outcome === 'failed' ? 'text-destructive' : 'text-muted-foreground'}
          data-name-outcome={item.outcome}
        >
          <span>{item.name}</span> <span>{item.text}</span> <span>{item.counts}</span>
        </li>
      ))}
    </ul>
  )
}
