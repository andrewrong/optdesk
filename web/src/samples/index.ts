import advice from '../../fixtures/advice-page/ok.json'
import home from '../../fixtures/home-page/ok.json'
import household from '../../fixtures/household-page/ok.json'
import risk from '../../fixtures/household-risk/ok.json'
import option from '../../fixtures/option-page/ok.json'
import positions from '../../fixtures/positions-page/ok.json'

// The sample payloads of the six names of `page.sample_page.v1` (Decision 4 of docs/design/unified-site.md): existing
// fixture data, imported as it is. The shell shows the one a `sample_page` payload names.
export const samples: Readonly<Record<string, unknown>> = {
  home_page: home,
  household_page: household,
  positions_page: positions,
  household_risk: risk,
  option_page: option,
  advice_page: advice,
}
