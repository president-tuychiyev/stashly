/**
 * Helpers for reading route query parameters, which Vue Router types as
 * `string | string[] | null`.
 */

/** First value of a query parameter, or null when it is absent/empty. */
export function queryString(value: unknown): string | null {
  const raw = Array.isArray(value) ? value[0] : value
  if (raw === undefined || raw === null || raw === '') return null
  return String(raw)
}

/** First value of a query parameter parsed as a finite number, else null. */
export function queryNumber(value: unknown): number | null {
  const raw = queryString(value)
  if (raw === null) return null
  const parsed = Number(raw)
  return Number.isFinite(parsed) ? parsed : null
}
