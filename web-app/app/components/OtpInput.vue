<script setup lang="ts">
/**
 * Six-digit code entry. Wraps Naive UI's `n-input-otp`, keeping an internal
 * `string[]` of length `OTP_LENGTH` as the source of truth for its `:value`
 * (padded arrays are what the component emits natively) and exposing a plain
 * joined string to the rest of the app.
 */
const value = defineModel<string>('value', { default: '' })

withDefaults(defineProps<{ disabled?: boolean; ariaLabel?: string }>(), { disabled: false })

const emit = defineEmits<{ finish: [code: string] }>()

const { t } = useI18n()

const toChars = (raw: string | null | undefined): string[] => {
  const normalized = normalizeOtp(raw)
  return Array.from({ length: OTP_LENGTH }, (_, index) => normalized[index] ?? '')
}

const chars = ref<string[]>(toChars(value.value))

// The model can be reset (or otherwise changed) from outside the component
// (e.g. clearing the field after a failed submit); re-sync when that happens.
watch(value, (next) => {
  if (normalizeOtp(next) !== chars.value.join('')) {
    chars.value = toChars(next)
  }
})

const onUpdate = (next: string[]) => {
  chars.value = next
  value.value = next.join('')
}

const onFinish = (next: string[]) => emit('finish', normalizeOtp(next.join('')))
</script>

<template>
  <div role="group" :aria-label="ariaLabel">
    <n-input-otp
      :value="chars"
      :length="OTP_LENGTH"
      :disabled="disabled"
      block
      size="large"
      @update:value="onUpdate"
      @finish="onFinish"
    >
      <template #default="slotProps">
        <n-input
          v-bind="slotProps"
          :input-props="{
            inputmode: 'numeric',
            autocomplete: 'one-time-code',
            'aria-label': t('auth.codeDigit', { n: slotProps.index + 1 }),
          }"
        />
      </template>
    </n-input-otp>
  </div>
</template>
