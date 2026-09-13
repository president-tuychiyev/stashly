<script setup lang="ts">
import { CloudUploadOutline, DownloadOutline, FolderOutline, TrashOutline } from '@vicons/ionicons5'
import type { UploadFileInfo } from 'naive-ui'
import type { ApiError, FileItem, Folder, Paginated, Visibility } from '~/types/api'

const api = useApi()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const { clients, options: clientOptions, load: loadClients } = useClientOptions()

/** Sentinel for the "all clients" option; `client_id` is then omitted. */
const ALL_CLIENTS = 0

const clientId = ref<number>(queryNumber(route.query.client_id) ?? ALL_CLIENTS)
const folder = ref<string>('')
const search = ref('')
const visibility = ref<Visibility | null>(null)
const mime = ref<string | null>(null)
const page = ref(1)
const perPage = ref(20)

const folders = ref<Folder[]>([])
const files = ref<FileItem[]>([])
const total = ref(0)
const selected = ref<number[]>([])
const loading = ref(true)
const error = ref<ApiError | null>(null)

const selectedClient = computed(() => clients.value.find((item) => item.id === clientId.value) ?? null)

const clientFilterOptions = computed(() => [
  { label: t('files.allClients'), value: ALL_CLIENTS },
  ...clientOptions.value,
])

const visibilityOptions = computed(() => [
  { label: t('files.visibilities.public'), value: 'public' },
  { label: t('files.visibilities.private'), value: 'private' },
])

const mimeOptions = [
  'image/*',
  'video/*',
  'audio/*',
  'application/pdf',
  'application/zip',
  'text/plain',
].map((value) => ({ label: value, value }))

const breadcrumbs = computed(() => {
  const parts = folder.value ? folder.value.split('/').filter(Boolean) : []
  const trail: Array<{ label: string; path: string }> = [{ label: t('files.root'), path: '' }]
  let current = ''
  for (const part of parts) {
    current = current ? `${current}/${part}` : part
    trail.push({ label: part, path: current })
  }
  return trail
})

// Debounced mirror of `search`; only this one takes part in loading.
const searchTerm = ref('')
let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    searchTerm.value = search.value
  }, 400)
})
onScopeDispose(() => clearTimeout(searchTimer))

const loadFolders = async (): Promise<Folder[]> => {
  // The folders endpoint requires a client.
  if (!clientId.value) return []
  const response = await api.get<{ data: Folder[] }>('/admin/folders', {
    client_id: clientId.value,
    parent: folder.value || undefined,
  })
  return response.data ?? []
}

const loadFiles = async (): Promise<Paginated<FileItem>> => {
  return api.get<Paginated<FileItem>>('/admin/files', {
    client_id: clientId.value || undefined,
    folder: clientId.value ? folder.value : undefined,
    search: searchTerm.value || undefined,
    visibility: visibility.value || undefined,
    mime: mime.value || undefined,
    page: page.value,
    per_page: perPage.value,
  })
}

// Every reload goes through this loader; stale responses are dropped.
let requestId = 0

const load = async () => {
  const current = ++requestId
  loading.value = true
  error.value = null
  selected.value = []
  try {
    const [folderList, filesResponse] = await Promise.all([loadFolders(), loadFiles()])
    if (current !== requestId) return
    folders.value = folderList
    files.value = filesResponse.data ?? []
    total.value = filesResponse.meta?.total ?? 0
  } catch (e) {
    if (current !== requestId) return
    error.value = e as ApiError
    folders.value = []
    files.value = []
    total.value = 0
  } finally {
    if (current === requestId) loading.value = false
  }
}

onMounted(async () => {
  await loadClients()
  await load()
})

// Filter changes jump back to page 1; the single watcher below still fires once.
watch(clientId, () => {
  folder.value = ''
})
watch([clientId, searchTerm, visibility, mime, perPage], () => {
  page.value = 1
})
watch([clientId, folder, searchTerm, visibility, mime, page, perPage], load)

// Keep the client filter in the URL so back/forward restores it.
watch(clientId, (value) => {
  if ((queryNumber(route.query.client_id) ?? ALL_CLIENTS) === value) return
  const query = { ...route.query }
  if (!value) delete query.client_id
  else query.client_id = String(value)
  router.replace({ query })
})
watch(
  () => route.query.client_id,
  (value) => {
    const next = queryNumber(value) ?? ALL_CLIENTS
    if (next !== clientId.value) clientId.value = next
  },
)

const openFolder = (path: string) => {
  folder.value = path
  page.value = 1
}

/* ---------------------------------------------------------------- selection */

