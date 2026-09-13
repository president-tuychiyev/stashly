import type { Client, Paginated } from '~/types/api'

/** Loads a flat list of clients for `n-select` filters. */
export function useClientOptions() {
  const api = useApi()
  const clients = ref<Client[]>([])
  const loading = ref(false)

  const options = computed(() =>
    clients.value.map((client) => ({ label: `${client.name} (@${client.username})`, value: client.id })),
  )

  const load = async () => {
    loading.value = true
    try {
      const collected: Client[] = []
      let page = 1
      let lastPage = 1
      // Page through so clients beyond the first 100 still show up.
      do {
        const response = await api.get<Paginated<Client>>('/admin/clients', { page, per_page: 100 })
        collected.push(...(response.data ?? []))
        lastPage = response.meta?.last_page ?? page
        page++
      } while (page <= lastPage)
      clients.value = collected
    } catch {
      clients.value = []
    } finally {
      loading.value = false
    }
  }

  return { clients, options, loading, load }
}
