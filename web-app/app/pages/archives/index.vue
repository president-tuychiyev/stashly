<script setup lang="ts">
import { AddOutline, DownloadOutline, TrashOutline } from '@vicons/ionicons5'
import type { TreeOption } from 'naive-ui'
import type { ApiError, Archive, ArchiveStatus, ArchiveStatusItem, Folder, Paginated } from '~/types/api'

const api = useApi()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const { options: clientOptions, load: loadClients } = useClientOptions()

const clientId = ref<number | null>(queryNumber(route.query.client_id))
const status = ref<ArchiveStatus | null>(null)
const page = ref(1)
const perPage = ref(20)

const items = ref<Archive[]>([])
const total = ref(0)
const loading = ref(true)
const error = ref<ApiError | null>(null)

const statusOptions = computed(() =>
  (['pending', 'processing', 'done', 'failed', 'expired'] as ArchiveStatus[]).map((value) => ({
    label: t(`archives.statuses.${value}`),
    value,
  })),
)

const tagType = (value: ArchiveStatus) => {
  switch (value) {
    case 'done':
      return 'success'
    case 'failed':
      return 'error'
    case 'processing':
      return 'info'
    case 'pending':
      return 'warning'
    default:
      return 'default'
  }
}

// Every reload goes through this loader; stale responses are dropped.
let requestId = 0

