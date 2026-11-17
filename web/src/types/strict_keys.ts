// A payload check for the pages whose generated validator has not landed: the placeholder check (an object with a supported
// `schema_version`) plus a refusal of any field the contract does not name, at every depth. A shape lists the fields of an
// object, the shape of the items of an array or the shape of the values of a map; `null` is a value read as given.
import { placeholderValidate, type Result } from './placeholder.ts'

export type Shape = null | { fields: { [key: string]: Shape } } | { items: Shape } | { values: Shape }
export const obj = (fields: { [key: string]: Shape }): Shape => ({ fields })
export const arr = (items: Shape): Shape => ({ items })
export const map = (values: Shape): Shape => ({ values })

function unknownField(value: unknown, shape: Shape): boolean {
  if (shape === null || typeof value !== 'object' || value === null) return false
  if ('items' in shape) return Array.isArray(value) && value.some((v) => unknownField(v, shape.items))
  if ('values' in shape) return Object.values(value).some((v) => unknownField(v, shape.values))
  const record = value as { [key: string]: unknown }
  return Object.keys(record).some((k) => {
    const sub = shape.fields[k]
    return sub === undefined || unknownField(record[k], sub)
  })
}

export function strictValidate<P>(supported: readonly number[], shape: Shape): (input: unknown) => Result<P> {
  const base = placeholderValidate<P>(supported)
  return (input) => {
    const result = base(input)
    if (!result.ok || !unknownField(input, shape)) return result
    return {
      ok: false,
      gap: { reason: 'invalid_payload', text: 'The payload does not match its schema. Do not act on this card.' },
      errors: [],
    }
  }
}
