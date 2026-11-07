// The tiles, key levels, timeline, stale label and gap lines of the structure detail page.
import { Info } from 'lucide-react'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { Payload } from '@/types/structure_detail'

export function StaleLabel({ stale }: { stale: Payload['stale'] }) {
  return (
    <Alert data-stale-label>
      <Info aria-hidden />
      <AlertDescription>{stale.text}</AlertDescription>
    </Alert>
  )
}

export function GapLines({ gaps }: { gaps: Payload['gaps'] }) {
  return (
    <>
      {gaps.map((gap, index) => (
        <Alert key={`${index}-${gap.text}`} data-gap={gap.reason}>
          <Info aria-hidden />
          <AlertDescription>{gap.text}</AlertDescription>
        </Alert>
      ))}
    </>
  )
}

export function Tiles({ tiles }: { tiles: Payload['tiles'] }) {
  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
      {[tiles.pnl, tiles.max_gain, tiles.max_loss, tiles.break_even].map((tile) => (
        <Card key={tile.label} data-tile>
          <CardHeader>
            <CardDescription>{tile.label}</CardDescription>
            <CardTitle>{tile.figure.text}</CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">{tile.caption}</CardContent>
        </Card>
      ))}
    </div>
  )
}

export function KeyLevels({ title, levels }: { title: string; levels: Payload['key_levels'] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        {levels.map((level) => (
          <div key={level.name} className="flex flex-wrap items-baseline gap-2" data-level>
            <span className="font-medium">{level.name}</span>
            <Badge variant="secondary">{level.price.text}</Badge>
            <span>{typeof level.distance === 'string' ? level.distance : level.distance.text}</span>
            <span className="text-xs text-muted-foreground">{level.note}</span>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

export function Timeline({ title, stages }: { title: string; stages: Payload['timeline'] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <Accordion type="multiple" defaultValue={stages.map((stage) => stage.name)}>
          {stages.map((stage) => (
            <AccordionItem key={stage.name} value={stage.name}>
              <AccordionTrigger>{stage.name}</AccordionTrigger>
              <AccordionContent className="flex gap-3">
                <span>{stage.date_text}</span>
                <span>{stage.days_text}</span>
                <Badge variant="secondary">{stage.elapsed.text}</Badge>
              </AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      </CardContent>
    </Card>
  )
}