const load = async () => {
  const current = ++requestId
  loading.value = true
  error.value = null
  try {
    const response = await api.get<Paginated<Archive>>('/admin/archives', {
      client_id: clientId.value || undefined,
      status: status.value || undefined,
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
    if (current === requestId) {
      loading.value = false
      syncPolling()
    }
  }
}

onMounted(async () => {
  await loadClients()
  await load()
})

// Filter changes jump back to page 1; the single watcher below still fires once.
watch([clientId, status, perPage], () => {
  page.value = 1
})
watch([clientId, status, page, perPage], load)

// Keep the client filter in the URL so back/forward restores it.
watch(clientId, (value) => {
  if (queryNumber(route.query.client_id) === value) return
  const query = { ...route.query }
  if (value === null) delete query.client_id
  else query.client_id = String(value)
  router.replace({ query })
})
watch(
  () => route.query.client_id,
  (value) => {
    const next = queryNumber(value)
    if (next !== clientId.value) clientId.value = next
  },
)

/* ---------------------------------------------------------------- polling */

let pollTimer: ReturnType<typeof setTimeout> | undefined
let polling = false
let pollingStopped = false

const activeIds = computed(() =>
  items.value.filter((item) => item.status === 'pending' || item.status === 'processing').map((item) => item.id),
)

const poll = async () => {
  pollTimer = undefined
  if (polling || !activeIds.value.length) return
  polling = true
  try {
    const response = await api.get<{ data: ArchiveStatusItem[] }>('/admin/archives/status', {
      ids: activeIds.value.join(','),
    })
    let finished = false
    for (const update of response.data ?? []) {
      const target = items.value.find((item) => item.id === update.id)
      if (!target) continue
      if (target.status !== update.status && (update.status === 'done' || update.status === 'failed')) {
        finished = true
      }
      target.status = update.status
      target.progress = update.progress
      target.url = update.url
      target.expires_at = update.expires_at
    }
    // Finished archives carry size/files_count the status endpoint does not return.
    if (finished) await load()
  } catch {
    // Polling failures are silent; the next tick retries.
  } finally {
    polling = false
    syncPolling()
  }
}

// Self-scheduling chain: one tick is queued at a time, never while a poll runs.
function syncPolling() {
  if (pollingStopped || polling) return
  if (activeIds.value.length && !pollTimer) {
    pollTimer = setTimeout(poll, 5000)
  } else if (!activeIds.value.length && pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = undefined
  }
}

onScopeDispose(() => {
  pollingStopped = true
  if (pollTimer) clearTimeout(pollTimer)
  pollTimer = undefined
})

/* ---------------------------------------------------------------- create */

const createOpen = ref(false)
const creating = ref(false)
const createError = ref<ApiError | null>(null)
const formClientId = ref<number | null>(null)
const wholeRoot = ref(false)
const checkedFolders = ref<string[]>([])
const treeData = ref<TreeOption[]>([])
const treeLoading = ref(false)

const fetchFolders = async (parent?: string): Promise<TreeOption[]> => {
  if (!formClientId.value) return []
  const response = await api.get<{ data: Folder[] }>('/admin/folders', {
    client_id: formClientId.value,
    parent: parent || undefined,
  })
  return (response.data ?? []).map((item) => ({
    key: item.path,
    label: `${item.name} (${item.files_count})`,
    isLeaf: false,
  }))
}

const loadRootFolders = async () => {
  treeLoading.value = true
  createError.value = null
  try {
    treeData.value = await fetchFolders()
  } catch (e) {
    createError.value = e as ApiError
    treeData.value = []
  } finally {
    treeLoading.value = false
  }
}

// Lazy-load children when a node is expanded.
const onTreeLoad = async (node: TreeOption) => {
  try {
    const children = await fetchFolders(String(node.key))
    node.children = children
    if (!children.length) node.isLeaf = true
  } catch (e) {
    node.isLeaf = true
    message.error((e as ApiError).message)
  }
}

// Single path for (re)loading the folder tree: called on dialog open and
// whenever the client select changes, never both for the same change.
const onFormClientChange = (id: number | null) => {
  formClientId.value = id
  checkedFolders.value = []
  treeData.value = []
  if (id) loadRootFolders()
}

const openCreate = () => {
  wholeRoot.value = false
  createError.value = null
  createOpen.value = true
  onFormClientChange(clientId.value)
}

const submitCreate = async () => {
  if (!formClientId.value) {
    createError.value = { message: t('archives.pickClient') }
    return
  }
  const folders = wholeRoot.value ? ['*'] : checkedFolders.value
  if (!folders.length) {
    createError.value = { message: t('archives.pickFolders') }
    return
  }

  creating.value = true
  createError.value = null
  try {
    await api.post<{ data: Archive }>('/admin/archives', { client_id: formClientId.value, folders })
    message.success(t('archives.created'))
    createOpen.value = false
    if (page.value === 1) await load()
    else page.value = 1
  } catch (e) {
    createError.value = e as ApiError
  } finally {
    creating.value = false
  }
}

/* ---------------------------------------------------------------- actions */

const download = async (archive: Archive) => {
  try {
    await api.download(`/admin/archives/${archive.id}/download`, `archive-${archive.id}.zip`)
  } catch (e) {
    message.error((e as ApiError).message)
  }
}

const removeArchive = (archive: Archive) => {
  dialog.error({
    title: t('archives.delete'),
    content: t('archives.deleteConfirm', { id: archive.id }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await api.del(`/admin/archives/${archive.id}`)
        message.success(t('archives.deleted'))
        await load()
      } catch (e) {
        message.error((e as ApiError).message)
      }
    },
  })
}
</script>

<template>
  <div>
    <PageHeader :title="$t('archives.title')" :subtitle="$t('archives.subtitle')">
      <template #actions>
        <n-button type="primary" @click="openCreate">
          <template #icon><n-icon><AddOutline /></n-icon></template>
          {{ $t('archives.create') }}
        </n-button>
      </template>
    </PageHeader>

    <n-card class="mb-4 rounded-xl">
      <div class="flex flex-col gap-3 sm:flex-row">
        <n-select
          v-model:value="clientId"
          filterable
          clearable
          :options="clientOptions"
          :placeholder="$t('archives.client')"
          class="sm:max-w-sm"
        />
        <n-select
          v-model:value="status"
          clearable
          :options="statusOptions"
          :placeholder="$t('archives.status')"
          class="sm:max-w-[12rem]"
        />
      </div>
    </n-card>

    <ApiErrorAlert :error="error" @retry="load" />

    <n-spin :show="loading">
      <n-empty v-if="!items.length && !loading" :description="$t('common.noData')" class="py-12" />

      <!-- Mobile: card list -->
      <div v-else class="space-y-3 md:hidden">
        <n-card v-for="archive in items" :key="archive.id" size="small" class="rounded-xl">
          <div class="mb-2 flex items-start justify-between gap-2">
            <div class="min-w-0">
              <p class="font-medium">#{{ archive.id }}</p>
              <p class="truncate text-xs opacity-60">{{ archive.client?.name || `client ${archive.client_id}` }}</p>
            </div>
            <n-tag size="small" round :type="tagType(archive.status)">
              {{ $t(`archives.statuses.${archive.status}`) }}
            </n-tag>
          </div>

          <p class="truncate text-xs opacity-70">{{ (archive.folders || []).join(', ') || '*' }}</p>

          <n-progress
            v-if="archive.status === 'processing' || archive.status === 'pending'"
            class="mt-2"
            type="line"
            :percentage="archive.progress || 0"
            :height="6"
          />
          <p v-if="archive.status === 'failed'" class="mt-2 text-xs text-red-500">{{ archive.error }}</p>
          <p v-if="archive.status === 'done' && archive.expires_at" class="mt-2 text-xs opacity-60">
            {{ $t('archives.expiresIn', { days: daysUntil(archive.expires_at) ?? 0 }) }}
          </p>

          <div class="mt-3 flex flex-wrap gap-2">
            <n-button v-if="archive.status === 'done'" size="tiny" secondary @click="download(archive)">
              {{ $t('common.download') }}
            </n-button>
            <n-button size="tiny" secondary type="error" @click="removeArchive(archive)">
              {{ $t('common.delete') }}
            </n-button>
          </div>
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
                  <th>{{ $t('archives.client') }}</th>
                  <th>{{ $t('archives.folders') }}</th>
                  <th class="min-w-[180px]">{{ $t('archives.status') }}</th>
                  <th>{{ $t('archives.filesCount') }}</th>
                  <th>{{ $t('archives.size') }}</th>
                  <th>{{ $t('common.createdAt') }}</th>
                  <th class="text-right">{{ $t('common.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="archive in items" :key="archive.id">
                  <td>{{ archive.id }}</td>
                  <td class="max-w-[12rem] truncate">
                    {{ archive.client?.name || `#${archive.client_id}` }}
                  </td>
                  <td class="max-w-[16rem] truncate text-xs opacity-70">
                    {{ (archive.folders || []).join(', ') || '*' }}
                  </td>
                  <td>
                    <div class="flex items-center gap-2">
                      <n-tooltip v-if="archive.status === 'failed'" trigger="hover">
                        <template #trigger>
                          <n-tag size="small" round :type="tagType(archive.status)">
                            {{ $t(`archives.statuses.${archive.status}`) }}
                          </n-tag>
                        </template>
                        {{ archive.error || $t('archives.unknownError') }}
                      </n-tooltip>
                      <n-tag v-else size="small" round :type="tagType(archive.status)">
                        {{ $t(`archives.statuses.${archive.status}`) }}
                      </n-tag>
  
                      <n-progress
                        v-if="archive.status === 'processing' || archive.status === 'pending'"
                        class="w-24"
                        type="line"
                        :percentage="archive.progress || 0"
                        :height="6"
                        :show-indicator="false"
                      />
                      <span v-else-if="archive.status === 'done' && archive.expires_at" class="text-xs opacity-60">
                        {{ $t('archives.expiresIn', { days: daysUntil(archive.expires_at) ?? 0 }) }}
                      </span>
                    </div>
                  </td>
                  <td>{{ archive.files_count }}</td>
                  <td class="whitespace-nowrap">{{ archive.size ? formatBytes(archive.size) : '—' }}</td>
                  <td class="whitespace-nowrap opacity-70">{{ formatDate(archive.created_at) }}</td>
                  <td>
                    <div class="flex justify-end gap-1">
                      <n-button
                        v-if="archive.status === 'done'"
                        size="tiny"
                        quaternary
                        :aria-label="$t('common.download')"
                        @click="download(archive)"
                      >
                        <template #icon><n-icon><DownloadOutline /></n-icon></template>
                      </n-button>
                      <n-button
                        size="tiny"
                        quaternary
                        type="error"
                        :aria-label="$t('common.delete')"
                        @click="removeArchive(archive)"
                      >
                        <template #icon><n-icon><TrashOutline /></n-icon></template>
                      </n-button>
                    </div>
                  </td>
                </tr>
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

    <ResponsiveDialog v-model:show="createOpen" :title="$t('archives.create')" :width="560">
      <n-alert v-if="createError" type="error" class="mb-4">{{ createError.message }}</n-alert>

      <n-form label-placement="top">
        <n-form-item :label="$t('archives.client')">
          <n-select
            :value="formClientId"
            filterable
            :options="clientOptions"
            :placeholder="$t('archives.pickClient')"
            @update:value="onFormClientChange"
          />
        </n-form-item>
      </n-form>

      <n-checkbox v-model:checked="wholeRoot" class="mb-3">{{ $t('archives.wholeRoot') }}</n-checkbox>

      <n-spin :show="treeLoading">
        <div class="max-h-72 overflow-y-auto rounded-lg border border-gray-200 p-2 dark:border-dark-100">
          <n-empty v-if="!treeData.length" size="small" :description="$t('archives.noFolders')" class="py-6" />
          <n-tree
            v-else
            v-model:checked-keys="checkedFolders"
            block-line
            checkable
            :cascade="false"
            allow-checking-not-loaded
            :disabled="wholeRoot"
            :data="treeData"
            :on-load="onTreeLoad"
          />
        </div>
      </n-spin>

      <template #footer>
        <div class="flex justify-end gap-2">
          <n-button @click="createOpen = false">{{ $t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="creating" @click="submitCreate">{{ $t('archives.create') }}</n-button>
        </div>
      </template>
    </ResponsiveDialog>
  </div>
</template>
