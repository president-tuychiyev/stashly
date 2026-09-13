<script setup lang="ts">
import {
  CloudyNightOutline,
  GlobeOutline,
  LogOutOutline,
  MenuOutline,
  PersonCircleOutline,
  PersonOutline,
  SunnyOutline,
} from '@vicons/ionicons5'
import { NIcon } from 'naive-ui'
import type { DropdownOption } from 'naive-ui'
import type { Component } from 'vue'

const { isDark, toggleDark } = useDarkMode()
const { locale, setLocale, locales, t } = useI18n()
const { user, logout } = useAuth()
const drawer = useSidebarDrawer()

const languageOptions = computed(() =>
  (locales.value as Array<{ code: string; name?: string }>).map((item) => ({
    label: item.name || item.code,
    key: item.code,
  })),
)

const currentLanguage = computed(
  () => languageOptions.value.find((item) => item.key === locale.value)?.label ?? locale.value,
)

const handleLanguageSelect = (key: string) => {
  setLocale(key as never)
}

const renderIcon = (icon: Component) => () => h(NIcon, null, { default: () => h(icon) })

const userLabel = computed(() => user.value?.name || t('common.admin'))
const roleLabel = computed(() => {
  const slug = user.value?.role?.slug
  if (!slug) return ''
  const key = `users.roles.${slug}`
  const translated = t(key)
  return translated === key ? (user.value?.role?.name ?? slug) : translated
})

const userOptions = computed<DropdownOption[]>(() => [
  {
    key: 'header',
    type: 'render',
    render: () =>
      h('div', { class: 'px-3 py-2' }, [
        h('p', { class: 'truncate text-sm font-medium' }, userLabel.value),
        h('p', { class: 'truncate text-xs opacity-60' }, user.value?.email || ''),
        roleLabel.value ? h('p', { class: 'mt-0.5 truncate text-xs text-teal-500' }, roleLabel.value) : null,
      ]),
  },
  { key: 'divider', type: 'divider' },
  { label: t('menu.profile'), key: 'profile', icon: renderIcon(PersonOutline) },
  { label: t('common.logout'), key: 'logout', icon: renderIcon(LogOutOutline) },
])

const handleUserSelect = async (key: string) => {
  if (key === 'profile') await navigateTo('/profile')
  if (key === 'logout') await logout()
}
</script>

<template>
  <div class="flex items-center justify-between gap-x-2 border-b border-gray-200 px-3 py-2 dark:border-dark-100">
    <div class="flex min-w-0 items-center gap-x-2">
      <div class="md:hidden">
        <n-button quaternary :aria-label="$t('common.openMenu')" @click="drawer = true">
          <template #icon>
            <MenuOutline />
          </template>
        </n-button>
      </div>
      <span class="truncate text-sm font-medium">{{ $t('app.title') }}</span>
    </div>

    <div class="flex items-center gap-x-2">
      <n-button
        size="small"
        quaternary
        :title="$t('common.toggleTheme')"
        :aria-label="$t('common.toggleTheme')"
        @click="toggleDark"
      >
        <template #icon>
          <SunnyOutline v-if="isDark" />
          <CloudyNightOutline v-else />
        </template>
      </n-button>

      <n-dropdown :options="languageOptions" trigger="click" placement="bottom-end" @select="handleLanguageSelect">
        <n-button size="small" quaternary :aria-label="$t('common.language')">
          <template #icon>
            <GlobeOutline />
          </template>
          <span class="hidden sm:inline">{{ currentLanguage }}</span>
        </n-button>
      </n-dropdown>

      <n-dropdown
        :options="userOptions"
        trigger="click"
        placement="bottom-end"
        @select="handleUserSelect"
      >
        <n-button size="small" quaternary :aria-label="$t('profile.menuLabel')">
          <template #icon>
            <PersonCircleOutline />
          </template>
          <span class="hidden max-w-[10rem] truncate sm:inline">{{ userLabel }}</span>
        </n-button>
      </n-dropdown>
    </div>
  </div>
</template>
