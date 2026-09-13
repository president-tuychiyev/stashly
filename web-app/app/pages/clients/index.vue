<script setup lang="ts">
import { AddOutline, KeyOutline, PencilOutline, PhonePortraitOutline, TrashOutline } from '@vicons/ionicons5'
import { NCheckbox } from 'naive-ui'
import type { ApiError, Client, ClientStatus, Paginated } from '~/types/api'

const api = useApi()
const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const { isSuperAdmin } = useAuth()

// Only super admins may pick an owner; for `admin` the API forces it to self.
const owners = useUserOptions()
watch(isSuperAdmin, (value) => {
  if (value) void owners.load()
}, { immediate: true })

const items = ref<Client[]>([])
const total = ref(0)
const page = ref(1)
const perPage = ref(20)
const search = ref('')
const status = ref<ClientStatus | null>(null)
const loading = ref(true)
const error = ref<ApiError | null>(null)

const statusOptions = computed(() =>
  (['active', 'inactive', 'blocked'] as ClientStatus[]).map((value) => ({
    label: t(`clients.statuses.${value}`),
    value,
  })),
)

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

// Every reload goes through this loader; stale responses are dropped.
let requestId = 0

const load = async () => {
  const current = ++requestId
  loading.value = true
  error.value = null
  try {
    const response = await api.get<Paginated<Client>>('/admin/clients', {
      page: page.value,
      per_page: perPage.value,
      search: searchTerm.value || undefined,
      status: status.value || undefined,
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
watch([searchTerm, status, perPage], () => {
  page.value = 1
})
watch([page, perPage, searchTerm, status], load)

/* ---------------------------------------------------------------- create / edit */

type QuotaUnit = (typeof QUOTA_UNITS)[number]
type FileSizeUnit = (typeof FILE_SIZE_UNITS)[number]

const quotaUnitOptions = QUOTA_UNITS.map((value) => ({ label: value, value }))
const fileSizeUnitOptions = FILE_SIZE_UNITS.map((value) => ({ label: value, value }))

const formOpen = ref(false)
const saving = ref(false)
const editing = ref<Client | null>(null)
const form = reactive({
  name: '',
  username: '',
  password: '',
  status: 'active' as ClientStatus,
  quotaAmount: null as number | null,
  quotaUnit: 'GB' as QuotaUnit,
  allowedMimes: [] as string[],
  maxFileAmount: null as number | null,
  maxFileUnit: 'MB' as FileSizeUnit,
  ownerId: null as number | null,
})
const formError = ref<ApiError | null>(null)

const resetForm = () => {
  form.name = ''
  form.username = ''
  form.password = ''
  form.status = 'active'
  form.quotaAmount = null
  form.quotaUnit = 'GB'
  form.allowedMimes = []
  form.maxFileAmount = null
  form.maxFileUnit = 'MB'
  form.ownerId = null
  formError.value = null
}

const openCreate = () => {
  editing.value = null
  resetForm()
  formOpen.value = true
}

const openEdit = (client: Client) => {
  editing.value = client
  resetForm()
  form.name = client.name
  form.username = client.username
  form.status = client.status
  const quota = fromBytes(client.quota_bytes, QUOTA_UNITS, 'GB')
  form.quotaAmount = quota.amount
  form.quotaUnit = quota.unit
  const maxFile = fromBytes(client.max_file_size, FILE_SIZE_UNITS, 'MB')
  form.maxFileAmount = maxFile.amount
  form.maxFileUnit = maxFile.unit
  form.allowedMimes = [...(client.allowed_mimes ?? [])]
  form.ownerId = client.owner_id ?? client.owner?.id ?? null
  formOpen.value = true
}

const revealPassword = ref<string | null>(null)
const revealUsername = ref<string | null>(null)
const revealOpen = ref(false)

const submitForm = async () => {
  if (!form.name.trim() || !form.username.trim()) {
    formError.value = { message: t('clients.nameUsernameRequired') }
    return
  }
  saving.value = true
  formError.value = null

  const payload: Record<string, unknown> = {
    name: form.name.trim(),
    username: form.username.trim(),
    status: form.status,
    quota_bytes: !form.quotaAmount ? null : toBytes(form.quotaAmount, form.quotaUnit),
    allowed_mimes: form.allowedMimes,
    max_file_size: !form.maxFileAmount ? null : toBytes(form.maxFileAmount, form.maxFileUnit),
  }
  if (form.password) payload.password = form.password
  // `admin` callers must not send it at all; the API would ignore it anyway.
  if (isSuperAdmin.value) {
    if (editing.value) {
      // Always send it on update so clearing the select can unset the owner.
      payload.owner_id = form.ownerId ?? null
    } else if (form.ownerId) {
      // Omit on create when unset: the server defaults it to the caller.
      payload.owner_id = form.ownerId
    }
  }

  try {
    if (editing.value) {
      await api.put<{ data: Client }>(`/admin/clients/${editing.value.id}`, payload)
      message.success(t('clients.updated'))
    } else {
      const response = await api.post<{ data: Client; password: string }>('/admin/clients', payload)
      message.success(t('clients.created'))
      if (response.password) {
        revealPassword.value = response.password
        revealUsername.value = response.data?.username ?? form.username
        revealOpen.value = true
      }
    }
    formOpen.value = false
    await load()
  } catch (e) {
    formError.value = e as ApiError
  } finally {
    saving.value = false
  }
}

/* ---------------------------------------------------------------- row actions */

const resetPassword = (client: Client) => {
  dialog.warning({
    title: t('clients.resetPassword'),
    content: t('clients.resetPasswordConfirm', { name: client.name }),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        const response = await api.post<{ data: Client; password: string }>(
          `/admin/clients/${client.id}/reset-password`,
        )
        revealPassword.value = response.password
        revealUsername.value = client.username
        revealOpen.value = true
        message.success(t('clients.passwordReset'))
      } catch (e) {
        message.error((e as ApiError).message)
      }
    },
  })
}

const purge = ref(false)

const removeClient = (client: Client) => {
  purge.value = false
  dialog.error({
    title: t('clients.delete'),
    content: () =>
      h('div', { class: 'space-y-3' }, [
        h('p', null, t('clients.deleteConfirm', { name: client.name })),
        h(
          NCheckbox,
          {
            checked: purge.value,
            'onUpdate:checked': (value: boolean) => {
              purge.value = value
            },
          },
          { default: () => t('clients.purgeFiles') },
        ),
      ]),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await api.del(`/admin/clients/${client.id}`, purge.value ? { purge: true } : undefined)
        message.success(t('clients.deleted'))
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
    <PageHeader :title="$t('clients.title')" :subtitle="$t('clients.subtitle')">
      <template #actions>
        <n-button type="primary" @click="openCreate">
          <template #icon>
            <n-icon><AddOutline /></n-icon>
          </template>
          {{ $t('clients.create') }}
        </n-button>
      </template>
    </PageHeader>

    <n-card class="mb-4 rounded-xl">
      <div class="flex flex-col gap-3 sm:flex-row">
        <n-input v-model:value="search" clearable :placeholder="$t('common.search')" class="sm:max-w-xs" />
        <n-select
          v-model:value="status"
          clearable
          :options="statusOptions"
          :placeholder="$t('clients.status')"
          class="sm:max-w-[12rem]"
        />
      </div>
    </n-card>

    <ApiErrorAlert :error="error" @retry="load" />

    <n-spin :show="loading">
      <n-empty v-if="!items.length && !loading" :description="$t('common.noData')" class="py-12" />

      <!-- Mobile: card list -->
      <div v-else class="space-y-3 md:hidden">
        <n-card v-for="client in items" :key="client.id" class="rounded-xl" size="small">
          <div class="mb-2 flex items-start justify-between gap-2">
            <div class="min-w-0">
              <NuxtLink :to="`/clients/${client.id}`" class="block truncate font-medium hover:text-teal-500">
                {{ client.name }}
              </NuxtLink>
              <p class="truncate text-xs opacity-60">@{{ client.username }}</p>
            </div>
            <ClientStatusTag :status="client.status" />
          </div>

          <QuotaBar :used="client.used_bytes" :quota="client.quota_bytes" />

          <div class="mt-2 flex justify-between text-xs opacity-60">
            <span>{{ client.files_count }} {{ $t('dashboard.filesShort') }}</span>
            <span>{{ formatDate(client.created_at, false) }}</span>
          </div>

          <p v-if="isSuperAdmin" class="mt-1 truncate text-xs opacity-60">
            {{ $t('clients.owner') }}: {{ client.owner?.name || '—' }}
          </p>

          <div class="mt-3 flex flex-wrap gap-2">
            <n-button size="tiny" secondary @click="openEdit(client)">{{ $t('common.edit') }}</n-button>
            <n-button size="tiny" secondary @click="resetPassword(client)">{{ $t('clients.resetPassword') }}</n-button>
            <n-button size="tiny" secondary @click="navigateTo(`/clients/${client.id}`)">
              {{ $t('clients.devices') }}
            </n-button>
            <n-button size="tiny" secondary type="error" @click="removeClient(client)">
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
                  <th>{{ $t('clients.name') }}</th>
                  <th>{{ $t('clients.username') }}</th>
                  <th>{{ $t('clients.status') }}</th>
                  <th class="min-w-[180px]">{{ $t('clients.usage') }}</th>
                  <th>{{ $t('clients.filesCount') }}</th>
                  <th v-if="isSuperAdmin">{{ $t('clients.owner') }}</th>
                  <th>{{ $t('common.createdAt') }}</th>
                  <th class="text-right">{{ $t('common.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="client in items" :key="client.id">
                  <td>
                    <NuxtLink :to="`/clients/${client.id}`" class="font-medium hover:text-teal-500">
                      {{ client.name }}
                    </NuxtLink>
                  </td>
                  <td class="opacity-70">@{{ client.username }}</td>
                  <td><ClientStatusTag :status="client.status" /></td>
                  <td><QuotaBar :used="client.used_bytes" :quota="client.quota_bytes" /></td>
                  <td>{{ client.files_count }}</td>
                  <td v-if="isSuperAdmin" class="opacity-70">{{ client.owner?.name || '—' }}</td>
                  <td class="whitespace-nowrap opacity-70">{{ formatDate(client.created_at, false) }}</td>
                  <td>
                    <div class="flex justify-end gap-1">
                      <n-tooltip trigger="hover">
                        <template #trigger>
                          <n-button size="tiny" quaternary :aria-label="$t('common.edit')" @click="openEdit(client)">
                            <template #icon><n-icon><PencilOutline /></n-icon></template>
                          </n-button>
                        </template>
                        {{ $t('common.edit') }}
                      </n-tooltip>
                      <n-tooltip trigger="hover">
                        <template #trigger>
                          <n-button
                            size="tiny"
                            quaternary
                            :aria-label="$t('clients.resetPassword')"
                            @click="resetPassword(client)"
                          >
                            <template #icon><n-icon><KeyOutline /></n-icon></template>
                          </n-button>
                        </template>
                        {{ $t('clients.resetPassword') }}
                      </n-tooltip>
                      <n-tooltip trigger="hover">
                        <template #trigger>
                          <n-button
                            size="tiny"
                            quaternary
                            :aria-label="$t('clients.devices')"
                            @click="navigateTo(`/clients/${client.id}`)"
                          >
                            <template #icon><n-icon><PhonePortraitOutline /></n-icon></template>
                          </n-button>
                        </template>
                        {{ $t('clients.devices') }}
                      </n-tooltip>
                      <n-tooltip trigger="hover">
                        <template #trigger>
                          <n-button
                            size="tiny"
                            quaternary
                            type="error"
                            :aria-label="$t('common.delete')"
                            @click="removeClient(client)"
                          >
                            <template #icon><n-icon><TrashOutline /></n-icon></template>
                          </n-button>
                        </template>
                        {{ $t('common.delete') }}
                      </n-tooltip>
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

    <ResponsiveDialog
      v-model:show="formOpen"
      :title="editing ? $t('clients.edit') : $t('clients.create')"
      :width="600"
    >
      <n-alert v-if="formError" type="error" class="mb-4">
        <p>{{ formError.message }}</p>
        <ul v-if="formError.errors" class="mt-1 list-inside list-disc text-sm">
          <li v-for="(msgs, field) in formError.errors" :key="field">{{ field }}: {{ joinErrorMessages(msgs) }}</li>
        </ul>
      </n-alert>

      <n-form label-placement="top">
        <div class="grid grid-cols-1 gap-x-4 sm:grid-cols-2">
          <n-form-item :label="$t('clients.name')">
            <n-input v-model:value="form.name" />
          </n-form-item>
          <n-form-item :label="$t('clients.username')">
            <n-input v-model:value="form.username" />
          </n-form-item>
        </div>

        <n-form-item :label="$t('clients.password')">
          <n-input
            v-model:value="form.password"
            type="password"
            show-password-on="click"
            :placeholder="$t('clients.passwordAutoHint')"
          />
        </n-form-item>

        <n-form-item :label="$t('clients.status')">
          <n-select v-model:value="form.status" :options="statusOptions" />
        </n-form-item>

        <n-form-item v-if="isSuperAdmin" :label="$t('clients.owner')">
          <n-select
            v-model:value="form.ownerId"
            filterable
            clearable
            :loading="owners.loading.value"
            :options="owners.options.value"
            :placeholder="$t('clients.ownerPlaceholder')"
          />
        </n-form-item>

        <n-form-item :label="$t('clients.quota')">
          <n-input-group>
            <n-input-number
              v-model:value="form.quotaAmount"
              class="w-full"
              :min="1"
              clearable
              :placeholder="$t('clients.unlimited')"
            />
            <n-select
              v-model:value="form.quotaUnit"
              class="w-28"
              :options="quotaUnitOptions"
            />
          </n-input-group>
        </n-form-item>

        <n-form-item :label="$t('clients.maxFileSize')">
          <n-input-group>
            <n-input-number
              v-model:value="form.maxFileAmount"
              class="w-full"
              :min="1"
              clearable
              :placeholder="$t('clients.defaultFromEnv')"
            />
            <n-select
              v-model:value="form.maxFileUnit"
              class="w-28"
              :options="fileSizeUnitOptions"
            />
          </n-input-group>
        </n-form-item>

        <n-form-item :label="$t('clients.allowedMimes')">
          <n-dynamic-tags v-model:value="form.allowedMimes" />
        </n-form-item>
        <p class="-mt-2 text-xs opacity-60">{{ $t('clients.allowedMimesHint') }}</p>
      </n-form>

      <template #footer>
        <div class="flex justify-end gap-2">
          <n-button @click="formOpen = false">{{ $t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="saving" @click="submitForm">{{ $t('common.save') }}</n-button>
        </div>
      </template>
    </ResponsiveDialog>

    <PasswordRevealModal v-model:show="revealOpen" :password="revealPassword" :username="revealUsername" />
  </div>
</template>
