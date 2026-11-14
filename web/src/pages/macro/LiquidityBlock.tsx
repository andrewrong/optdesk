// The liquidity block of the macro page (docs/design/liquidity-view.md Decision 10): every string is sent by
// Python; this file formats, parses and compares nothing, and chooses no colour from a value.
import { Info } from 'lucide-react'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { MacroPageV2 } from '@/types/macro_page'

type Gap = { reason: string; text: string }

function GapText({ gap }: { gap: Gap }) {
  return (
    <span className="text-muted-foreground" data-liquidity-gap>
      {gap.text}
    </span>
  )
}

export function LiquidityBlock({
  title,
  liquidity,
}: {
  title: string
  liquidity: MacroPageV2['liquidity']
}) {
  return (
    <Card data-liquidity>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {'reason' in liquidity ? null : (
          <CardDescription>{liquidity.background_text}</CardDescription>
        )}
      </CardHeader>
      <CardContent className="space-y-3">
        {'reason' in liquidity ? (
          <Alert data-liquidity-gap>
            <Info aria-hidden />
            <AlertDescription>{liquidity.text}</AlertDescription>
          </Alert>
        ) : (
          <>
            <ul className="space-y-2">
              {liquidity.lines.map((line) => (
                <li key={line.key} data-liquidity-line={line.key}>
                  <div className="text-xs text-muted-foreground">{line.label}</div>
                  <div>
                    {typeof line.sentence === 'string' ? (
                      line.sentence
                    ) : (
                      <GapText gap={line.sentence} />
                    )}
                  </div>
                  <div className="text-xs text-muted-foreground">{line.source_text}</div>
                </li>
              ))}
            </ul>
            <Accordion type="multiple" data-liquidity-taps>
              {liquidity.taps.map((tap) => (
                <AccordionItem key={tap.key} value={tap.key} data-liquidity-tap={tap.key}>
                  <AccordionTrigger>{tap.label}</AccordionTrigger>
                  <AccordionContent className="space-y-2">
                    <ul className="space-y-1">
                      {tap.rows.map((row, i) => (
                        <li key={i} className="flex flex-wrap justify-between gap-2">
                          <span>{row.label}</span>
                          <span>
                            {typeof row.text === 'string' ? row.text : <GapText gap={row.text} />}
                          </span>
                          {row.data_day !== null && (
                            <span className="text-xs text-muted-foreground">{row.data_day}</span>
                          )}
                        </li>
                      ))}
                    </ul>
                    {tap.note !== null && (
                      <p className="text-xs text-muted-foreground" data-liquidity-note>
                        {tap.note}
                      </p>
                    )}
                  </AccordionContent>
                </AccordionItem>
              ))}
            </Accordion>
          </>
        )}
      </CardContent>
    </Card>
  )
}
