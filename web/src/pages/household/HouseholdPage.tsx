// The household page (board 12, issue #1437). Every figure, sum, share, step and sentence is supplied by Python;
// tables only sort on demand and the charts draw the points of the payload. A block that holds a gap shows its text.
import { ChevronRight, Info, Users } from 'lucide-react'
import { Chart } from '@/charts/Chart'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/household_page'
import { CardNotes } from '../home/CardNotes'
import { StaleAlert } from '../home/StaleAlert'
import { AccountsSection, MoversSection, ScopeHeading, WhoseBadge } from './AccountsMovers'
import { AllocationSection, Block, IncomeMonths, MoneySection } from './MoneySections'
import type { MoneyBlock, Scope } from './householdTypes'
import { incomeToSpec, monthChangeToSpec, trendToSpec } from './householdToSpec'
export { validate as validateHouseholdPage } from '@/types/household_page'
export type { Payload as HouseholdPagePayload } from '@/types/household_page'

const BADGE_VARIANT = {
  up: 'success',
  down: 'down',
  flat: 'secondary',
  none: 'secondary',
} as const

// The concentration table (issue #1680): the names under their scope, each with the accounts that hold it; the
// household total row stays under the household scope; a scope with no row shows Python's gap line.
function ConcentrationSection({ block, scopes }: { block: MoneyBlock; scopes: Scope[] }) {
  return (
    <Block
      name="concentration"
      title={block.title}
      meaning={block.meaning}
      result={block.result}
      gap={block.gap}
    >
      {scopes.map((scope) => {
        const rows = block.rows.filter((row) => row.scope === scope.key)
        const gap = block.scope_gaps?.find((g) => g.scope === scope.key)
        if (rows.length === 0 && !gap) return null
        return (
          <section key={scope.key} className="min-w-0 space-y-2" data-scope-rows={scope.key}>
            <ScopeHeading scope={scope} />
            {gap ? (
              <Alert data-scope-gap data-reason={gap.gap.reason}>
                <Info aria-hidden />
                <AlertDescription>{gap.gap.text}</AlertDescription>
              </Alert>
            ) : null}
            {rows.length > 0 ? (
              <Table>
                {block.heads ? (
                  <TableHeader>
                    <TableRow>
                      {block.heads.map((head, i) => (
                        <TableHead key={i} className={i > 0 ? 'text-right' : undefined}>
                          {head}
                        </TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                ) : null}
                <TableBody>
                  {rows.map((row) => (
                    <TableRow key={`${row.scope}-${row.name}`}>
                      <TableCell>
                        {row.name}
                        {row.accounts?.map((tag) => (
                          <span key={tag.label} className="ml-2 inline-flex items-center gap-1">
                            <WhoseBadge tag={tag} />
                            <span className="text-xs text-muted-foreground">
                              {tag.label}
                              {tag.amount ? ` ${tag.amount.text}` : ''}
                            </span>
                          </span>
                        ))}
                      </TableCell>
                      <TableCell className="text-right">{row.share?.text ?? ''}</TableCell>
                      <TableCell className="text-right">{row.amount.text}</TableCell>
                    </TableRow>
                  ))}
                  {scope.key === 'household'
                    ? block.totals.map((cell, i) => (
                        <TableRow key={`total-${i}`}>
                          <TableCell className="font-medium">
                            {block.total_labels?.[i] ?? block.title}
                          </TableCell>
                          <TableCell />
                          <TableCell className="text-right">{cell.text}</TableCell>
                        </TableRow>
                      ))
                    : null}
                </TableBody>
              </Table>
            ) : null}
          </section>
        )
      })}
    </Block>
  )
}

// One grid cell of the page: a wide cell spans both columns from lg, a half cell shares the row with its pair.
function Cell({ name, wide, children }: { name: string; wide?: boolean; children: React.ReactNode }) {
  return (
    <div className={wide ? 'min-w-0 lg:col-span-2' : 'min-w-0'} data-cell={name}>
      {children}
    </div>
  )
}

export function HouseholdPage({ payload }: { payload: Payload }) {
  const { trend, income, month_change: change } = payload
  const trendSpec = trendToSpec(trend.series ?? [])
  const trendGap = (trend.series ?? []).some((s) => s.gap) ? null : (trend.gap ?? null)
  const incomeCurrencies = income.totals.map((cell) => cell.currency)
  return (
    <div className="min-w-0 space-y-6" data-page="household">
      <header>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h1 className="text-2xl font-semibold">{payload.title}</h1>
          <Button asChild variant="link">
            <a href="/risk" aria-label={payload.risk_link ?? undefined}>
              {payload.risk_link}
              <ChevronRight />
            </a>
          </Button>
          <Button asChild variant="link">
            <a href="/positions" aria-label={payload.positions_link ?? undefined}>
              {payload.positions_link}
              <ChevronRight />
            </a>
          </Button>
        </div>
        <p className="mt-2 text-muted-foreground">{payload.sentence}</p>
      </header>
      <StaleAlert stale={payload.stale} generatedAt={payload.generated_at} />
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <Cell name="overview" wide>
        <MoneySection name="overview" block={payload.overview} />
      </Cell>
      {payload.accounts ? (
        <Cell name="accounts" wide>
          <AccountsSection block={payload.accounts} scopes={payload.scopes} />
        </Cell>
      ) : null}
      <Cell name="composition" wide>
        <MoneySection name="composition" block={payload.composition} />
      </Cell>
      <Cell name="movers" wide>
        <MoversSection block={payload.movers} scopes={payload.scopes} />
      </Cell>
      <Cell name="trend">
      <Block name="trend" title={trend.title} meaning={trend.meaning} gap={trendGap}>
        {trendSpec ? (
          <Chart
            name="household-trend"
            spec={trendSpec}
            ariaText={trend.meaning}
          />
        ) : null}
        {(trend.series ?? []).map((series) => (
          <section key={series.scope} className="min-w-0 space-y-2" data-scope-rows={series.scope}>
            <h3 className="flex items-center gap-2 text-sm font-medium">
              <Users aria-hidden className="size-4" />
              {series.label}
            </h3>
            {series.gap ? (
              <Alert data-block-gap data-reason={series.gap.reason}>
                <Info aria-hidden />
                <AlertDescription>{series.gap.text}</AlertDescription>
              </Alert>
            ) : null}
            {series.change ? (
              <Badge variant={BADGE_VARIANT[series.change.tone ?? 'none']}>
                {series.change.text}
              </Badge>
            ) : null}
            {series.window ? (
              <p className="text-sm text-muted-foreground" data-trend-window>
                {series.window}
              </p>
            ) : null}
          </section>
        ))}
      </Block>
      </Cell>
      <Cell name="stress">
        <MoneySection name="stress" block={payload.stress} />
      </Cell>
      <Cell name="income" wide>
      <Block
        name="income"
        title={income.title}
        meaning={income.meaning}
        result={income.result}
        gap={income.gap}
      >
        {incomeCurrencies.map((currency) => (
          <Chart
            key={currency}
            name={`household-income-${currency}`}
            spec={incomeToSpec(income, currency)}
            ariaText={income.meaning}
          />
        ))}
        <IncomeMonths block={income} />
        {income.scope_gaps?.map((g) => (
          <p key={g.scope} className="text-sm text-muted-foreground" data-scope-gap={g.scope}>
            {g.gap.text}
          </p>
        ))}
      </Block>
      </Cell>
      <Cell name="month_change">
      <Block
        name="month_change"
        title={change.title}
        meaning={change.meaning}
        result={change.result}
        gap={change.gap}
      >
        {change.rows.length > 0 ? (
          <Chart
            name="household-month-change"
            spec={monthChangeToSpec(change)}
            ariaText={change.meaning}
          />
        ) : null}
      </Block>
      </Cell>
      <Cell name="cash_after">
        <MoneySection name="cash_after" block={payload.cash_after} />
      </Cell>
      <Cell name="obligations" wide>
        <MoneySection name="obligations" block={payload.obligations} />
      </Cell>
      <Cell name="concentration" wide>
        <ConcentrationSection block={payload.concentration} scopes={payload.scopes} />
      </Cell>
      <Cell name="allocation" wide>
        <AllocationSection block={payload.allocation} />
      </Cell>
      </div>
      <CardNotes notes={payload.notes} gaps={payload.gaps} />
    </div>
  )
}

export const registration = {
  schema: 'household_page',
  page: page<Payload>(validate, HouseholdPage),
}