const allSelected = computed(() => files.value.length > 0 && selected.value.length === files.value.length)

const toggleAll = (checked: boolean) => {
  selected.value = checked ? files.value.map((file) => file.id) : []
}

const toggleOne = (id: number, checked: boolean) => {
  selected.value = checked ? [...selected.value, id] : selected.value.filter((item) => item !== id)
}

/* ---------------------------------------------------------------- actions */

const download = async (file: FileItem) => {
  try {
    await api.download(`/admin/files/${file.id}/download`, file.original_name)
  } catch (e) {
    message.error((e as ApiError).message)
  }
}

const removeFile = (file: FileItem) => {
  dialog.error({
    title: t('files.delete'),
    content: t('files.deleteConfirm', { name: file.original_name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await api.del(`/admin/files/${file.id}`)
        message.success(t('files.deleted'))
        await load()
      } catch (e) {
        message.error((e as ApiError).message)
      }
    },
  })
}

const bulkDelete = () => {
  if (!selected.value.length) return
  dialog.error({
    title: t('files.bulkDelete'),
    content: t('files.bulkDeleteConfirm', { count: selected.value.length }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        const result = await api.post<{ deleted: number }>('/admin/files/bulk-delete', { ids: selected.value })
        message.success(t('files.bulkDeleted', { count: result.deleted }))
        await load()
      } catch (e) {
        message.error((e as ApiError).message)
      }
    },
  })
}

/* ---------------------------------------------------------------- upload */

const uploadOpen = ref(false)
const uploadFiles = ref<UploadFileInfo[]>([])
const uploadVisibility = ref<Visibility>('public')
const uploadProgress = ref(0)
const uploading = ref(false)
const uploadError = ref<ApiError | null>(null)

const openUpload = () => {
  uploadFiles.value = []
  uploadVisibility.value = 'public'
  uploadProgress.value = 0
  uploadError.value = null
  uploadOpen.value = true
}

/** True when `mime` matches one of the client patterns (`type/*` supported). */
const mimeAllowed = (value: string, patterns: string[]) => {
  if (!patterns.length) return true
  const lowerValue = value.toLowerCase()
  return patterns.some((pattern) => {
    const lowerPattern = pattern.toLowerCase()
    if (lowerPattern === '*' || lowerPattern === '*/*') return true
    if (lowerPattern.endsWith('/*')) return lowerValue.startsWith(lowerPattern.slice(0, -1))
    return lowerPattern === lowerValue
  })
}

/** Per-file problems found before the request is sent. */
const uploadIssues = computed(() => {
  const client = selectedClient.value
  if (!client) return [] as string[]
  const issues: string[] = []
  let totalSize = 0
  for (const item of uploadFiles.value) {
    const file = item.file
    if (!file) continue
    totalSize += file.size
    if (client.max_file_size && file.size > client.max_file_size) {
      issues.push(
        t('files.errors.tooLarge', {
          name: item.name,
          size: formatBytes(file.size),
          limit: formatBytes(client.max_file_size),
        }),
      )
    }
    const type = (file.type || '').toLowerCase()
    if (type && !mimeAllowed(type, client.allowed_mimes ?? [])) {
      issues.push(t('files.errors.mimeNotAllowed', { name: item.name, mime: type }))
    }
  }
  if (client.quota_bytes !== null && client.quota_bytes !== undefined) {
    const remaining = client.quota_bytes - client.used_bytes
    if (totalSize > remaining) {
      issues.push(
        t('files.errors.quotaExceeded', {
          size: formatBytes(totalSize),
          remaining: formatBytes(Math.max(0, remaining)),
        }),
      )
    }
  }
  return issues
})

/** `errors["files.N"]` keys mapped to the actual selected file name, for display. */
const uploadErrorItems = computed(() => {
  if (!uploadError.value?.errors) return [] as Array<{ label: string; text: string }>
  return Object.entries(uploadError.value.errors).map(([field, msgs]) => {
    const match = /^files\.(\d+)$/.exec(field)
    const label = match ? (uploadFiles.value[Number(match[1])]?.name ?? field) : field
    return { label, text: joinErrorMessages(msgs) }
  })
})

