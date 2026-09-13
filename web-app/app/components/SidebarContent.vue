<script setup lang="ts">
import { LogOutOutline } from '@vicons/ionicons5'

const { options, activeKey } = useMenu()
const { logout } = useAuth()
const drawer = useSidebarDrawer()

const onLogout = async () => {
  drawer.value = false
  await logout()
}
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="flex justify-center py-4">
      <ApplicationLogo class="h-12" />
    </div>

    <div class="flex-1 overflow-y-auto">
      <n-menu :value="activeKey" :options="options" @update:value="drawer = false" />
    </div>

    <div class="border-t border-gray-200 p-4 dark:border-dark-100">
      <n-button type="error" secondary block @click="onLogout">
        <template #icon>
          <LogOutOutline />
        </template>
        {{ $t('common.logout') }}
      </n-button>
    </div>
  </div>
</template>
