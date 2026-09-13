<script setup lang="ts">
import { darkTheme } from 'naive-ui'

const { isDark } = useDarkMode()
</script>

<template>
  <naive-config>
    <!-- Only the base theme lives here; the overrides come from `naiveui.themeConfig`. -->
    <n-config-provider :theme="isDark ? darkTheme : null" inline-theme-disabled>
      <n-dialog-provider>
        <n-message-provider :max="3">
          <NuxtLayout>
            <NuxtPage />
          </NuxtLayout>
        </n-message-provider>
      </n-dialog-provider>
    </n-config-provider>
  </naive-config>
</template>
