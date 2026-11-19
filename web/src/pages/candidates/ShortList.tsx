// The first block of the candidates page: the counts line, the one-sentence caveat, the filter chips, the dropped
// line and the grouped list. Every text is a payload field; this file computes, sorts and filters nothing.
import { TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { TooltipProvider } from '@/components/ui/tooltip'
import { Hint, ShortListRows } from '@/pages/candidates/ShortListRows'
import type { ShortList as ShortListT } from '@/generated/option_candidates_page.v1'

export function ShortList({ list }: { list: ShortListT | null | undefined }) {
  if (!list) return null
  return (
    <TooltipProvider>
      <section data-short-list className="min-w-0 space-y-3">
      <p data-short-counts className="text-sm">
        <span>{list.run_day}</span> <span>{list.counts}</span>
      </p>
      {list.caveat !== null ? (
        <Alert data-short-caveat-alert>
          <TriangleAlert aria-hidden />
          <AlertDescription data-short-caveat>{list.caveat}</AlertDescription>
        </Alert>
      ) : null}
      <div className="flex flex-wrap gap-2">
        {list.filters.map((f) => (
          <Hint key={f.id} text={f.hint}>
            <Badge variant="outline" data-filter={f.id}>
              {f.label} {f.value}
            </Badge>
          </Hint>
        ))}
      </div>
      {list.dropped !== null ? (
        <p data-dropped className="text-xs text-muted-foreground">
          {list.dropped}
          {list.dropped_items.length > 0 ? ': ' : ''}
          {list.dropped_items.map((d, i) => (
            <span key={d.code}>
              {i > 0 ? '; ' : ''}
              <Hint text={d.hint}>
                {d.label} {d.count}
              </Hint>
            </span>
          ))}
        </p>
      ) : null}
      <ShortListRows columns={list.columns} groups={list.groups} />
      </section>
    </TooltipProvider>
  )
}
