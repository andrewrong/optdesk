// The checks of one dataset, one accordion item per check: rule id, English label, the Chinese sentence, the counts
// and the threshold used, as the builder sent them. Nothing is computed here.
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import type { Payload } from '@/types/base_data_page'
import { LevelBadge } from './LevelBadge'

type Check = Payload['groups'][number]['rows'][number]['checks'][number]

export function CheckList({ dataset, checks }: { dataset: string; checks: readonly Check[] }) {
  return (
    <Accordion type="multiple" data-checks={dataset} defaultValue={checks.map((c) => c.rule)}>
      {checks.map((c) => (
        <AccordionItem key={c.rule} value={c.rule} data-rule={c.rule}>
          <AccordionTrigger>
            <span className="flex items-center gap-2">
              <LevelBadge level={c.level} />
              <span className="font-mono text-xs">{c.rule}</span>
              <span>{c.label}</span>
            </span>
          </AccordionTrigger>
          <AccordionContent className="space-y-1 text-sm">
            <p className="text-muted-foreground">{c.hint_zh}</p>
            <p>
              rows examined {c.n}, bad {c.bad}
              {c.share !== '' ? `, share ${c.share}` : ''}
            </p>
            {c.warn_at !== '' || c.fail_at !== '' ? (
              <p>
                warn at {c.warn_at}, fail at {c.fail_at}
                {c.threshold_source !== '' ? <span className="text-muted-foreground"> ({c.threshold_source})</span> : null}
              </p>
            ) : null}
            {c.example !== '' ? <p>example: {c.example}</p> : null}
          </AccordionContent>
        </AccordionItem>
      ))}
    </Accordion>
  )
}
