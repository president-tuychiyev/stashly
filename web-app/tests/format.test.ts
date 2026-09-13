import assert from 'node:assert/strict'
import test from 'node:test'

import { FILE_SIZE_UNITS, QUOTA_UNITS, fromBytes, toBytes } from '../app/utils/format.ts'

const GB = 1024 ** 3
const MB = 1024 ** 2
const KB = 1024

test('fromBytes picks the largest clean unit', () => {
  assert.deepEqual(fromBytes(5 * GB, QUOTA_UNITS, 'GB'), { amount: 5, unit: 'GB' })
  assert.deepEqual(fromBytes(2 * 1024 ** 4, QUOTA_UNITS, 'GB'), { amount: 2, unit: 'TB' })
  assert.deepEqual(fromBytes(1.5 * GB, QUOTA_UNITS, 'GB'), { amount: 1.5, unit: 'GB' })
  assert.deepEqual(fromBytes(512 * MB, QUOTA_UNITS, 'GB'), { amount: 512, unit: 'MB' })
})

test('fromBytes keeps at most two decimals', () => {
  // 1.25 GB is clean in GB; 1.333… GB is not, so it drops to MB.
  assert.deepEqual(fromBytes(1.25 * GB, QUOTA_UNITS, 'GB'), { amount: 1.25, unit: 'GB' })
  assert.deepEqual(fromBytes(1365 * MB, QUOTA_UNITS, 'GB'), { amount: 1365, unit: 'MB' })
})

test('fromBytes honours the allowed unit list', () => {
  assert.deepEqual(fromBytes(700 * KB, FILE_SIZE_UNITS, 'MB'), { amount: 700, unit: 'KB' })
  assert.deepEqual(fromBytes(10 * MB, FILE_SIZE_UNITS, 'MB'), { amount: 10, unit: 'MB' })
  // TB is not offered for the per-file limit.
  assert.equal(fromBytes(1024 ** 4, FILE_SIZE_UNITS, 'MB').unit, 'GB')
})

test('fromBytes handles null, zero and sub-unit values', () => {
  assert.deepEqual(fromBytes(null, QUOTA_UNITS, 'GB'), { amount: null, unit: 'GB' })
  assert.deepEqual(fromBytes(undefined, FILE_SIZE_UNITS, 'MB'), { amount: null, unit: 'MB' })
  assert.deepEqual(fromBytes(0, QUOTA_UNITS, 'GB'), { amount: 0, unit: 'GB' })
  assert.deepEqual(fromBytes(500, FILE_SIZE_UNITS, 'MB'), { amount: 500 / KB, unit: 'KB' })
})

test('fromBytes round-trips through toBytes', () => {
  const values = [0, 500, 1234567, 5 * GB, 1.5 * GB, 700 * KB, 3 * 1024 ** 4, 1365 * MB]
  for (const value of values) {
    const quota = fromBytes(value, QUOTA_UNITS, 'GB')
    assert.equal(toBytes(quota.amount, quota.unit), value, `quota round trip for ${value}`)
    const maxFile = fromBytes(value, FILE_SIZE_UNITS, 'MB')
    assert.equal(toBytes(maxFile.amount, maxFile.unit), value, `max file round trip for ${value}`)
  }
})
