<script setup lang="ts">
import type { UserStatus } from '~/types/api'

const props = defineProps<{ status: UserStatus }>()

const type = computed(() => {
  switch (props.status) {
    case 'active':
      return 'success'
    case 'pending':
      return 'warning'
    case 'blocked':
      return 'error'
    default:
      return 'default'
  }
})
</script>

<template>
  <n-tag :type="type" size="small" round>{{ $t(`users.statuses.${status}`) }}</n-tag>
</template>
