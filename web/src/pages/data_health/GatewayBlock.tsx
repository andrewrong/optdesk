// The gateways of the data health page (docs/design/gateway-health.md#8-gateway-health-web): each gateway with its
// verdict text and its farms. Every visible value is a payload field; this file computes no verdict.
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Payload } from '@/types/data_health_page'

export function GatewayBlock({ gateways }: { gateways: NonNullable<Payload['gateways']> }) {
  return (
    <Card data-gateways>
      <CardHeader>
        <CardTitle>网关</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {gateways.map((g) => (
          <div key={g.name} data-gateway={g.name} data-verdict={g.verdict} className="space-y-1 text-sm">
            <p className="font-medium">{g.name}</p>
            {g.note ? <p>{g.note}</p> : null}
            {g.since_text ? <p data-since>{g.since_text}</p> : null}
            {g.farms.length > 0 ? (
              <ul className="flex flex-wrap gap-3">
                {g.farms.map((f) => (
                  <li key={f.name} data-farm={f.name} data-farm-state={f.up ? 'ok' : 'down'}>
                    {f.name}: {f.up ? 'ok' : 'down'}
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
