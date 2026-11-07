import { MacroPage, validateMacroPage, type MacroPagePayload } from '@/pages/macro/MacroPage'
import {
  HouseholdPage,
  validateHouseholdPage,
  type HouseholdPagePayload,
} from '@/pages/household/HouseholdPage'
import {
  CompaniesPage,
  validateCompaniesPage,
  type CompaniesPagePayload,
} from '@/pages/companies/CompaniesPage'
import { AdvicePage, validateAdvicePage, type AdvicePagePayload } from '@/pages/advice/AdvicePage'
import { MacroCard, validateMacro, type MacroPayload } from '@/pages/home/MacroCard'
import {
  FullScanPage,
  validateFullScan,
  type FullScanPayload,
} from '@/pages/candidates/FullScanPage'
import { OptionPage, validateOptionPage, type OptionPagePayload } from '@/pages/options/OptionPage'
import {
  PositionsPage,
  validatePositionsPage,
  type PositionsPagePayload,
} from '@/pages/positions/PositionsPage'
import { Tracking, validateTracking, type TrackingPayload } from '@/pages/options/Tracking'
import {
  StructuresList,
  validateStructuresList,
  type StructuresListPayload,
} from '@/pages/structure/StructuresList'
import {
  OptionDataPage,
  validateOptionDataPage,
  type OptionDataPagePayload,
} from '@/pages/optiondata/OptionDataPage'
import { page, type Page } from '@/shell/page'

// The page registry of the shell (ADR 0034): schema name -> the generated validator and the component.
// A card file `pages/home/*Card.tsx` that exports `registration = { schema, page: page(validate, Card) }` is found
// by a Vite glob import and needs no line here (issue #1155); the entries below are the ones that stay explicit.

export interface CardRegistration {
  schema: string
  page: Page
}

// The cards of a module map: a module with a `registration` adds its page, one without is skipped. A schema name
// that is registered twice, or is already in `taken`, throws, so a copy-paste slip fails at load.
export function discoverCards(
  modules: Record<string, unknown>,
  taken: Record<string, Page> = {},
): Record<string, Page> {
  const found = new Map<string, Page>() // a Map: a schema named `__proto__` is still one entry
  for (const [file, module] of Object.entries(modules)) {
    const registration = (module as { registration?: CardRegistration } | null)?.registration
    if (!registration) continue
    if (Object.hasOwn(taken, registration.schema) || found.has(registration.schema)) {
      throw new Error(`Duplicate page schema "${registration.schema}" (${file})`)
    }
    found.set(registration.schema, registration.page)
  }
  return Object.fromEntries(found)
}

const explicit: Record<string, Page> = {
  advice_page: page<AdvicePagePayload>(validateAdvicePage, AdvicePage),
  household_page: page<HouseholdPagePayload>(validateHouseholdPage, HouseholdPage),
  companies_page: page<CompaniesPagePayload>(validateCompaniesPage, CompaniesPage),
  structures_list: page<StructuresListPayload>(validateStructuresList, StructuresList),
  tracking: page<TrackingPayload>(validateTracking, Tracking),
  macro_page: page<MacroPagePayload>(validateMacroPage, MacroPage),
  macro_summary: page<MacroPayload>(validateMacro, MacroCard),
  full_scan: page<FullScanPayload>(validateFullScan, FullScanPage),
  option_page: page<OptionPagePayload>(validateOptionPage, OptionPage),
  positions_page: page<PositionsPagePayload>(validatePositionsPage, PositionsPage),
  option_data_page: page<OptionDataPagePayload>(validateOptionDataPage, OptionDataPage),
}

export const pages: Record<string, Page> = {
  ...explicit,
  ...discoverCards(import.meta.glob('../pages/home/*Card.tsx', { eager: true }), explicit),
}
