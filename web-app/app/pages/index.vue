<script setup lang="ts">
import { ArchiveOutline, DocumentsOutline, PeopleOutline, ServerOutline, SyncOutline } from '@vicons/ionicons5'
import type { ApiError, DashboardStats } from '~/types/api'

const api = useApi()
const { t, locale } = useI18n()
const message = useMessage()

const stats = ref<DashboardStats | null>(null)
const loading = ref(true)
const error = ref<ApiError | null>(null)
const { running: syncing, run: runStorageSync } = useStorageSync()
// Storage sync is a super-admin-only endpoint per the API contract.
const { isSuperAdmin } = useAuth()

const load = async () => {
  loading.value = true
  error.value = null
  try {
    const response = await api.get<{ data: DashboardStats }>('/admin/dashboard')
    stats.value = response.data
  } catch (e) {
    error.value = e as ApiError
    stats.value = null
  } finally {
    loading.value = false
  }
}

onMounted(load)

const cards = computed(() => [
  {
    key: 'clients',
    label: t('dashboard.clients'),
    value: (stats.value?.clients_count ?? 0).toLocaleString(locale.value),
    icon: PeopleOutline,
  },
  {
    key: 'files',
    label: t('dashboard.files'),
    value: (stats.value?.files_count ?? 0).toLocaleString(locale.value),
    icon: DocumentsOutline,
  },
  { key: 'size', label: t('dashboard.totalSize'), value: formatBytes(stats.value?.total_size ?? 0), icon: ServerOutline },
  {
    key: 'archives',
    label: t('dashboard.pendingArchives'),
    value: (stats.value?.archives_pending ?? 0).toLocaleString(locale.value),
    icon: ArchiveOutline,
  },
])

// Usage by client: the dashboard endpoint has no per-client quota, so the
// largest client is used as the reference for the bar width.
const maxClientSize = computed(() =>
  Math.max(1, ...(stats.value?.by_client ?? []).map((item) => item.size)),
)

const uploads = computed(() => stats.value?.uploads_last_30_days ?? [])
const maxUploads = computed(() => Math.max(1, ...uploads.value.map((item) => item.count)))

const runSync = () =>
  runStorageSync({
    onDone: async (result) => {
      message.success(
        t('storage.syncResult', {
          orphans: result.orphans_on_disk,
          missing: result.missing_on_disk,
          records: result.removed_records,
          files: result.removed_files,
        }),
      )
      if (result.errors) message.warning(`${t('storage.errors')}: ${result.errors}`)
      await load()
    },
    onError: (e) => message.error(e.message),
  })
</script>

<template>
  <div>
    <PageHeader :title="$t('dashboard.title')" :subtitle="$t('dashboard.subtitle')">
      <template #actions>
        <n-button v-if="isSuperAdmin" :loading="syncing" secondary @click="runSync">
          <template #icon>
            <n-icon><SyncOutline /></n-icon>
          </template>
          {{ $t('storage.sync') }}
        </n-button>
      </template>
    </PageHeader>

    <ApiErrorAlert :error="error" @retry="load" />

    <n-spin :show="loading">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <n-card v-for="card in cards" :key="card.key" class="rounded-xl">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-teal-500/10 p-3 text-teal-500">
              <n-icon size="24"><component :is="card.icon" /></n-icon>
            </div>
            <div class="min-w-0">
              <p class="truncate text-xs uppercase opacity-60">{{ card.label }}</p>
              <p class="truncate text-xl font-semibold">{{ card.value }}</p>
            </div>
          </div>
        </n-card>
      </div>

      <div class="mt-4 grid grid-cols-1 gap-4 xl:grid-cols-2">
        <n-card :title="$t('dashboard.usageByClient')" class="rounded-xl">
          <n-empty v-if="!stats?.by_client?.length" :description="$t('common.noData')" />
          <div v-else class="space-y-4">
            <div v-for="row in stats.by_client" :key="row.client_id">
              <div class="mb-1 flex items-center justify-between gap-2 text-sm">
                <NuxtLink :to="`/clients/${row.client_id}`" class="truncate hover:text-teal-500">
                  {{ row.name }}
                </NuxtLink>
                <span class="shrink-0 opacity-70">
                  {{ formatBytes(row.size) }} · {{ row.files_count }} {{ $t('dashboard.filesShort') }}
                </span>
              </div>
              <n-progress
                type="line"
                :percentage="Math.round((row.size / maxClientSize) * 100)"
                :show-indicator="false"
                :height="8"
              />
            </div>
          </div>
        </n-card>

        <n-card :title="$t('dashboard.uploads30d')" class="rounded-xl">
          <n-empty v-if="!uploads.length" :description="$t('common.noData')" />
          <div v-else class="overflow-x-auto">
            <div class="flex h-40 min-w-[480px] items-end gap-[3px]">
              <div
                v-for="day in uploads"
                :key="day.date"
                class="group relative flex-1"
                :title="`${day.date}: ${day.count} · ${formatBytes(day.size)}`"
              >
                <div
                  class="w-full rounded-t bg-teal-500/70 transition-colors group-hover:bg-teal-500"
                  :style="{ height: `${Math.max(2, (day.count / maxUploads) * 150)}px` }"
                />
              </div>
            </div>
            <div class="mt-2 flex min-w-[480px] justify-between text-xs opacity-60">
              <span>{{ uploads[0]?.date }}</span>
              <span>{{ uploads[uploads.length - 1]?.date }}</span>
            </div>
          </div>
        </n-card>
      </div>
    </n-spin>
  </div>
</template>
