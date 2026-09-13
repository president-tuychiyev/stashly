<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <div class="mb-4 flex flex-col gap-3 sm:mb-6 sm:flex-row sm:items-center sm:justify-between">
    <div class="min-w-0">
      <h1 class="truncate text-xl font-semibold sm:text-2xl">{{ title }}</h1>
      <p v-if="subtitle" class="mt-0.5 text-sm opacity-60">{{ subtitle }}</p>
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>
