// The price maps, one tab per expiry, and the held block. Their text and figures are the payload's.
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { Payload } from '@/types/stock_option_page'

export function PriceMaps({ maps }: { maps: Payload['maps'] }) {
  const first = maps[0]
  if (!first) return null
  return (
    <Card data-block="maps">
      <CardContent>
        <Tabs defaultValue={first.expiry}>
          <TabsList>
            {maps.map((map) => (
              <TabsTrigger key={map.expiry} value={map.expiry}>
                {map.heading}
              </TabsTrigger>
            ))}
          </TabsList>
          {maps.map((map) => (
            <TabsContent key={map.expiry} value={map.expiry} className="space-y-2 text-sm">
              <p>{map.description}</p>
              {map.bands.map((band) => (
                <p key={band.kind} data-band={band.kind}>
                  {band.text}
                </p>
              ))}
              <ul>
                {map.marks.map((mark) => (
                  <li key={`${mark.kind}-${mark.price}`} data-mark={mark.kind}>
                    {mark.text}
                  </li>
                ))}
              </ul>
            </TabsContent>
          ))}
        </Tabs>
      </CardContent>
    </Card>
  )
}

export function HeldBlock({ held }: { held: Payload['held'] }) {
  if (!held[0]) return null
  return (
    <div className="grid gap-4 md:grid-cols-2" data-block="held">
      {held.map((item) => (
        <Card key={`${item.account}-${item.headline}`}>
          <CardHeader>
            <CardTitle>{item.headline}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <p>{item.detail}</p>
            <p className="text-muted-foreground">{item.account}</p>
            {item.earned ? <p>{item.earned}</p> : null}
            {item.progress ? (
              <div>
                <Progress value={Number(item.progress.done)} aria-label={item.progress.note} />
                <p className="text-xs text-muted-foreground">{item.progress.note}</p>
              </div>
            ) : null}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
