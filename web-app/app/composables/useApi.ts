import type { ApiError, User } from '~/types/api'

export interface ApiRequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  query?: Record<string, unknown>
  body?: unknown
  headers?: Record<string, string>
  /** Skip the automatic 401 handling (used by the login request itself). */
  skipAuthRedirect?: boolean
}

/** Cookie holding the admin JWT. */
export function useTokenCookie() {
  const config = useRuntimeConfig()
  return useCookie<string | null>('token', {
    sameSite: 'lax',
    secure: Boolean(config.public.cookieSecure),
    path: '/',
    maxAge: 60 * 60 * 24 * 30,
  })
}

/** Normalize whatever `$fetch` throws into `{ message, errors, status }`. */
export function normalizeApiError(error: unknown, fallbackMessage = 'Request failed'): ApiError {
  const anyError = error as {
    data?: { message?: string; errors?: Record<string, string[]> }
    statusCode?: number
    status?: number
    message?: string
  }
  const status = anyError?.statusCode ?? anyError?.status
  return {
    message: anyError?.data?.message || anyError?.message || fallbackMessage,
    errors: anyError?.data?.errors,
    status,
  }
}

/** Nuxt app instance extended with the in-flight token refresh request. */
interface ApiNuxtApp {
  _refreshPromise?: Promise<boolean> | null
}

