// The other underlyings of the option data page: label and href are shown as the builder wrote them.
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Payload } from '@/types/option_data_page'

type UnderlyingChoice = NonNullable<Payload['underlyings']>[number]

export function UnderlyingLinks({ underlyings }: { underlyings: UnderlyingChoice[] }) {
  return (
    <Card data-underlying-links>
      <CardHeader>
        <CardTitle>Underlyings</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-2">
        {underlyings.map((choice) => (
          <Button key={choice.id} asChild variant={choice.current ? 'secondary' : 'ghost'}>
            <a href={choice.href} aria-current={choice.current ? 'page' : undefined}>
              {choice.label}
            </a>
          </Button>
        ))}
      </CardContent>
    </Card>
  )
}
