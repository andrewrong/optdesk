// The display suffix of a payload unit id: a short professional text or none, never the id itself. The ids in the
// payloads are ids, not display text; an id that is not in the table gets no suffix.
const suffixes = new Map<string, string>([
  ['percent', '%'],
  ['percentage_points', ' pp'],
  ['basis_points', ' bp'],
])

export function unitSuffix(unit: string): string {
  return suffixes.get(unit) ?? ''
}

const prefixes = new Map<string, string>([
  ['usd', '$'],
  ['usd_signed', '$'],
])

// The display prefix of a unit id: the dollar sign of money, none for any other id.
export function unitPrefix(unit: string): string {
  return prefixes.get(unit) ?? ''
}
