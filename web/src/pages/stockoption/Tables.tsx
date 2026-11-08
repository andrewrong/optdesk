// The matrix and the contracts table of the stock option page, both drawn by the shared DataTable.
// The headers, the cells and the sentences are the payload's own text; nothing is computed here.
import { useState } from 'react'
import { Toggle } from '@/components/ui/toggle'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import type { Payload } from '@/types/stock_option_page'

type Matrix = Payload['matrix']
type Contracts = Payload['contracts']

// One row per expiry, one column per strike; a cell is found by its expiry and strike, and a pair with no
// contract is an empty cell.
export function MatrixTable({ matrix }: { matrix: Matrix }) {
  // View state only: each cell carries both texts; the toggle picks which one is shown.
  const [showPremium, setShowPremium] = useState(false)
  const columns: Column[] = [
    { id: 'expiry', label: '' },
    ...matrix.strikes.map((strike) => ({ id: `s:${strike}`, label: strike, align: 'right' as const })),
  ]
  const rows = matrix.expiries.map((expiry) => {
    const row: Record<string, Cell> = { expiry: { text: expiry.text } }
    for (const strike of matrix.strikes) {
      const cell = matrix.cells.find((c) => c.expiry === expiry.expiry && c.strike === strike)
      row[`s:${strike}`] = cell
        ? { text: showPremium ? cell.premium : cell.yield_text, badge: cell.chosen }
        : { text: '' }
    }
    return row
  })
  return (
    <Card data-block="matrix">
      <CardContent className="space-y-3">
        <Toggle
          variant="outline"
          size="sm"
          pressed={showPremium}
          onPressedChange={setShowPremium}
          aria-label="年化 / 权利金"
        >
          {showPremium ? '权利金' : '年化'}
        </Toggle>
        {matrix.facts ? <p className="text-sm text-muted-foreground">{matrix.facts}</p> : null}
        {matrix.empty ? (
          <p data-matrix-empty>{matrix.empty}</p>
        ) : (
          <DataTable columns={columns} rows={rows} />
        )}
        <p className="text-xs text-muted-foreground">{matrix.legend}</p>
      </CardContent>
    </Card>
  )
}

export function ContractsTable({ contracts }: { contracts: Contracts }) {
  const columns: Column[] = contracts.columns.map((label, index) => ({ id: `c${index}`, label }))
  const rows = contracts.rows.map((line) => {
    const row: Record<string, Cell> = {}
    line.cells.forEach((text, index) => {
      row[`c${index}`] = { text }
    })
    return row
  })
  return (
    <Card data-block="contracts">
      <CardHeader>
        <CardTitle>{contracts.basis}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {contracts.empty ? (
          <p data-contracts-empty>{contracts.empty}</p>
        ) : (
          <DataTable columns={columns} rows={rows} />
        )}
        {contracts.notes.map((note) => (
          <p key={note} className="text-sm text-muted-foreground">
            {note}
          </p>
        ))}
      </CardContent>
    </Card>
  )
}
