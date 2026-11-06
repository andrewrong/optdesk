import type { ComponentType } from 'react'

// A page of the shell (ADR 0034): the generated validator and the component of one schema. The validator returns the
// payload typed, or a gap whose text is thrown.
export type Validation<P> = { ok: true; payload: P } | { ok: false; gap: { text: string } }
export interface Page {
  validate: (input: unknown) => Validation<unknown>
  component: ComponentType<{ payload: never }>
}

// One place where the payload type is erased: the validator and the component of an entry share `P`.
export function page<P>(
  validate: (input: unknown) => Validation<P>,
  component: ComponentType<{ payload: P }>,
): Page {
  return { validate, component } as Page
}
