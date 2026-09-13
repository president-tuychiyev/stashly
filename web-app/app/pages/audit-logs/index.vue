<script setup lang="ts">
import type { ApiError, AuditLog, Paginated } from '~/types/api'

const api = useApi()
const { t } = useI18n()

const ACTIONS = [
  'client.create',
  'client.update',
  'client.delete',
  'client.reset_password',
  'file.upload',
  'file.delete',
  'archive.create',
  'archive.delete',
  'auth.login',
]

const items = ref<AuditLog[]>([])
const total = ref(0)
const page = ref(1)
const perPage = ref(20)
const actorType = ref<'user' | 'client' | null>(null)
const action = ref<string | null>(null)
const loading = ref(true)
const error = ref<ApiError | null>(null)
const expanded = ref<number[]>([])

const actorOptions = computed(() => [
  { label: t('audit.actorTypes.user'), value: 'user' },
  { label: t('audit.actorTypes.client'), value: 'client' },
])
const actionOptions = ACTIONS.map((value) => ({ label: value, value }))

// Every reload goes through this loader; stale responses are dropped.
let requestId = 0

const load = async () => {
  const current = ++requestId
  loading.value = true
  error.value = null
  try {
    const response = await api.get<Paginated<AuditLog>>('/admin/audit-logs', {
      actor_type: actorType.value || undefined,
      action: action.value || undefined,
      page: page.value,
      per_page: perPage.value,
    })
    if (current !== requestId) return
    items.value = response.data ?? []
    total.value = response.meta?.total ?? 0
  } catch (e) {
    if (current !== requestId) return
    error.value = e as ApiError
    items.value = []
    total.value = 0
  } finally {
    if (current === requestId) loading.value = false
  }
}

onMounted(load)
// Filter changes jump back to page 1; the single watcher below still fires once.
watch([actorType, action, perPage], () => {
  page.value = 1
})
watch([actorType, action, page, perPage], load)

const toggle = (id: number) => {
  expanded.value = expanded.value.includes(id)
    ? expanded.value.filter((item) => item !== id)
    : [...expanded.value, id]
}

const pretty = (details: unknown) => JSON.stringify(details ?? {}, null, 2)
</script>

<template>
  <div>
    <PageHeader :title="$t('audit.title')" :subtitle="$t('audit.subtitle')" />

    <n-card class="mb-4 rounded-xl">
      <div class="flex flex-col gap-3 sm:flex-row">
        <n-select
          v-model:value="actorType"
          clearable
          :options="actorOptions"
          :placeholder="$t('audit.actorType')"
          class="sm:max-w-[12rem]"
        />
        <n-select
          v-model:value="action"
          clearable
          :options="actionOptions"
          :placeholder="$t('audit.action')"
          class="sm:max-w-[16rem]"
        />
      </div>
    </n-card>

    <ApiErrorAlert :error="error" @retry="load" />

    <n-spin :show="loading">
      <n-empty v-if="!items.length && !loading" :description="$t('common.noData')" class="py-12" />

      <!-- Mobile: card list -->
      <div v-else class="space-y-3 md:hidden">
        <n-card v-for="log in items" :key="log.id" size="small" class="rounded-xl">
          <p class="font-medium">{{ log.action }}</p>
          <p class="text-xs opacity-60">
            {{ log.actor?.name || '—' }} ({{ log.actor?.type || '—' }}) · {{ log.ip || '—' }}
          </p>
          <p class="text-xs opacity-60">{{ formatDate(log.created_at) }}</p>
          <p class="text-xs opacity-60">{{ log.subject_type || '—' }} #{{ log.subject_id ?? '—' }}</p>
          <n-button size="tiny" secondary class="mt-2" @click="toggle(log.id)">
            {{ expanded.includes(log.id) ? $t('audit.hideDetails') : $t('audit.showDetails') }}
          </n-button>
          <pre
            v-if="expanded.includes(log.id)"
            class="mt-2 overflow-x-auto rounded bg-black/5 p-2 text-xs dark:bg-white/5"
          >{{ pretty(log.details) }}</pre>
        </n-card>
      </div>

      <!-- Desktop: table -->
      <div v-if="items.length || loading" class="hidden md:block">
        <n-card class="rounded-xl" content-style="padding: 0;">
          <div class="overflow-x-auto">
            <n-table :single-line="false" size="small">
              <thead>
                <tr>
                  <th>#</th>
                  <th>{{ $t('audit.action') }}</th>
                  <th>{{ $t('audit.actor') }}</th>
                  <th>{{ $t('audit.subject') }}</th>
                  <th>IP</th>
                  <th>{{ $t('common.createdAt') }}</th>
                  <th class="text-right">{{ $t('audit.details') }}</th>
                </tr>
              </thead>
              <tbody>
                <template v-for="log in items" :key="log.id">
                  <tr>
                    <td>{{ log.id }}</td>
                    <td class="whitespace-nowrap font-medium">{{ log.action }}</td>
                    <td class="whitespace-nowrap">
                      {{ log.actor?.name || '—' }}
                      <span class="opacity-60">({{ log.actor?.type || '—' }})</span>
                    </td>
                    <td class="whitespace-nowrap opacity-70">
                      {{ log.subject_type || '—' }} <span v-if="log.subject_id">#{{ log.subject_id }}</span>
                    </td>
                    <td>{{ log.ip || '—' }}</td>
                    <td class="whitespace-nowrap opacity-70">{{ formatDate(log.created_at) }}</td>
                    <td>
                      <div class="flex justify-end">
                        <n-button size="tiny" quaternary @click="toggle(log.id)">
                          {{ expanded.includes(log.id) ? $t('audit.hideDetails') : $t('audit.showDetails') }}
                        </n-button>
                      </div>
                    </td>
                  </tr>
                  <tr v-if="expanded.includes(log.id)">
                    <td colspan="7">
                      <pre class="overflow-x-auto rounded bg-black/5 p-3 text-xs dark:bg-white/5">{{ pretty(log.details) }}</pre>
                    </td>
                  </tr>
                </template>
              </tbody>
            </n-table>
          </div>
        </n-card>
      </div>

      <div v-if="total > perPage" class="mt-4 flex justify-center sm:justify-end">
        <n-pagination
          v-model:page="page"
          v-model:page-size="perPage"
          :item-count="total"
          :page-sizes="[20, 50, 100]"
          show-size-picker
        />
      </div>
    </n-spin>
  </div>
</template>
