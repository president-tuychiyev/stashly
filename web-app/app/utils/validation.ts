/**
 * Pure form-validation helpers. Kept free of Vue/i18n so they stay unit testable;
 * callers map the returned issue keys to translated messages.
 */

/** Minimum length required by the admin API for account passwords. */
export const PASSWORD_MIN_LENGTH = 8

/** Number of digits in an OTP code. */
export const OTP_LENGTH = 6

/** Seconds to wait before a new OTP may be requested. */
export const OTP_RESEND_SECONDS = 60

// Deliberately permissive: one @, no spaces, a dot-separated domain.
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/

/** True when the value looks like an email address. */
export function isValidEmail(value: string | null | undefined): boolean {
  const trimmed = (value ?? '').trim()
  if (!trimmed || trimmed.length > 254) return false
  return EMAIL_PATTERN.test(trimmed)
}

/** Why a password is not acceptable, or `null` when it is fine. */
export type PasswordIssue = 'required' | 'tooShort' | 'weak'

/**
 * Password rule: at least {@link PASSWORD_MIN_LENGTH} characters, containing
 * at least one letter and one digit.
 */
export function passwordIssue(value: string | null | undefined): PasswordIssue | null {
  const password = value ?? ''
  if (!password) return 'required'
  if (password.length < PASSWORD_MIN_LENGTH) return 'tooShort'
  const hasLetter = /\p{L}/u.test(password)
  const hasDigit = /\d/.test(password)
  if (!hasLetter || !hasDigit) return 'weak'
  return null
}

/** Convenience wrapper around {@link passwordIssue}. */
export function isStrongPassword(value: string | null | undefined): boolean {
  return passwordIssue(value) === null
}

/** True when the value is exactly {@link OTP_LENGTH} digits. */
export function isValidOtp(value: string | null | undefined): boolean {
  return new RegExp(`^\\d{${OTP_LENGTH}}$`).test((value ?? '').trim())
}

/** Keep only digits, capped at {@link OTP_LENGTH} characters. */
export function normalizeOtp(value: string | null | undefined): string {
  return (value ?? '').replace(/\D/g, '').slice(0, OTP_LENGTH)
}
