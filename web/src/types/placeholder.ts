// TEMPORARY hand-written placeholder (not generated; web/src/generated stays the stub). Types copied from the old repository until the week1-schemas issue generates them.
// It checks what a page needs to refuse to render a payload it cannot read: the payload is an object and its
// `schema_version` is one the page supports. It does not check the shape of the fields (the generated validators
// will); the page types beside it are copies of the old repository's generated types.
export type Gap = { reason: 'unsupported_version' | 'invalid_payload'; text: string }
export type Result<P> = { ok: true; payload: P } | { ok: false; gap: Gap; errors: never[] }

export function placeholderValidate<P>(supported: readonly number[]): (input: unknown) => Result<P> {
  return (input) => {
    if (typeof input !== 'object' || input === null) {
      return {
        ok: false,
        gap: {
          reason: 'invalid_payload',
          text: 'The payload does not match its schema. Do not act on this card.',
        },
        errors: [],
      }
    }
    const version = 'schema_version' in input ? input.schema_version : undefined
    if (typeof version !== 'number' || !supported.includes(version)) {
      return {
        ok: false,
        gap: {
          reason: 'unsupported_version',
          text: `Unsupported schema_version; this page reads ${supported.join(', ')} only. Do not act on this card.`,
        },
        errors: [],
      }
    }
    return { ok: true, payload: input as P }
  }
}
