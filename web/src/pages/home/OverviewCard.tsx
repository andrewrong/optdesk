// The top card of the home page: the page title and say, one line per card (its label links to the card's page,
// the text is the card's own words) and the real gaps. Every string is a field of the payload; nothing is computed.
import type { ReactNode } from 'react'
import { ChevronRight, TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { Payload } from '@/types/home_page'

export function OverviewCard({ payload }: { payload: Payload }) {
  const { overview } = payload
  return (
    <Card data-card="overview">
      <CardHeader>
        <CardTitle>{payload.title}</CardTitle>
        <CardDescription>{payload.say}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <ul className="space-y-1" data-overview-items>
          {overview.items.map((item, index) => (
            <li key={`${index}-${item.href}`} className="flex flex-wrap items-baseline gap-x-2" data-overview-item>
              <Button asChild variant="link" className="h-auto p-0">
                <a href={item.href}>
                  {item.label}
                  <ChevronRight aria-hidden />
                </a>
              </Button>
              <span data-overview-text>{item.text}</span>
            </li>
          ))}
        </ul>
        <Alert data-gap-line>
          {overview.gaps.length > 0 ? <TriangleAlert aria-hidden /> : null}
          <AlertTitle>{payload.gap_line}</AlertTitle>
          {overview.gaps.length > 0 ? (
            <AlertDescription>
              <ul className="space-y-1" data-overview-gaps>
                {overview.gaps.map((gap, index) => (
                  <li key={`${index}-${gap.card}`}>
                    <span className="font-medium">{gap.card}</span> {gap.text}
                  </li>
                ))}
              </ul>
            </AlertDescription>
          ) : null}
        </Alert>
      </CardContent>
    </Card>
  )
}

// The title of a home card as a link to the card's page (`titleHref`, the href of its overview line). Without an
// href (a card shown on its own page) the title is plain text. The view holds no route table.
export function TitleLink({ href, children }: { href?: string | undefined; children: ReactNode }) {
  if (!href) return <>{children}</>
  return (
    <Button asChild variant="link" className="h-auto p-0 text-[length:inherit] font-[inherit]">
      <a href={href}>{children}</a>
    </Button>
  )
}
