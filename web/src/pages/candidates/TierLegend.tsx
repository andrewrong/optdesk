// The legend of the tier view: one card, one line per payload item. Every text is a payload field.
import { Card, CardContent } from '@/components/ui/card'
import type { TierView } from '@/generated/option_candidates_page.v1'

export function TierLegend({ items }: { items: TierView['legend'] }) {
  return (
    <Card data-legend>
      <CardContent className="grid gap-2 text-sm sm:grid-cols-2">
        {items.map((item) => (
          <p key={item.tier} data-legend-item={item.tier}>
            <span className="font-medium">{item.title}</span> <span className="text-muted-foreground">{item.text}</span>
          </p>
        ))}
      </CardContent>
    </Card>
  )
}
