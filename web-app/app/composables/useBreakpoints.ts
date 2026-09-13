/**
 * Reactive viewport helpers based on the Tailwind `md` breakpoint (768px).
 * Defaults to desktop during SSR so the markup stays stable before hydration.
 */
export function useBreakpoints() {
  const isMobile = ref(false)

  if (import.meta.client) {
    const query = window.matchMedia('(max-width: 767px)')
    const update = () => {
      isMobile.value = query.matches
    }
    update()
    query.addEventListener('change', update)
    onScopeDispose(() => query.removeEventListener('change', update))
  }

  return { isMobile }
}
