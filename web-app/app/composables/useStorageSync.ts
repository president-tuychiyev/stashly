import type { ApiError, StorageSyncResult } from '~/types/api'

/**
 * Storage reconciliation shared by the dashboard and the settings page.
 * The run is destructive, so it always goes through a confirm dialog.
 */
export function useStorageSync() {
  const api = useApi()
  const { t } = useI18n()
  const dialog = useDialog()

  const running = ref(false)
  const result = ref<StorageSyncResult | null>(null)
  const error = ref<ApiError | null>(null)

  /**
   * Ask for confirmation, then run the sync.
   * @param onDone called with the result after a successful run.
   * @param onError called instead of setting `error` when provided.
   */
  const run = (options: {
    onDone?: (result: StorageSyncResult) => void
    onError?: (error: ApiError) => void
  } = {}) => {
    dialog.warning({
      title: t('storage.sync'),
      content: t('storage.syncConfirm'),
      positiveText: t('common.confirm'),
      negativeText: t('common.cancel'),
      onPositiveClick: async () => {
        running.value = true
        error.value = null
        try {
          const response = await api.post<StorageSyncResult>('/admin/storage/sync')
          result.value = response
          options.onDone?.(response)
        } catch (e) {
          result.value = null
          if (options.onError) options.onError(e as ApiError)
          else error.value = e as ApiError
        } finally {
          running.value = false
        }
      },
    })
  }

  return { running, result, error, run }
}
