<script setup lang="ts">
import { ArchiveOutline, ArrowBackOutline, FolderOpenOutline, TrashOutline } from '@vicons/ionicons5'
import type { ApiError, Client, Device, Paginated } from '~/types/api'

const route = useRoute()
const api = useApi()
const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const clientId = computed(() => Number(route.params.id))

const client = ref<Client | null>(null)
const devices = ref<Device[]>([])
const devicesTotal = ref(0)
const devicePage = ref(1)
const perPage = 20
const loading = ref(true)
const error = ref<ApiError | null>(null)

const loadClient = async () => {
  const response = await api.get<{ data: Client }>(`/admin/clients/${clientId.value}`)
  client.value = response.data
}

const loadDevices = async () => {
  const response = await api.get<Paginated<Device>>(`/admin/clients/${clientId.value}/devices`, {
    page: devicePage.value,
    per_page: perPage,
  })
  devices.value = response.data ?? []
  devicesTotal.value = response.meta?.total ?? 0
}

const load = async () => {
  loading.value = true
  error.value = null
  try {
    await Promise.all([loadClient(), loadDevices()])
  } catch (e) {
    error.value = e as ApiError
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => route.params.id, load)
watch(devicePage, () => {
  loadDevices().catch((e) => {
    error.value = e as ApiError
  })
})

const removeDevice = (device: Device) => {
  dialog.error({
    title: t('clients.deleteDevice'),
    content: t('clients.deleteDeviceConfirm', { uid: device.uid }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await api.del(`/admin/clients/${clientId.value}/devices/${device.id}`)
        message.success(t('clients.deviceDeleted'))
        // Deleting the last row of a page beyond the first steps back a page;
        // the `devicePage` watcher then reloads. Otherwise reload in place.
        if (devices.value.length === 1 && devicePage.value > 1) {
          devicePage.value -= 1
        } else {
          await loadDevices()
        }
      } catch (e) {
        message.error((e as ApiError).message)
      }
    },
  })
}
</script>

<template>
  <div>
    <PageHeader :title="client?.name || $t('clients.detail')" :subtitle="client ? `@${client.username}` : ''">
      <template #actions>
        <n-button secondary @click="navigateTo('/clients')">
          <template #icon><n-icon><ArrowBackOutline /></n-icon></template>
          {{ $t('common.back') }}
        </n-button>
        <n-button secondary @click="navigateTo(`/files?client_id=${clientId}`)">
          <template #icon><n-icon><FolderOpenOutline /></n-icon></template>
          {{ $t('menu.files') }}
        </n-button>
        <n-button secondary @click="navigateTo(`/archives?client_id=${clientId}`)">
          <template #icon><n-icon><ArchiveOutline /></n-icon></template>
          {{ $t('menu.archives') }}
        </n-button>
      </template>
    </PageHeader>

    <ApiErrorAlert :error="error" @retry="load" />

    <n-spin :show="loading">
      <n-card v-if="client" :title="$t('clients.info')" class="mb-4 rounded-xl">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
          <div>
            <p class="text-xs uppercase opacity-60">{{ $t('clients.status') }}</p>
            <ClientStatusTag :status="client.status" class="mt-1" />
          </div>
          <div>
            <p class="text-xs uppercase opacity-60">{{ $t('clients.usage') }}</p>
            <QuotaBar class="mt-1" :used="client.used_bytes" :quota="client.quota_bytes" />
          </div>
          <div>
            <p class="text-xs uppercase opacity-60">{{ $t('clients.filesCount') }}</p>
            <p class="mt-1 font-medium">{{ client.files_count }}</p>
          </div>
          <div>
            <p class="text-xs uppercase opacity-60">{{ $t('clients.maxFileSize') }}</p>
            <p class="mt-1 font-medium">
              {{ client.max_file_size ? formatBytes(client.max_file_size) : $t('clients.defaultFromEnv') }}
            </p>
          </div>
          <div>
            <p class="text-xs uppercase opacity-60">{{ $t('clients.allowedMimes') }}</p>
            <div class="mt-1 flex flex-wrap gap-1">
              <n-tag v-for="mime in client.allowed_mimes" :key="mime" size="small">{{ mime }}</n-tag>
              <span v-if="!client.allowed_mimes?.length" class="text-sm opacity-70">{{ $t('clients.allMimes') }}</span>
            </div>
          </div>
          <div>
            <p class="text-xs uppercase opacity-60">{{ $t('common.createdAt') }}</p>
            <p class="mt-1 font-medium">{{ formatDate(client.created_at) }}</p>
          </div>
        </div>
      </n-card>

      <n-card :title="$t('clients.devices')" class="rounded-xl" content-style="padding: 0;">
        <n-empty v-if="!devices.length" :description="$t('common.noData')" class="py-10" />

        <div v-else class="space-y-3 p-3 md:hidden">
          <div
            v-for="device in devices"
            :key="device.id"
            class="rounded-lg border border-gray-200 p-3 dark:border-dark-100"
          >
            <p class="truncate font-medium">{{ device.uid }}</p>
            <p class="text-xs opacity-60">{{ device.platform || '—' }} · {{ device.app_version || '—' }}</p>
            <p class="text-xs opacity-60">{{ device.ip || '—' }} · {{ formatDate(device.last_seen_at) }}</p>
            <n-button class="mt-2" size="tiny" type="error" secondary @click="removeDevice(device)">
              {{ $t('common.delete') }}
            </n-button>
          </div>
        </div>

        <div v-if="devices.length" class="hidden overflow-x-auto md:block">
          <n-table :single-line="false" size="small">
            <thead>
              <tr>
                <th>UID</th>
                <th>{{ $t('clients.platform') }}</th>
                <th>{{ $t('clients.appVersion') }}</th>
                <th>IP</th>
                <th>{{ $t('clients.lastSeen') }}</th>
                <th class="text-right">{{ $t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="device in devices" :key="device.id">
                <td class="font-mono text-xs">{{ device.uid }}</td>
                <td>{{ device.platform || '—' }}</td>
                <td>{{ device.app_version || '—' }}</td>
                <td>{{ device.ip || '—' }}</td>
                <td class="whitespace-nowrap">{{ formatDate(device.last_seen_at) }}</td>
                <td>
                  <div class="flex justify-end">
                    <n-button
                      size="tiny"
                      quaternary
                      type="error"
                      :aria-label="$t('common.delete')"
                      @click="removeDevice(device)"
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

      <div v-if="devicesTotal > perPage" class="mt-4 flex justify-center sm:justify-end">
        <n-pagination v-model:page="devicePage" :item-count="devicesTotal" :page-size="perPage" />
      </div>
    </n-spin>
  </div>
</template>
