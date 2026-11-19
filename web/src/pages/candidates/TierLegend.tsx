// The legend of the tier view: one bordered strip of four cells, divided by thin lines. Every text is a payload field.
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import type { TierView } from '@/generated/option_candidates_page.v1'

const pills: Record<string, 'success' | 'warn' | 'outline'> = { fits: 'success', check: 'warn', grey: 'outline' }

export function TierLegend({ items }: { items: TierView['legend'] }) {
  return (
    <Card data-legend className="py-0">
      <CardContent className="grid grid-cols-1 divide-y px-0 text-sm sm:grid-cols-4 sm:divide-x sm:divide-y-0">
        {items.map((item) => {
          const pill = pills[item.tier]
          return (
            <div key={item.tier} data-legend-item={item.tier} className="flex flex-col gap-1 p-3">
              {pill ? (
                <Badge variant={pill} className="h-auto w-fit whitespace-normal">
                  {item.title}
                </Badge>
              ) : (
                <span className="font-bold">{item.title}</span>
              )}
              <span className="text-muted-foreground">{item.text}</span>
            </div>
          )
        })}
      </CardContent>
    </Card>
  )
}
