import assert from 'node:assert/strict'
import test from 'node:test'

import {
  OTP_LENGTH,
  PASSWORD_MIN_LENGTH,
  isStrongPassword,
  isValidEmail,
  isValidOtp,
  normalizeOtp,
  passwordIssue,
} from '../app/utils/validation.ts'

test('isValidEmail accepts ordinary addresses', () => {
  assert.equal(isValidEmail('admin@example.com'), true)
  assert.equal(isValidEmail('  spaced@example.co.uk  '), true)
  assert.equal(isValidEmail('first.last+tag@sub.example.org'), true)
})

test('isValidEmail rejects malformed values', () => {
  for (const value of ['', '   ', null, undefined, 'admin', 'admin@', '@example.com', 'a b@example.com', 'admin@example', 'a@@b.com']) {
    assert.equal(isValidEmail(value), false, `expected ${String(value)} to be invalid`)
  }
  assert.equal(isValidEmail(`${'a'.repeat(250)}@example.com`), false)
})

test('passwordIssue enforces length plus letter and digit', () => {
  assert.equal(passwordIssue(''), 'required')
  assert.equal(passwordIssue(null), 'required')
  assert.equal(passwordIssue('a1b2c3'), 'tooShort')
  assert.equal(passwordIssue('abcdefgh'), 'weak')
  assert.equal(passwordIssue('12345678'), 'weak')
  assert.equal(passwordIssue('abcdefg1'), null)
  assert.equal(passwordIssue('Parol123!'), null)
  // Non-Latin letters count as letters.
  assert.equal(passwordIssue('парол123'), null)
})

test('passwordIssue boundary is PASSWORD_MIN_LENGTH', () => {
  const digits = '1'.repeat(PASSWORD_MIN_LENGTH - 1)
  assert.equal(passwordIssue(`a${digits}`), null)
  assert.equal(passwordIssue(`a${digits.slice(1)}`), 'tooShort')
})

test('isStrongPassword mirrors passwordIssue', () => {
  assert.equal(isStrongPassword('abcdefg1'), true)
  assert.equal(isStrongPassword('short1'), false)
})

test('isValidOtp requires exactly six digits', () => {
  assert.equal(OTP_LENGTH, 6)
  assert.equal(isValidOtp('123456'), true)
  assert.equal(isValidOtp(' 123456 '), true)
  assert.equal(isValidOtp('12345'), false)
  assert.equal(isValidOtp('1234567'), false)
  assert.equal(isValidOtp('12345a'), false)
  assert.equal(isValidOtp(''), false)
})

test('normalizeOtp strips non-digits and truncates', () => {
  assert.equal(normalizeOtp('12-34 56'), '123456')
  assert.equal(normalizeOtp('123456789'), '123456')
  assert.equal(normalizeOtp('abc'), '')
  assert.equal(normalizeOtp(undefined), '')
})