export function useApi() {
  const config = useRuntimeConfig()
  const token = useTokenCookie()
  const baseURL = config.public.apiBase as string
  // Captured in setup scope: `clearSession` may run from an async callback
  // where the Nuxt instance is no longer active.
  const route = useRoute()
  // Kept on the Nuxt app instance (per request on the server, a singleton on
  // the client) rather than module-level, so it can't leak between unrelated
  // server requests while still deduping concurrent 401s on the client.
  const nuxtApp = useNuxtApp() as ReturnType<typeof useNuxtApp> & ApiNuxtApp
  const user = useState<User | null>('auth-user', () => null)

  const clearSession = async () => {
    token.value = null
    user.value = null
    if (route.path !== '/auth/sign-in') {
      await nuxtApp.runWithContext(() => navigateTo('/auth/sign-in'))
    }
  }

  const REFRESH_PATH = '/admin/auth/refresh'

  /** Swap the current token for a fresh one. Returns false when it fails. */
  async function refresh(): Promise<boolean> {
    if (!token.value) return false
    // Concurrent 401s share one in-flight refresh instead of racing separate requests.
    if (nuxtApp._refreshPromise) return nuxtApp._refreshPromise
    nuxtApp._refreshPromise = (async () => {
      try {
        const response = await $fetch<{ token: string }>(REFRESH_PATH, {
          baseURL,
          method: 'POST',
          headers: { Accept: 'application/json', Authorization: `Bearer ${token.value}` },
        })
        if (!response?.token) return false
        token.value = response.token
        return true
      } catch {
        return false
      } finally {
        nuxtApp._refreshPromise = null
      }
    })()
    return nuxtApp._refreshPromise
  }

  /** Perform a JSON request against the admin API. */
  async function request<T>(path: string, options: ApiRequestOptions = {}, retried = false): Promise<T> {
    const headers: Record<string, string> = {
      Accept: 'application/json',
      ...(options.headers || {}),
    }
    if (token.value) headers.Authorization = `Bearer ${token.value}`

    try {
      return await $fetch<T>(path, {
        baseURL,
        method: options.method || 'GET',
        query: options.query,
        body: options.body as never,
        headers,
      })
    } catch (error) {
      const normalized = normalizeApiError(error)
      if (normalized.status === 401 && !options.skipAuthRedirect) {
        // One refresh attempt before giving up on the session.
        if (!retried && path !== REFRESH_PATH && token.value && (await refresh())) {
          return request<T>(path, options, true)
        }
        await clearSession()
      }
      throw normalized
    }
  }

  const get = <T>(path: string, query?: Record<string, unknown>) => request<T>(path, { query })
  const post = <T>(path: string, body?: unknown, query?: Record<string, unknown>) =>
    request<T>(path, { method: 'POST', body, query })
  const put = <T>(path: string, body?: unknown) => request<T>(path, { method: 'PUT', body })
  const del = <T>(path: string, query?: Record<string, unknown>) =>
    request<T>(path, { method: 'DELETE', query })

  /**
   * Multipart upload with progress. `$fetch` cannot report upload progress,
   * so XMLHttpRequest is used directly.
   */
  function sendUpload<T>(
    path: string,
    formData: FormData,
    onProgress?: (percent: number) => void,
  ): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `${baseURL}${path}`)
      xhr.setRequestHeader('Accept', 'application/json')
      if (token.value) xhr.setRequestHeader('Authorization', `Bearer ${token.value}`)

      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable && onProgress) {
          onProgress(Math.round((event.loaded / event.total) * 100))
        }
      }

      xhr.onload = () => {
        let payload: unknown = null
        try {
          payload = xhr.responseText ? JSON.parse(xhr.responseText) : null
        } catch {
          payload = null
        }

        if (xhr.status >= 200 && xhr.status < 300) {
          resolve(payload as T)
          return
        }

        const body = (payload || {}) as { message?: string; errors?: Record<string, string[]> }
        reject({
          message: body.message || `Upload failed (${xhr.status})`,
          errors: body.errors,
          status: xhr.status,
        } satisfies ApiError)
      }

      xhr.onerror = () => reject({ message: 'Network error during upload' } satisfies ApiError)
      xhr.onabort = () => reject({ message: 'Upload aborted' } satisfies ApiError)

      xhr.send(formData)
    })
  }

  /**
   * Multipart upload with progress. `$fetch` cannot report upload progress,
   * so XMLHttpRequest is used directly.
   */
  async function upload<T>(
    path: string,
    formData: FormData,
    onProgress?: (percent: number) => void,
  ): Promise<T> {
    if (import.meta.server) {
      throw { message: 'Upload is only available in the browser' } satisfies ApiError
    }

    try {
      return await sendUpload<T>(path, formData, onProgress)
    } catch (error) {
      const apiError = error as ApiError
      if (apiError.status === 401 && token.value && (await refresh())) {
        try {
          return await sendUpload<T>(path, formData, onProgress)
        } catch (retryError) {
          const retryApiError = retryError as ApiError
          if (retryApiError.status === 401) await clearSession()
          throw retryApiError
        }
      }
      if (apiError.status === 401) await clearSession()
      throw apiError
    }
  }

  /** Filename from a `Content-Disposition` header, `filename*=` first. */
  function filenameFromDisposition(header: string | null): string | null {
    if (!header) return null
    const extended = /filename\*\s*=\s*(?:UTF-8|utf-8)?''([^;]+)/i.exec(header)
    if (extended?.[1]) {
      try {
        return decodeURIComponent(extended[1].trim())
      } catch {
        return extended[1].trim()
      }
    }
    const plain = /filename\s*=\s*("([^"]*)"|[^;]+)/i.exec(header)
    const value = (plain?.[2] ?? plain?.[1] ?? '').trim()
    return value || null
  }

  /** Download a protected binary endpoint through fetch and save it locally. */
  async function download(path: string, filename: string): Promise<void> {
    const headers: Record<string, string> = {}
    if (token.value) headers.Authorization = `Bearer ${token.value}`

    const response = await fetch(`${baseURL}${path}`, { headers })
    if (!response.ok) {
      if (response.status === 401) await clearSession()
      let message = `Download failed (${response.status})`
      try {
        const body = await response.json()
        if (body?.message) message = body.message
      } catch {
        // keep default message
      }
      throw { message, status: response.status } satisfies ApiError
    }

    const blob = await response.blob()
    const serverName = filenameFromDisposition(response.headers.get('content-disposition'))
    const objectUrl = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = objectUrl
    link.download = serverName || filename
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(objectUrl)
  }

  return { baseURL, request, get, post, put, del, upload, download, refresh, token }
}
