import type { StockOptionPage } from '../../../../src/generated/stock_option_page.v3'
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
  vol_view: {
    ...ok.vol_view,
    rows: [
      { label: 'Implied vol', text: '', data_day: '', gap: 'no_input' },
      { label: 'Realised vol 20d', text: '', data_day: '', gap: 'no_input' },
      { label: 'Realised vol 60d', text: '', data_day: '', gap: 'no_input' },
      { label: 'Realised vol 120d', text: '', data_day: '', gap: 'no_input' },
      { label: 'Forecast vol', text: '', data_day: '', gap: 'no_input' },
      { label: 'Vol edge', text: '', data_day: '', gap: 'no_input' },
      { label: 'IV percentile', text: '', data_day: '', gap: 'no_input' },
      { label: 'Forecast error', text: '', data_day: '', gap: 'no_input' },
    ],
  },
  notice: null,
  shared_gap: 'Gap the quotes are missing for this run',
} satisfies StockOptionPage
