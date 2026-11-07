import type { StockOptionPage } from '../../../../src/generated/stock_option_page.v1'
import { ok } from './ok'

export const gap = {
  ...ok,
  run_status: 'partial',
  cards: [],
  no_card: 'No card because no contract passes',
  matrix: { facts: null, legend: ok.matrix.legend, empty: 'Matrix empty no cell', strikes: [], expiries: [], cells: [] },
  maps: [],
  contracts: { ...ok.contracts, rows: [], empty: 'Contracts empty no put fetched' },
  held: [],
  notice: null,
  shared_gap: 'Gap the quotes are missing for this run',
} satisfies StockOptionPage
