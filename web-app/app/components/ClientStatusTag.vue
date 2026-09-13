<script setup lang="ts">
import type { ClientStatus } from '~/types/api'

const props = defineProps<{ status: ClientStatus }>()

const type = computed(() => {
  switch (props.status) {
    case 'active':
      return 'success'
    case 'inactive':
      return 'warning'
    case 'blocked':
      return 'error'
    default:
      return 'default'
  }
})
</script>

<template>
  <n-tag :type="type" size="small" round>{{ $t(`clients.statuses.${status}`) }}</n-tag>
</template>
