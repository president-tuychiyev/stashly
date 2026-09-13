<script setup lang="ts">
import { AddOutline, MailOutline, PencilOutline, TrashOutline } from '@vicons/ionicons5'
import type { ApiError, Paginated, RoleSlug, User, UserStatus } from '~/types/api'

const api = useApi()
const { t } = useI18n()
const message = useMessage()
const { user: me, ensureUser, isSuperAdmin } = useAuth()

const items = ref<User[]>([])
const total = ref(0)
const page = ref(1)
const perPage = ref(20)
const search = ref('')
const role = ref<RoleSlug | null>(null)
const status = ref<UserStatus | null>(null)
const loading = ref(true)
const error = ref<ApiError | null>(null)

/** Unknown while `ensureUser()` is still resolving; the list must not load until then. */
type Access = 'unknown' | 'allowed' | 'denied'
const access = ref<Access>('unknown')

const roleOptions = computed(() =>
  (['super_admin', 'admin'] as RoleSlug[]).map((value) => ({ label: t(`users.roles.${value}`), value })),
)
const statusOptions = computed(() =>
  (['pending', 'active', 'blocked'] as UserStatus[]).map((value) => ({
    label: t(`users.statuses.${value}`),
    value,
  })),
)

const roleLabel = (item: User) => {
  const slug = item.role?.slug
  if (!slug) return '—'
  const key = `users.roles.${slug}`
  const translated = t(key)
  return translated === key ? (item.role?.name ?? slug) : translated
}

const isSelf = (item: User) => item.id === me.value?.id

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
  if (access.value === 'denied') return
  const current = ++requestId
  loading.value = true
  error.value = null
  try {
    const response = await api.get<Paginated<User>>('/admin/users', {
      page: page.value,
      per_page: perPage.value,
      search: searchTerm.value || undefined,
      role: role.value || undefined,
      status: status.value || undefined,
    })
    if (current !== requestId) return
    items.value = response.data ?? []
    total.value = response.meta?.total ?? 0
  } catch (e) {
    if (current !== requestId) return
    const apiError = e as ApiError
    if (apiError.status === 403) access.value = 'denied'
    error.value = apiError
    items.value = []
    total.value = 0
  } finally {
    if (current === requestId) loading.value = false
  }
}

// Client-side guard: the API also 403s, but the page should never render (or
// fire the list request) for `admin` while access is still unknown.
onMounted(async () => {
  try {
    await ensureUser()
  } catch {
    // Unreachable API: fall through and let the list request report the problem.
  }
  if (me.value && !isSuperAdmin.value) {
    access.value = 'denied'
    loading.value = false
    return
  }
  access.value = 'allowed'
  await load()
})

watch([searchTerm, role, status, perPage], () => {
  page.value = 1
})
watch([page, perPage, searchTerm, role, status], load)

/* ---------------------------------------------------------------- create / edit */

const formOpen = ref(false)
const saving = ref(false)
const editing = ref<User | null>(null)
const form = reactive({
  name: '',
  email: '',
  role: 'admin' as RoleSlug,
  status: 'active' as Exclude<UserStatus, 'pending'>,
})
const formError = ref<ApiError | null>(null)

const fieldError = (apiError: ApiError | null, field: string): string | null => {
  const value = apiError?.errors?.[field]
  const text = joinErrorMessages(value)
  return text || null
}

const resetForm = () => {
  form.name = ''
  form.email = ''
  form.role = 'admin'
  form.status = 'active'
  formError.value = null
}

const openCreate = () => {
  editing.value = null
  resetForm()
  formOpen.value = true
}

const openEdit = (item: User) => {
  editing.value = item
  resetForm()
  form.name = item.name
  form.email = item.email
  form.role = (item.role?.slug as RoleSlug) ?? 'admin'
  form.status = item.status === 'blocked' ? 'blocked' : 'active'
  formOpen.value = true
}

