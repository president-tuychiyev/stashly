<script setup lang="ts">
import { CloudyNightOutline, GlobeOutline, SunnyOutline } from '@vicons/ionicons5'

/** Shared shell for the public auth pages: logo, title, language and theme switch. */
defineProps<{ title: string; subtitle?: string }>()

const { locale, setLocale, locales } = useI18n()
const { isDark, toggleDark } = useDarkMode()

const languageOptions = computed(() =>
  (locales.value as Array<{ code: string; name?: string }>).map((item) => ({
    label: item.name || item.code,
    key: item.code,
  })),
)
</script>

<template>
  <div class="flex min-h-screen items-center justify-center p-4">
    <div class="w-full max-w-md">
      <n-card class="rounded-2xl" content-style="padding: 1.5rem;">
        <div class="mb-6 flex justify-center">
          <ApplicationLogo class="h-14" />
        </div>

        <div class="mb-5 flex items-start justify-between gap-3">
          <div class="min-w-0">
            <h1 class="text-2xl font-medium text-teal-500">{{ title }}</h1>
            <p v-if="subtitle" class="mt-1 text-sm opacity-70">{{ subtitle }}</p>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <n-dropdown
              :options="languageOptions"
              trigger="click"
              placement="bottom-end"
              @select="(key: string) => setLocale(key as never)"
            >
              <n-button size="small" quaternary :aria-label="$t('common.language')">
                <template #icon>
                  <GlobeOutline />
                </template>
                <span class="hidden sm:inline">{{ locale }}</span>
              </n-button>
            </n-dropdown>
            <n-button
              size="small"
              quaternary
              :aria-label="$t('common.toggleTheme')"
              @click="toggleDark"
            >
              <template #icon>
                <SunnyOutline v-if="isDark" />
                <CloudyNightOutline v-else />
              </template>
            </n-button>
          </div>
        </div>

        <slot />

        <p class="mt-6 text-center text-xs opacity-50">{{ $t('app.title') }}</p>
      </n-card>
    </div>
  </div>
</template>
