import type { Paginated, User } from '~/types/api'

/**
 * Flat list of admin users for `n-select` (client owner, delete reassignment).
 * Only `super_admin` may call `/admin/users`; other roles get an empty list.
 */
export function useUserOptions() {
  const api = useApi()
  const users = ref<User[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  const options = computed(() =>
    users.value.map((item) => ({ label: `${item.name} (${item.email})`, value: item.id })),
  )

  /** Page through `/admin/users`; repeated calls are ignored once loaded. */
  const load = async (force = false) => {
    if (loading.value) return
    if (loaded.value && !force) return
    loading.value = true
    try {
      const collected: User[] = []
      let page = 1
      let lastPage = 1
      do {
        const response = await api.get<Paginated<User>>('/admin/users', { page, per_page: 100 })
        collected.push(...(response.data ?? []))
        lastPage = response.meta?.last_page ?? page
        page++
      } while (page <= lastPage)
      users.value = collected
      loaded.value = true
    } catch {
      users.value = []
    } finally {
      loading.value = false
    }
  }

  return { users, options, loading, loaded, load }
}