/** Role and status are locked on your own account; the API refuses it too. */
const selfLocked = computed(() => Boolean(editing.value && isSelf(editing.value)))
/** A pending invite has no active/blocked state to pick yet. */
const statusEditable = computed(() => Boolean(editing.value) && editing.value?.status !== 'pending')

const submitForm = async () => {
  formError.value = null

  const localErrors: Record<string, string[]> = {}
  if (!form.name.trim()) localErrors.name = [t('users.nameRequired')]
  if (!editing.value && !isValidEmail(form.email)) localErrors.email = [t('auth.emailInvalid')]
  if (Object.keys(localErrors).length) {
    formError.value = { message: t('common.errorTitle'), errors: localErrors }
    return
  }

  saving.value = true
  try {
    if (editing.value) {
      const payload: Record<string, unknown> = { name: form.name.trim() }
      if (!selfLocked.value) {
        payload.role = form.role
        if (statusEditable.value) payload.status = form.status
      }
      await api.put<{ data: User }>(`/admin/users/${editing.value.id}`, payload)
      message.success(t('users.updated'))
    } else {
      const email = form.email.trim()
      await api.post<{ data: User }>('/admin/users', {
        name: form.name.trim(),
        email,
        role: form.role,
      })
      message.success(t('users.invitationSent', { email }))
    }
    formOpen.value = false
    await load()
  } catch (e) {
    formError.value = e as ApiError
  } finally {
    saving.value = false
  }
}

/* ---------------------------------------------------------------- resend invite */

const resending = ref<number | null>(null)

const resendCode = async (item: User) => {
  if (resending.value !== null) return
  resending.value = item.id
  try {
    await api.post(`/admin/users/${item.id}/resend-otp`)
    message.success(t('users.codeResent', { email: item.email }))
  } catch (e) {
    message.error((e as ApiError).message)
  } finally {
    resending.value = null
  }
}

/* ---------------------------------------------------------------- delete */

const deleteOpen = ref(false)
const deleting = ref(false)
const deleteTarget = ref<User | null>(null)
const needsReassign = ref(false)
const reassignTo = ref<number | null>(null)
const deleteError = ref<ApiError | null>(null)

// Reassignment must be able to target users outside the current page.
const allUsers = useUserOptions()

/** Everyone except the account being deleted can inherit its clients. */
const reassignOptions = computed(() =>
  allUsers.users.value
    .filter((item) => item.id !== deleteTarget.value?.id && item.status !== 'pending')
    .map((item) => ({ label: `${item.name} (${item.email})`, value: item.id })),
)

const openDelete = (item: User) => {
  deleteTarget.value = item
  needsReassign.value = (item.clients_count ?? 0) > 0
  reassignTo.value = null
  deleteError.value = null
  deleteOpen.value = true
  if (needsReassign.value) void allUsers.load(true)
}

