// The ledger schema's Cell is `{text, tone}` with tone gain / loss / none; the shared table reads up / down / flat.
import type { FigureCell } from '@/components/data-table/data-table'
import type { Cell } from '@/generated/ledger_page.v1'

const TONE = { gain: 'up', loss: 'down', none: 'none' } as const

export function figure(cell: Cell): FigureCell {
  return { text: cell.text, tone: TONE[cell.tone ?? 'none'] }
}

// The detail lines of a row are shown as a note under its first cell.
export function withDetail(cells: Cell[], detail: string[]): FigureCell[] {
  return cells.map((cell, index) =>
    index === 0 ? { ...figure(cell), note: detail.join(' · ') } : figure(cell),
  )
}

export function rowOf(cells: FigureCell[]): Record<string, FigureCell> {
  return Object.fromEntries(cells.map((cell, index) => [`c${index}`, cell]))
}
