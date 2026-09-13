/**
 * Formatting helpers shared across the admin panel.
 */

const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

/** Convert a byte count into a short human readable string. */
export function formatBytes(bytes: number | null | undefined, fractionDigits = 1): string {
  if (bytes === null || bytes === undefined || Number.isNaN(bytes)) return '—'
  if (bytes <= 0) return '0 B'

  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < BYTE_UNITS.length - 1) {
    value /= 1024
    unit++
  }

  const digits = unit === 0 ? 0 : fractionDigits
  return `${value.toFixed(digits).replace(/\.0+$/, '')} ${BYTE_UNITS[unit]}`
}

export type ByteUnit = 'B' | 'KB' | 'MB' | 'GB' | 'TB'

const BYTE_FACTORS: Record<ByteUnit, number> = {
  B: 1,
  KB: 1024,
  MB: 1024 ** 2,
  GB: 1024 ** 3,
  TB: 1024 ** 4,
}

/** Units offered for a client storage quota, largest first. */
export const QUOTA_UNITS = ['TB', 'GB', 'MB'] as const
/** Units offered for the per-file size limit, largest first. */
export const FILE_SIZE_UNITS = ['GB', 'MB', 'KB'] as const

/** Multiply a numeric amount by its unit to get bytes. */
export function toBytes(amount: number | null, unit: ByteUnit): number | null {
  if (amount === null || amount === undefined || Number.isNaN(amount)) return null
  return Math.round(amount * (BYTE_FACTORS[unit] ?? 1))
}

/**
 * Split a byte count into an `{ amount, unit }` pair for editing in forms.
 *
 * Picks the largest of `units` for which the amount stays a clean number
 * (at most two decimals), so the value survives a `fromBytes` → `toBytes`
 * round trip. Falls back to the smallest allowed unit, keeping full
 * precision, when no unit yields a clean amount.
 */
export function fromBytes<U extends ByteUnit>(
  bytes: number | null | undefined,
  units: readonly U[],
  fallbackUnit: U = units[units.length - 1] as U,
): { amount: number | null; unit: U } {
  if (bytes === null || bytes === undefined || Number.isNaN(bytes)) {
    return { amount: null, unit: fallbackUnit }
  }
  if (bytes === 0) return { amount: 0, unit: fallbackUnit }

  const smallest = units[units.length - 1] as U
  for (const unit of units) {
    const factor = BYTE_FACTORS[unit]
    // Clean means: at most two decimals once divided by the unit factor.
    if (bytes >= factor && (bytes * 100) % factor === 0) {
      return { amount: Math.round((bytes * 100) / factor) / 100, unit }
    }
  }
  return { amount: bytes / BYTE_FACTORS[smallest], unit: smallest }
}

/** Percentage of quota used, capped at 100. Returns null for unlimited quota. */
export function usagePercent(used: number, quota: number | null | undefined): number | null {
  if (!quota) return null
  return Math.min(100, Math.round((used / quota) * 100))
}

/** RFC3339 timestamp to a readable local date-time. */
export function formatDate(value: string | null | undefined, withTime = true): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  const datePart = date.toLocaleDateString(undefined, { year: 'numeric', month: '2-digit', day: '2-digit' })
  if (!withTime) return datePart
  const timePart = date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
  return `${datePart} ${timePart}`
}

/** Whole days between now and a future timestamp (0 when past). */
export function daysUntil(value: string | null | undefined): number | null {
  if (!value) return null
  const target = new Date(value).getTime()
  if (Number.isNaN(target)) return null
  return Math.max(0, Math.ceil((target - Date.now()) / 86_400_000))
}

export type MimeKind = 'image' | 'video' | 'audio' | 'pdf' | 'archive' | 'text' | 'spreadsheet' | 'document' | 'file'

/** Map a MIME type to a coarse kind used to pick an icon. */
export function mimeKind(mime: string | null | undefined): MimeKind {
  const value = (mime || '').toLowerCase()
  if (value.startsWith('image/')) return 'image'
  if (value.startsWith('video/')) return 'video'
  if (value.startsWith('audio/')) return 'audio'
  if (value === 'application/pdf') return 'pdf'
  if (/zip|rar|7z|tar|gzip|compressed/.test(value)) return 'archive'
  if (/spreadsheet|excel|csv/.test(value)) return 'spreadsheet'
  if (/word|opendocument\.text|rtf/.test(value)) return 'document'
  if (value.startsWith('text/') || /json|xml|javascript/.test(value)) return 'text'
  return 'file'
}

/** True when the file can be shown as an inline thumbnail. */
export function isPreviewable(mime: string | null | undefined, url: string | null | undefined): boolean {
  return Boolean(url) && mimeKind(mime) === 'image'
}

/** Join an `ApiError.errors[field]` value into text, whether array or plain string. */
export function joinErrorMessages(value: string[] | string | undefined): string {
  if (!value) return ''
  return Array.isArray(value) ? value.join(', ') : value
}
