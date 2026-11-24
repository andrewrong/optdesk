// An RFC 3339 `YYYY-MM-DDTHH:mm:ss…` string shown as `YYYY-MM-DD HH:mm`, cut from the text as given
// (no Date, no zone arithmetic). Any other string comes back unchanged.
export function shownTime(value: string): string {
  return /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/.test(value) ? `${value.slice(0, 10)} ${value.slice(11, 16)}` : value
}