const submitUpload = async () => {
  if (!clientId.value || !uploadFiles.value.length) return
  if (uploadIssues.value.length) {
    uploadError.value = { message: t('files.errors.fixFirst'), errors: { files: uploadIssues.value } }
    return
  }
  uploading.value = true
  uploadError.value = null
  uploadProgress.value = 0

  const formData = new FormData()
  formData.append('client_id', String(clientId.value))
  formData.append('folder', folder.value)
  formData.append('visibility', uploadVisibility.value)
  for (const item of uploadFiles.value) {
    if (item.file) formData.append('files[]', item.file, item.name)
  }

  try {
    await api.upload<{ data: FileItem[] }>('/admin/files', formData, (percent) => {
      uploadProgress.value = percent
    })
    message.success(t('files.uploaded'))
    uploadOpen.value = false
    await load()
  } catch (e) {
    uploadError.value = e as ApiError
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <div>
    <PageHeader :title="$t('files.title')" :subtitle="$t('files.subtitle')">
      <template #actions>
        <n-button v-if="selected.length" type="error" secondary @click="bulkDelete">
          <template #icon><n-icon><TrashOutline /></n-icon></template>
          {{ $t('files.bulkDelete') }} ({{ selected.length }})
        </n-button>
        <n-button type="primary" :disabled="!clientId" @click="openUpload">
          <template #icon><n-icon><CloudUploadOutline /></n-icon></template>
          {{ $t('files.upload') }}
        </n-button>
      </template>
    </PageHeader>

    <n-card class="mb-4 rounded-xl">
      <div class="flex flex-col gap-3 lg:flex-row">
        <n-select
          v-model:value="clientId"
          filterable
          :options="clientFilterOptions"
          :placeholder="$t('files.selectClient')"
          class="lg:max-w-sm"
        />
        <n-input
          v-model:value="search"
          clearable
          :placeholder="$t('common.search')"
          class="lg:max-w-xs"
        />
        <n-select
          v-model:value="visibility"
          clearable
          :options="visibilityOptions"
          :placeholder="$t('files.visibility')"
          class="lg:max-w-[12rem]"
        />
        <n-select
          v-model:value="mime"
          clearable
          filterable
          tag
          :options="mimeOptions"
          :placeholder="$t('files.mimeFilter')"
          class="lg:max-w-[14rem]"
        />
      </div>
    </n-card>

    <ApiErrorAlert :error="error" @retry="load" />

    <n-breadcrumb v-if="clientId" class="mb-3">
      <n-breadcrumb-item v-for="crumb in breadcrumbs" :key="crumb.path">
        <button type="button" class="cursor-pointer border-0 bg-transparent p-0 font-[inherit] text-[inherit]" @click="openFolder(crumb.path)">
          {{ crumb.label }}
        </button>
      </n-breadcrumb-item>
    </n-breadcrumb>

    <n-spin :show="loading">
      <div v-if="folders.length" class="mb-4 grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-4">
        <button
          v-for="item in folders"
          :key="item.path"
          type="button"
          class="flex items-center gap-2 rounded-lg border border-gray-200 p-3 text-left transition-colors hover:border-teal-500 dark:border-dark-100"
          @click="openFolder(item.path)"
        >
          <n-icon size="20" class="text-teal-500"><FolderOutline /></n-icon>
          <span class="min-w-0 flex-1">
            <span class="block truncate text-sm font-medium">{{ item.name }}</span>
            <span class="block text-xs opacity-60">
              {{ item.files_count }} {{ $t('dashboard.filesShort') }} · {{ formatBytes(item.size) }}
            </span>
          </span>
        </button>
      </div>

      <n-empty v-if="!files.length && !loading" :description="$t('files.empty')" class="py-12" />

      <!-- Mobile: card list -->
      <div v-else class="space-y-3 md:hidden">
        <n-card v-for="file in files" :key="file.id" size="small" class="rounded-xl">
          <div class="flex items-start gap-3">
            <n-checkbox
              :checked="selected.includes(file.id)"
              @update:checked="(checked: boolean) => toggleOne(file.id, checked)"
            />
            <img
              v-if="isPreviewable(file.mime, file.url)"
              :src="file.url!"
              :alt="file.original_name"
              class="h-12 w-12 shrink-0 rounded object-cover"
            >
            <MimeIcon v-else :mime="file.mime" :size="24" />
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">{{ file.original_name }}</p>
              <p class="truncate text-xs opacity-60">{{ file.mime }} · {{ formatBytes(file.size) }}</p>
              <p class="text-xs opacity-60">{{ formatDate(file.created_at) }}</p>
              <n-tag size="tiny" :type="file.visibility === 'public' ? 'success' : 'default'" class="mt-1">
                {{ $t(`files.visibilities.${file.visibility}`) }}
              </n-tag>
            </div>
          </div>
          <div class="mt-2 flex gap-2">
            <n-button size="tiny" secondary @click="download(file)">{{ $t('common.download') }}</n-button>
            <n-button size="tiny" secondary type="error" @click="removeFile(file)">
              {{ $t('common.delete') }}
            </n-button>
          </div>
        </n-card>
      </div>

      <!-- Desktop: table -->
      <div v-if="files.length || loading" class="hidden md:block">
        <n-card class="rounded-xl" content-style="padding: 0;">
          <div class="overflow-x-auto">
            <n-table :single-line="false" size="small">
              <thead>
                <tr>
                  <th class="w-10">
                    <n-checkbox :checked="allSelected" @update:checked="toggleAll" />
                  </th>
                  <th class="w-14" />
                  <th>{{ $t('files.originalName') }}</th>
                  <th>{{ $t('files.storedName') }}</th>
                  <th>{{ $t('files.size') }}</th>
                  <th>{{ $t('files.visibility') }}</th>
                  <th>{{ $t('common.createdAt') }}</th>
                  <th class="text-right">{{ $t('common.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="file in files" :key="file.id">
                  <td>
                    <n-checkbox
                      :checked="selected.includes(file.id)"
                      @update:checked="(checked: boolean) => toggleOne(file.id, checked)"
                    />
                  </td>
                  <td>
                    <img
                      v-if="isPreviewable(file.mime, file.url)"
                      :src="file.url!"
                      :alt="file.original_name"
                      class="h-9 w-9 rounded object-cover"
                    >
                    <MimeIcon v-else :mime="file.mime" />
                  </td>
                  <td class="max-w-[18rem] truncate font-medium">{{ file.original_name }}</td>
                  <td class="max-w-[14rem] truncate font-mono text-xs opacity-60">{{ file.name }}</td>
                  <td class="whitespace-nowrap">{{ formatBytes(file.size) }}</td>
                  <td>
                    <n-tag size="small" :type="file.visibility === 'public' ? 'success' : 'default'">
                      {{ $t(`files.visibilities.${file.visibility}`) }}
                    </n-tag>
                  </td>
                  <td class="whitespace-nowrap opacity-70">{{ formatDate(file.created_at) }}</td>
                  <td>
                    <div class="flex justify-end gap-1">
                      <n-button size="tiny" quaternary :aria-label="$t('common.download')" @click="download(file)">
                        <template #icon><n-icon><DownloadOutline /></n-icon></template>
                      </n-button>
                      <n-button
                        size="tiny"
                        quaternary
                        type="error"
                        :aria-label="$t('common.delete')"
                        @click="removeFile(file)"
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

    <ResponsiveDialog v-model:show="uploadOpen" :title="$t('files.upload')" :width="560">
      <n-alert v-if="uploadError" type="error" class="mb-4">
        <p>{{ uploadError.message }}</p>
        <ul v-if="uploadErrorItems.length" class="mt-1 list-inside list-disc text-sm">
          <li v-for="item in uploadErrorItems" :key="item.label">{{ item.label }}: {{ item.text }}</li>
        </ul>
      </n-alert>

      <n-form label-placement="top">
        <n-form-item :label="$t('files.targetFolder')">
          <n-input :value="folder || '/'" readonly />
        </n-form-item>
        <n-form-item :label="$t('files.visibility')">
          <n-select
            v-model:value="uploadVisibility"
            :options="[
              { label: $t('files.visibilities.public'), value: 'public' },
              { label: $t('files.visibilities.private'), value: 'private' },
            ]"
          />
        </n-form-item>
      </n-form>

      <n-alert v-if="uploadIssues.length" type="warning" class="mb-4">
        <ul class="list-inside list-disc text-sm">
          <li v-for="issue in uploadIssues" :key="issue">{{ issue }}</li>
        </ul>
      </n-alert>

      <n-upload v-model:file-list="uploadFiles" multiple :default-upload="false" :max="20">
        <n-upload-dragger>
          <div class="py-4 text-center">
            <n-icon size="32" class="opacity-50"><CloudUploadOutline /></n-icon>
            <p class="mt-2 text-sm">{{ $t('files.dropHint') }}</p>
          </div>
        </n-upload-dragger>
      </n-upload>

      <n-progress v-if="uploading" class="mt-4" type="line" :percentage="uploadProgress" />

      <template #footer>
        <div class="flex justify-end gap-2">
          <n-button :disabled="uploading" @click="uploadOpen = false">{{ $t('common.cancel') }}</n-button>
          <n-button
            type="primary"
            :loading="uploading"
            :disabled="!uploadFiles.length || uploadIssues.length > 0"
            @click="submitUpload"
          >
            {{ $t('files.upload') }}
          </n-button>
        </div>
      </template>
    </ResponsiveDialog>
  </div>
</template>
