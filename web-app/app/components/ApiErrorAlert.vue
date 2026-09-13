<script setup lang="ts">
import type { ApiError } from '~/types/api'

defineProps<{ error: ApiError | null }>()
const emit = defineEmits<{ retry: [] }>()

const { t } = useI18n()

/** Translated field label, falling back to the raw API key when unknown. */
const fieldLabel = (field: string): string => {
  const key = `fields.${field}`
  const translated = t(key)
  return translated === key ? field : translated
}
</script>

<template>
  <n-alert v-if="error" type="error" :title="$t('common.errorTitle')" class="mb-4">
    <p>{{ error.message }}</p>
    <ul v-if="error.errors" class="mt-2 list-inside list-disc text-sm">
      <li v-for="(messages, field) in error.errors" :key="field">
        {{ fieldLabel(String(field)) }}: {{ joinErrorMessages(messages) }}
      </li>
    </ul>
    <div class="mt-3">
      <n-button size="small" secondary @click="emit('retry')">{{ $t('common.retry') }}</n-button>
    </div>
  </n-alert>
</template>
