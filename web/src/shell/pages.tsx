import { type Page } from '@/shell/page'

// The page registry of the shell (ADR 0034): schema name -> the generated validator and the component.
// A file `pages/<dir>/*.tsx` that exports `registration = { schema, page: page(validate, View) }` is found by one
// Vite glob import and needs no line here (issue #1155); no page is registered by hand in this file.

export interface CardRegistration {
  schema: string
  page: Page
}

// The pages of a module map: a module with a `registration` adds its page, one without is skipped. A schema name
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

export const pages: Record<string, Page> = discoverCards(
  import.meta.glob('../pages/*/*.tsx', { eager: true }),
)
