/** Routes reachable without a token. */
const PUBLIC_ROUTES = ['/auth/sign-in', '/auth/verify', '/auth/forgot', '/auth/reset']

export default defineNuxtRouteMiddleware((to) => {
  const token = useTokenCookie()
  const isPublic = PUBLIC_ROUTES.includes(to.path)

  if (!token.value && !isPublic) {
    return navigateTo({ path: '/auth/sign-in', query: { redirect: to.fullPath } })
  }

  // A signed-in user has no business on the sign-in page, but may still need
  // the OTP flows (e.g. following a reset link while a stale token is around).
  if (token.value && to.path === '/auth/sign-in') {
    return navigateTo('/')
  }
})
