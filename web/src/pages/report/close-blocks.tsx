// The close blocks that are not the global assets: actions, account_moves, risks and market_watch.
import { Badge } from '@/components/ui/badge'
import type {
  AccountMovesBlock,
  ActionsBlock,
  MarketWatchBlock,
  RisksBlock,
} from '@/generated/report_page.v2'
import { AccountMeters, GapMark, Tabular, isGap } from './cells'
import { Frame } from './frame'

export function Actions({ block }: { block: ActionsBlock }) {
  const { trades, candidates } = block
  return (
    <Frame block={block}>
      {isGap(trades) ? (
        <GapMark gap={trades} />
      ) : (
        <>
          <Tabular
            labels={trades.columns}
            rows={trades.rows.map((r) => [r.time, r.account, r.action, r.instrument, r.quantity, r.price])}
          />
          {trades.footnote ? <p className="text-xs text-muted-foreground">{trades.footnote}</p> : null}
        </>
      )}
      {candidates && isGap(candidates) ? <GapMark gap={candidates} /> : null}
      {candidates && !isGap(candidates) ? (
        <section data-candidates>
          <h3 className="text-sm font-medium">{candidates.heading}</h3>
          {candidates.groups.map((group) => (
            <div key={group.label} className="space-y-1">
              <h4 className="text-xs text-muted-foreground">{group.label}</h4>
              <ul className="space-y-1 text-sm">
                {group.items.map((item) => (
                  <li key={item.symbol}>
                    <strong>{item.symbol}</strong> {item.structure} · {item.verdict}
                    <div className="text-xs text-muted-foreground">
                      {item.line1} · {item.line2} · {item.liquidity}
                    </div>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </section>
      ) : null}
    </Frame>
  )
}

export function AccountMoves({ block }: { block: AccountMovesBlock }) {
  const { accounts, movers } = block
  return (
    <Frame block={block}>
      {isGap(accounts) ? (
        <GapMark gap={accounts} />
      ) : (
        <Tabular
          labels={accounts.columns}
          rows={accounts.rows.map((r) => [r.account, r.net, r.day_amount, r.day_pct, r.biggest])}
        />
      )}
      {isGap(movers) ? (
        <GapMark gap={movers} />
      ) : (
        <section>
          <h3 className="text-sm font-medium">{movers.heading}</h3>
          <Tabular
            labels={movers.columns}
            rows={movers.rows.map((r) => [r.symbol, r.account, r.day_amount, r.note])}
          />
        </section>
      )}
      <AccountMeters meters={block.meters} />
    </Frame>
  )
}

const LEVEL_VARIANT = { n: 'success', w: 'warning', d: 'down' } as const

export function Risks({ block }: { block: RisksBlock }) {
  const { items } = block
  return (
    <Frame block={block}>
      {isGap(items) ? (
        <GapMark gap={items} />
      ) : (
        <ul className="space-y-2" data-risks>
          {items.map((item, at) => (
            <li key={`${at}-${item.title}`} data-level={item.level}>
              <Badge variant={LEVEL_VARIANT[item.level]}>{item.level}</Badge> <strong>{item.title}</strong>
              <div className="text-xs text-muted-foreground">{item.detail}</div>
            </li>
          ))}
        </ul>
      )}
    </Frame>
  )
}

export function MarketWatch({ block }: { block: MarketWatchBlock }) {
  const { indices, companies } = block
  return (
    <Frame block={block}>
      {isGap(indices) ? (
        <GapMark gap={indices} />
      ) : (
        <Tabular
          labels={indices.columns}
          rows={indices.rows.map((r) => [r.name, r.close, r.day, r.five_day])}
        />
      )}
      {isGap(companies) ? (
        <GapMark gap={companies} />
      ) : (
        <Tabular labels={companies.columns} rows={companies.rows.map((r) => [r.symbol, r.day, r.news])} />
      )}
      {block.muted_note ? <p className="text-xs text-muted-foreground">{block.muted_note}</p> : null}
    </Frame>
  )
}

