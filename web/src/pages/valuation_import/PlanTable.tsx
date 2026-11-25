// The per-company preview table of the valuation import page: the action, the site's buy-zone top against the file's,
// the key field of an unmatched name and the tick of a row that needs one. Every figure is a payload text.
import type { Labels, Row } from '@/generated/valuation_import_page.v1'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

interface Props {
  labels?: Labels
  rows: Row[]
  keys: Record<string, string>
  markets: Record<string, string>
  ticks: Record<string, boolean>
  onKey: (ticker: string, key: string) => void
  onMarket: (ticker: string, market: string) => void
  onTick: (ticker: string, on: boolean) => void
}

function variant(action: Row['action']) {
  if (action === 'insert') return 'default'
  if (action === 'refused') return 'destructive'
  return 'secondary'
}

export function PlanTable({ labels, rows, keys, markets, ticks, onKey, onMarket, onTick }: Props) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>公司</TableHead>
          <TableHead>公司键</TableHead>
          <TableHead>处理</TableHead>
          <TableHead className="text-right">V</TableHead>
          <TableHead className="text-right">P</TableHead>
          <TableHead className="text-right">{labels?.buy_zone_top_site}</TableHead>
          <TableHead className="text-right">{labels?.buy_zone_top_file}</TableHead>
          <TableHead>勾选</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((r) => (
          <TableRow key={r.ticker} data-ticker={r.ticker}>
            <TableCell>
              <div>{r.name}</div>
              <div className="text-xs text-muted-foreground">
                {r.ticker} {r.currency} {r.r_market}
                {r.new_profile ? ' · 新建档案' : ''}
              </div>
            </TableCell>
            <TableCell>
              {r.unmatched ? (
                <Input
                  name={`key.${r.ticker}`}
                  aria-label={`key ${r.ticker}`}
                  value={keys[r.ticker] ?? r.proposed_key}
                  onChange={(e) => onKey(r.ticker, e.target.value)}
                />
              ) : (
                r.key
              )}
              {r.unmatched ? (
                <>
                  <Label htmlFor={`r-market-${r.ticker}`} className="mt-1">
                    {labels?.r}
                  </Label>
                  <Input
                    id={`r-market-${r.ticker}`}
                    name={`r_market.${r.ticker}`}
                    aria-label={`${labels?.r ?? ''} ${r.ticker}`}
                    value={markets[r.ticker] ?? r.r_market}
                    onChange={(e) => onMarket(r.ticker, e.target.value)}
                  />
                </>
              ) : null}
            </TableCell>
            <TableCell>
              <Badge variant={variant(r.action)}>{r.action}</Badge>
              {r.reason ? <div className="text-xs text-muted-foreground">{r.reason}</div> : null}
            </TableCell>
            <TableCell className="text-right">{r.v}</TableCell>
            <TableCell className="text-right">{r.p}</TableCell>
            <TableCell className="text-right">{r.site_buy_top}</TableCell>
            <TableCell className="text-right">{r.file_buy_top}</TableCell>
            <TableCell>
              {r.needs_tick ? (
                <Checkbox
                  name="tick"
                  value={r.ticker}
                  aria-label={`tick ${r.ticker}`}
                  checked={ticks[r.ticker] === true}
                  onCheckedChange={(c) => onTick(r.ticker, c === true)}
                />
              ) : null}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
