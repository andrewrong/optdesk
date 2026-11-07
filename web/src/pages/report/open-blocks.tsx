// The six blocks the open slot adds: market_line, tiles, accounts, candidates, reminders and system_status.
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Badge } from '@/components/ui/badge'
import type {
  Accounts2,
  Candidates2,
  MarketLine,
  Reminders,
  SystemStatus,
  Tiles,
} from '@/generated/report_page.v2'
import { Fig, Meters, Tabular, ToneText } from './cells'
import { Frame, LinkButton } from './frame'

export function MarketLineBlock({ block }: { block: MarketLine }) {
  return (
    <Frame block={block}>
      {block.sentence ? <p className="text-sm">{block.sentence}</p> : null}
      <dl className="flex flex-wrap gap-x-6 gap-y-1 text-sm" data-figures>
        {(block.figures ?? []).map((item) => (
          <div key={item.label}>
            <dt className="text-xs text-muted-foreground">{item.label}</dt>
            <dd>
              <Fig shown={item.figure} />
            </dd>
          </div>
        ))}
      </dl>
    </Frame>
  )
}

export function TilesBlock({ block }: { block: Tiles }) {
  return (
    <Frame block={block}>
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4" data-tiles>
        {(block.tiles ?? []).map((tile) => (
          <div key={tile.label} className="rounded-md border p-3">
            <div className="text-xs text-muted-foreground">{tile.label}</div>
            <div className="text-lg">
              <Fig shown={tile.figure} />
            </div>
            {tile.note ? <div className="text-xs text-muted-foreground">{tile.note}</div> : null}
          </div>
        ))}
      </div>
    </Frame>
  )
}

export function AccountsBlock({ block }: { block: Accounts2 }) {
  return (
    <Frame block={block}>
      <Meters meters={block.meters} />
    </Frame>
  )
}

export function CandidatesBlock({ block }: { block: Candidates2 }) {
  const { covered_call: call, put_spread: spread } = block
  return (
    <Frame block={block}>
      {(block.groups ?? []).map((group) => (
        <section key={group.sector} className="space-y-1">
          <h3 className="text-sm font-medium">{group.sector}</h3>
          <Tabular
            labels={group.columns}
            rows={group.rows.map((r) =>
              'figures' in r
                ? [r.symbol, { text: r.text, tone: r.tone }, ...r.figures]
                : [r.symbol, { text: r.text, tone: r.tone }],
            )}
          />
          {group.watch_title ? <p className="text-xs text-muted-foreground">{group.watch_title}</p> : null}
        </section>
      ))}
      {call ? (
        <p className="text-sm" data-covered-call>
          <strong>{call.symbol}</strong> <ToneText text={call.text} tone={call.tone} />{' '}
          <Fig shown={call.strike} /> <Fig shown={call.premium} /> <Fig shown={call.annualized} />
        </p>
      ) : null}
      {spread ? (
        <p className="text-sm" data-put-spread>
          <strong>{spread.symbol}</strong> <ToneText text={spread.text} tone={spread.tone} />{' '}
          <Fig shown={spread.short_strike} /> <Fig shown={spread.long_strike} />{' '}
          <Fig shown={spread.credit} /> <Fig shown={spread.max_loss} />
        </p>
      ) : null}
      {block.full_page ? <LinkButton link={block.full_page} /> : null}
    </Frame>
  )
}

export function RemindersBlock({ block }: { block: Reminders }) {
  const rest = block.rest ?? []
  return (
    <Frame block={block}>
      <ul className="space-y-1 text-sm" data-reminders>
        {(block.rows ?? []).map((row, at) => (
          <li key={`${at}-${row.text}`}>
            <ToneText text={row.text} tone={row.tone} />
            {row.link ? <LinkButton link={row.link} /> : null}
          </li>
        ))}
      </ul>
      {rest.length > 0 ? (
        <Accordion type="single" collapsible data-rest>
          <AccordionItem value="rest">
            <AccordionTrigger>{block.rest_title ?? ''}</AccordionTrigger>
            <AccordionContent>
              <Tabular labels={block.rest_columns ?? []} rows={rest.map((r) => r.cells)} />
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      ) : null}
    </Frame>
  )
}

export function SystemStatusBlock({ block }: { block: SystemStatus }) {
  const lines = (
    <dl className="text-sm" data-lines>
      {(block.lines ?? []).map((line, at) => (
        <div key={`${at}-${line.label}`} className="flex gap-2">
          <dt className="text-muted-foreground">{line.label}</dt>
          <dd>{line.text}</dd>
        </div>
      ))}
    </dl>
  )
  return (
    <Frame block={block}>
      <Accordion
        type="single"
        collapsible
        defaultValue={block.collapsed ? undefined : 'lines'}
        data-collapsed={block.collapsed === true}
      >
        <AccordionItem value="lines">
          <AccordionTrigger>
            <Badge variant="secondary">{block.title}</Badge>
          </AccordionTrigger>
          <AccordionContent>{lines}</AccordionContent>
        </AccordionItem>
      </Accordion>
    </Frame>
  )
}
