import type { NameOutcome } from '../../../../src/types/option_candidates_page'

// Synthetic outcomes of Japanese names without rows; every text is a field the page draws as sent.
export const nameOutcomes: NameOutcome[] = [
  { name: 'no_quote Co', outcome: 'no_quote', text: 'no 交易所无报价', counts: 'no 共 2413 个' },
  { name: 'failed Co', outcome: 'failed', text: 'no 取数失败（可重试）', counts: 'no 共 3 个' },
]