const confirmDelete = async () => {
  const target = deleteTarget.value
  if (!target) return
  deleteError.value = null
  if (needsReassign.value && !reassignTo.value) {
    deleteError.value = { message: t('users.reassignRequired') }
    return
  }

  deleting.value = true
  try {
    await api.del(`/admin/users/${target.id}`, reassignTo.value ? { reassign_to: reassignTo.value } : undefined)
    message.success(t('users.deleted'))
    deleteOpen.value = false
    await load()
  } catch (e) {
    const apiError = e as ApiError
    // The API asks for a target when the user still owns clients.
    const mentionsReassign =
      Boolean(apiError.errors?.reassign_to) || /reassign/i.test(apiError.message ?? '')
    if (apiError.status === 422 && mentionsReassign) {
      needsReassign.value = true
      void allUsers.load(true)
    }
    deleteError.value = apiError
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <div>
    <PageHeader :title="$t('users.title')" :subtitle="$t('users.subtitle')">
      <template #actions>
        <n-button v-if="access === 'allowed'" type="primary" @click="openCreate">
          <template #icon>
            <n-icon><AddOutline /></n-icon>
          </template>
          {{ $t('users.create') }}
        </n-button>
      </template>
    </PageHeader>

    <n-spin v-if="access === 'unknown'" :show="true">
      <div class="min-h-[12rem]" />
    </n-spin>

    <n-alert v-else-if="access === 'denied'" type="error" :title="$t('users.forbiddenTitle')">
      {{ $t('users.forbidden') }}
    </n-alert>

    <template v-else>
      <n-card class="mb-4 rounded-xl">
        <div class="flex flex-col gap-3 sm:flex-row">
          <n-input v-model:value="search" clearable :placeholder="$t('common.search')" class="sm:max-w-xs" />
          <n-select
            v-model:value="role"
            clearable
            :options="roleOptions"
            :placeholder="$t('users.role')"
            class="sm:max-w-[12rem]"
          />
          <n-select
            v-model:value="status"
            clearable
            :options="statusOptions"
            :placeholder="$t('users.status')"
            class="sm:max-w-[12rem]"
          />
        </div>
      </n-card>

      <ApiErrorAlert :error="error" @retry="load" />

      <n-spin :show="loading">
        <n-empty v-if="!items.length && !loading" :description="$t('common.noData')" class="py-12" />

        <!-- Mobile: card list -->
        <div v-else class="space-y-3 md:hidden">
          <n-card v-for="item in items" :key="item.id" class="rounded-xl" size="small">
            <div class="mb-2 flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="truncate font-medium">{{ item.name }}</p>
                <p class="truncate text-xs opacity-60">{{ item.email }}</p>
              </div>
              <UserStatusTag :status="item.status" />
            </div>

            <div class="flex flex-wrap items-center gap-2 text-xs opacity-70">
              <n-tag size="small" :bordered="false">{{ roleLabel(item) }}</n-tag>
              <span>{{ $t('users.clientsCount') }}: {{ item.clients_count ?? 0 }}</span>
              <span>{{ $t('users.lastLogin') }}: {{ formatDate(item.last_login_at) }}</span>
            </div>

            <div class="mt-3 flex flex-wrap gap-2">
              <n-button size="tiny" secondary @click="openEdit(item)">{{ $t('common.edit') }}</n-button>
              <n-button
                v-if="item.status === 'pending'"
                size="tiny"
                secondary
                :loading="resending === item.id"
                @click="resendCode(item)"
              >
                {{ $t('users.resendCode') }}
              </n-button>
              <n-button
                v-if="!isSelf(item)"
                size="tiny"
                secondary
                type="error"
                @click="openDelete(item)"
              >
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
                    <th>{{ $t('users.name') }}</th>
                    <th>{{ $t('auth.email') }}</th>
                    <th>{{ $t('users.role') }}</th>
                    <th>{{ $t('users.status') }}</th>
                    <th>{{ $t('users.clientsCount') }}</th>
                    <th>{{ $t('users.lastLogin') }}</th>
                    <th class="text-right">{{ $t('common.actions') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in items" :key="item.id">
                    <td class="font-medium">
                      {{ item.name }}
                      <span v-if="isSelf(item)" class="ml-1 text-xs opacity-60">({{ $t('users.you') }})</span>
                    </td>
                    <td class="opacity-70">{{ item.email }}</td>
                    <td><n-tag size="small" :bordered="false">{{ roleLabel(item) }}</n-tag></td>
                    <td><UserStatusTag :status="item.status" /></td>
                    <td>{{ item.clients_count ?? 0 }}</td>
                    <td class="whitespace-nowrap opacity-70">{{ formatDate(item.last_login_at) }}</td>
                    <td>
                      <div class="flex justify-end gap-1">
                        <n-tooltip trigger="hover">
                          <template #trigger>
                            <n-button size="tiny" quaternary :aria-label="$t('common.edit')" @click="openEdit(item)">
                              <template #icon><n-icon><PencilOutline /></n-icon></template>
                            </n-button>
                          </template>
                          {{ $t('common.edit') }}
                        </n-tooltip>
                        <n-tooltip v-if="item.status === 'pending'" trigger="hover">
                          <template #trigger>
                            <n-button
                              size="tiny"
                              quaternary
                              :loading="resending === item.id"
                              :aria-label="$t('users.resendCode')"
                              @click="resendCode(item)"
                            >
                              <template #icon><n-icon><MailOutline /></n-icon></template>
                            </n-button>
                          </template>
                          {{ $t('users.resendCode') }}
                        </n-tooltip>
                        <n-tooltip v-if="!isSelf(item)" trigger="hover">
                          <template #trigger>
                            <n-button
                              size="tiny"
                              quaternary
                              type="error"
                              :aria-label="$t('common.delete')"
                              @click="openDelete(item)"
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
    </template>

    <ResponsiveDialog
      v-model:show="formOpen"
      :title="editing ? $t('users.edit') : $t('users.create')"
      :width="520"
    >
      <n-alert v-if="formError && !formError.errors" type="error" class="mb-4">{{ formError.message }}</n-alert>

      <n-form label-placement="top">
        <n-form-item
          :label="$t('users.name')"
          :validation-status="fieldError(formError, 'name') ? 'error' : undefined"
          :feedback="fieldError(formError, 'name') ?? undefined"
        >
          <n-input v-model:value="form.name" />
        </n-form-item>

        <n-form-item
          :label="$t('auth.email')"
          :validation-status="fieldError(formError, 'email') ? 'error' : undefined"
          :feedback="fieldError(formError, 'email') ?? undefined"
        >
          <n-input v-model:value="form.email" :disabled="Boolean(editing)" inputmode="email" />
        </n-form-item>

        <n-form-item
          :label="$t('users.role')"
          :validation-status="fieldError(formError, 'role') ? 'error' : undefined"
          :feedback="fieldError(formError, 'role') ?? undefined"
        >
          <n-select v-model:value="form.role" :options="roleOptions" :disabled="selfLocked" />
        </n-form-item>

        <n-form-item
          v-if="statusEditable"
          :label="$t('users.status')"
          :validation-status="fieldError(formError, 'status') ? 'error' : undefined"
          :feedback="fieldError(formError, 'status') ?? undefined"
        >
          <n-select
            v-model:value="form.status"
            :options="statusOptions.filter((option) => option.value !== 'pending')"
            :disabled="selfLocked"
          />
        </n-form-item>

        <p v-if="selfLocked" class="-mt-2 text-xs opacity-60">{{ $t('users.selfLocked') }}</p>
        <p v-else-if="!editing" class="-mt-2 text-xs opacity-60">{{ $t('users.createHint') }}</p>
      </n-form>

      <template #footer>
        <div class="flex justify-end gap-2">
          <n-button @click="formOpen = false">{{ $t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="saving" @click="submitForm">{{ $t('common.save') }}</n-button>
        </div>
      </template>
    </ResponsiveDialog>

    <ResponsiveDialog v-model:show="deleteOpen" :title="$t('users.delete')" :width="480">
      <n-alert v-if="deleteError" type="error" class="mb-4">
        <p>{{ deleteError.message }}</p>
        <ul v-if="deleteError.errors" class="mt-1 list-inside list-disc text-sm">
          <li v-for="(msgs, field) in deleteError.errors" :key="field">{{ joinErrorMessages(msgs) }}</li>
        </ul>
      </n-alert>

      <p>{{ $t('users.deleteConfirm', { name: deleteTarget?.name ?? '' }) }}</p>

      <div v-if="needsReassign" class="mt-4">
        <p class="mb-2 text-sm opacity-70">
          {{ $t('users.reassignHint', { count: deleteTarget?.clients_count ?? 0 }) }}
        </p>
        <n-select
          v-model:value="reassignTo"
          filterable
          :loading="allUsers.loading.value"
          :options="reassignOptions"
          :placeholder="$t('users.reassignPlaceholder')"
        />
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <n-button @click="deleteOpen = false">{{ $t('common.cancel') }}</n-button>
          <n-button type="error" :loading="deleting" @click="confirmDelete">{{ $t('common.delete') }}</n-button>
        </div>
      </template>
    </ResponsiveDialog>
  </div>
</template>
