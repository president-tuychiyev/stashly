<script setup lang="ts">
/** Renders as a centered modal on desktop and as a bottom-anchored drawer on mobile. */
const show = defineModel<boolean>('show', { default: false })
withDefaults(defineProps<{ title: string; width?: number }>(), { width: 560 })

const { isMobile } = useBreakpoints()
</script>

<template>
  <ClientOnly>
    <n-drawer v-if="isMobile" v-model:show="show" placement="bottom" height="88%">
      <n-drawer-content :title="title" closable :native-scrollbar="false">
        <slot />
        <template v-if="$slots.footer" #footer>
          <slot name="footer" />
        </template>
      </n-drawer-content>
    </n-drawer>

    <n-modal
      v-else
      v-model:show="show"
      preset="card"
      :title="title"
      :style="{ width: `${width}px`, maxWidth: '95vw' }"
    >
      <slot />
      <template v-if="$slots.footer" #footer>
        <slot name="footer" />
      </template>
    </n-modal>
  </ClientOnly>
</template>
