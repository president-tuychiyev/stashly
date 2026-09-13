<script setup lang="ts">
import { SyncOutline } from '@vicons/ionicons5'

const { t } = useI18n()
const message = useMessage()
const { running, result, error, run: runSync } = useStorageSync()
const { isSuperAdmin, ensureUser, user } = useAuth()

const forbidden = computed(() => Boolean(user.value) && !isSuperAdmin.value)

// The API 403s for `admin`; hide the destructive control client-side too.
onMounted(async () => {
  try {
    await ensureUser()
  } catch {
    // Non-fatal: the sync request itself will report the problem.
  }
})

const rows = computed(() => [
  { key: 'orphans_on_disk', label: t('storage.orphansOnDisk'), value: result.value?.orphans_on_disk ?? 0 },
  { key: 'missing_on_disk', label: t('storage.missingOnDisk'), value: result.value?.missing_on_disk ?? 0 },
  { key: 'removed_records', label: t('storage.removedRecords'), value: result.value?.removed_records ?? 0 },
  { key: 'removed_files', label: t('storage.removedFiles'), value: result.value?.removed_files ?? 0 },
  { key: 'errors', label: t('storage.errors'), value: result.value?.errors ?? 0 },
])

const run = () => runSync({ onDone: () => message.success(t('storage.syncDone')) })
</script>

<template>
  <div>
    <PageHeader :title="$t('storage.title')" :subtitle="$t('storage.subtitle')" />

    <n-alert v-if="forbidden" type="error" :title="$t('users.forbiddenTitle')">
      {{ $t('users.forbidden') }}
    </n-alert>

    <template v-else>
    <n-alert v-if="error" type="error" class="mb-4">{{ error.message }}</n-alert>

    <n-card class="max-w-2xl rounded-xl">
      <p class="mb-4 text-sm opacity-70">{{ $t('storage.description') }}</p>

      <n-button type="primary" :loading="running" @click="run">
        <template #icon><n-icon><SyncOutline /></n-icon></template>
        {{ $t('storage.sync') }}
      </n-button>

      <div v-if="result" class="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div
          v-for="row in rows"
          :key="row.key"
          class="rounded-lg border border-gray-200 p-3 dark:border-dark-100"
        >
          <p class="text-xs uppercase opacity-60">{{ row.label }}</p>
          <p class="text-xl font-semibold">{{ row.value }}</p>
        </div>
      </div>
    </n-card>
    </template>
  </div>
</template>
