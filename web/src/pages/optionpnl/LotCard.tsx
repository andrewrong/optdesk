// One short put lot: title, badges, the split lines, and the days and reconciliation rows folded in an accordion.
// A lot with a gap shows its reason in an Alert and no lines. Every text is a field of the payload.
import { CircleAlert } from 'lucide-react'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { Lot } from '@/generated/option_pnl_page.v1'
import { LinesTable } from './BookCard'

export function LotCard({ lot }: { lot: Lot }) {
  const main = [...lot.lines, ...(lot.iv_line ? [lot.iv_line] : []), ...(lot.end_line ? [lot.end_line] : [])]
  return (
    <Card data-lot={lot.id}>
      <CardHeader>
        <CardTitle className="text-base">{lot.title}</CardTitle>
        <CardDescription>{lot.summary}</CardDescription>
        {lot.badges.length > 0 ? (
          <div className="flex flex-wrap gap-2">
            {lot.badges.map((badge) => (
              <Badge key={badge} variant="secondary">
                {badge}
              </Badge>
            ))}
          </div>
        ) : null}
      </CardHeader>
      <CardContent className="space-y-3">
        {lot.gap ? (
          <Alert data-gap={lot.gap.reason}>
            <CircleAlert aria-hidden />
            <AlertDescription>{lot.gap.text}</AlertDescription>
          </Alert>
        ) : (
          <>
            {main.length > 0 ? <LinesTable lines={main} /> : null}
            <Accordion type="multiple">
              <AccordionItem value="days">
                <AccordionTrigger>{lot.days_line.label}</AccordionTrigger>
                <AccordionContent>
                  <LinesTable lines={[lot.days_line]} />
                </AccordionContent>
              </AccordionItem>
              {lot.recon.length > 0 ? (
                <AccordionItem value="recon">
                  <AccordionTrigger>{lot.recon[0]?.label}</AccordionTrigger>
                  <AccordionContent>
                    <LinesTable lines={lot.recon} />
                  </AccordionContent>
                </AccordionItem>
              ) : null}
            </Accordion>
          </>
        )}
      </CardContent>
    </Card>
  )
}
