import { NIcon } from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import {
  ArchiveOutline,
  FolderOpenOutline,
  GridOutline,
  ListOutline,
  PeopleOutline,
  PersonOutline,
  ShieldCheckmarkOutline,
  SyncOutline,
} from '@vicons/ionicons5'
import type { Component } from 'vue'

/** Shared drawer state for the mobile sidebar. */
export function useSidebarDrawer() {
  return useState<boolean>('sidebar-drawer', () => false)
}

function renderIcon(icon: Component) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

/**
 * Sidebar menu options, plus the active key derived from the current route.
 */
export function useMenu() {
  const { t } = useI18n()
  const route = useRoute()
  const localePath = useLocalePath()
  const { isSuperAdmin } = useAuth()

  // `superOnly` entries are hidden for the `admin` role; the API also 403s them.
  const allItems = computed<Array<MenuOption & { path: string; superOnly?: boolean }>>(() => [
    { label: t('menu.dashboard'), key: 'dashboard', path: '/', icon: renderIcon(GridOutline) },
    { label: t('menu.clients'), key: 'clients', path: '/clients', icon: renderIcon(PeopleOutline) },
    { label: t('menu.files'), key: 'files', path: '/files', icon: renderIcon(FolderOpenOutline) },
    { label: t('menu.archives'), key: 'archives', path: '/archives', icon: renderIcon(ArchiveOutline) },
    { label: t('menu.auditLogs'), key: 'audit-logs', path: '/audit-logs', icon: renderIcon(ListOutline) },
    {
      label: t('menu.users'),
      key: 'users',
      path: '/users',
      icon: renderIcon(ShieldCheckmarkOutline),
      superOnly: true,
    },
    { label: t('menu.profile'), key: 'profile', path: '/profile', icon: renderIcon(PersonOutline) },
    {
      label: t('menu.storageSync'),
      key: 'storage',
      path: '/settings/storage',
      icon: renderIcon(SyncOutline),
      superOnly: true,
    },
  ])

  const items = computed(() => allItems.value.filter((item) => !item.superOnly || isSuperAdmin.value))

  const options = computed<MenuOption[]>(() =>
    items.value.map((item) => ({
      label: () => h(resolveComponent('NuxtLink'), { to: localePath(item.path) }, { default: () => item.label }),
      key: item.key,
      icon: item.icon,
    })),
  )

  const activeKey = computed(() => {
    const path = route.path
    if (path === '/' || path === '') return 'dashboard'
    const match = allItems.value
      .filter((item) => item.path !== '/')
      .find((item) => path.startsWith(item.path))
    return match?.key ?? 'dashboard'
  })

  return { items, options, activeKey }
}
