<script setup lang="ts">
const drawer = useSidebarDrawer()
const { ensureUser } = useAuth()

// Load the signed-in user once per session; failures are non-fatal.
onMounted(async () => {
  try {
    await ensureUser()
  } catch {
    // The API may be unreachable; the panel still renders.
  }
})
</script>

<template>
  <div class="flex min-h-screen">
    <!-- Desktop sidebar; on < md it is replaced by the drawer below. -->
    <aside class="hidden w-60 shrink-0 border-r border-gray-200 bg-white md:block dark:border-dark-100 dark:bg-dark-400">
      <div class="sticky top-0 h-screen">
        <SidebarContent />
      </div>
    </aside>

    <div class="flex min-w-0 flex-1 flex-col">
      <Navbar />
      <main class="flex-1 bg-slate-50 dark:bg-dark-500">
        <div class="p-3 sm:p-5 lg:p-8">
          <slot />
        </div>
      </main>
    </div>

    <n-drawer v-model:show="drawer" :width="260" placement="left">
      <n-drawer-content :native-scrollbar="false" body-content-style="padding: 0">
        <SidebarContent />
      </n-drawer-content>
    </n-drawer>
  </div>
</template>
