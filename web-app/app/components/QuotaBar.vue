<script setup lang="ts">
const props = defineProps<{ used: number; quota: number | null }>()

const percent = computed(() => usagePercent(props.used, props.quota))
const status = computed(() => {
  const value = percent.value
  if (value === null) return 'success'
  if (value >= 90) return 'error'
  if (value >= 70) return 'warning'
  return 'success'
})
</script>

<template>
  <div class="min-w-[140px]">
    <div class="mb-1 flex justify-between gap-2 text-xs opacity-70">
      <span>{{ formatBytes(used) }}</span>
      <span>{{ quota ? formatBytes(quota) : $t('clients.unlimited') }}</span>
    </div>
    <n-progress
      type="line"
      :status="status"
      :percentage="percent ?? 0"
      :show-indicator="false"
      :height="6"
    />
  </div>
</template>
