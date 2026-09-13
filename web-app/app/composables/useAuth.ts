import type { User } from '~/types/api'

/** Nuxt app instance extended with the in-flight `/admin/auth/me` request. */
interface AuthNuxtApp {
  _mePromise?: Promise<User | null> | null
}

/**
 * Admin session state: token cookie plus the currently signed-in user.
 */
export function useAuth() {
  const api = useApi()
  const token = useTokenCookie()
  const user = useState<User | null>('auth-user', () => null)
  // Kept on the Nuxt app instance (per request on the server, a singleton on
  // the client) rather than module-level, so two components calling
  // `ensureUser()` in the same tick share one request without leaking it
  // across unrelated server requests.
  const nuxtApp = useNuxtApp() as ReturnType<typeof useNuxtApp> & AuthNuxtApp

  /** True for the `super_admin` role, which manages users and sees every client. */
  const isSuperAdmin = computed(() => user.value?.role?.slug === 'super_admin')

  async function login(email: string, password: string): Promise<User> {
    const response = await api.request<{ token: string; user: User }>('/admin/auth/login', {
      method: 'POST',
      body: { email, password },
      skipAuthRedirect: true,
    })
    token.value = response.token
    user.value = response.user
    return response.user
  }

  async function fetchMe(): Promise<User | null> {
    if (!token.value) return null
    const response = await api.get<{ data: User }>('/admin/auth/me')
    user.value = response.data
    return response.data
  }

  /** Load the signed-in user once; concurrent callers share the same request. */
  async function ensureUser(): Promise<User | null> {
    if (user.value) return user.value
    if (!token.value) return null
    if (!nuxtApp._mePromise) {
      nuxtApp._mePromise = fetchMe().finally(() => {
        nuxtApp._mePromise = null
      })
    }
    return nuxtApp._mePromise
  }

  async function logout(): Promise<void> {
    try {
      if (token.value) await api.post('/admin/auth/logout')
    } catch {
      // Signing out locally must succeed even when the API is unreachable.
    } finally {
      token.value = null
      user.value = null
      await navigateTo('/auth/sign-in')
    }
  }

  return { user, token, isSuperAdmin, login, logout, fetchMe, ensureUser, refresh: api.refresh }
}
