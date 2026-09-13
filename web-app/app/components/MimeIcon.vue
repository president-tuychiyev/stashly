<script setup lang="ts">
import {
  ArchiveOutline,
  CodeSlashOutline,
  DocumentOutline,
  DocumentTextOutline,
  GridOutline,
  ImageOutline,
  MusicalNotesOutline,
  VideocamOutline,
} from '@vicons/ionicons5'

const props = defineProps<{ mime: string | null | undefined; size?: number }>()

const icon = computed(() => {
  switch (mimeKind(props.mime)) {
    case 'image':
      return ImageOutline
    case 'video':
      return VideocamOutline
    case 'audio':
      return MusicalNotesOutline
    case 'pdf':
      return DocumentTextOutline
    case 'archive':
      return ArchiveOutline
    case 'spreadsheet':
      return GridOutline
    case 'text':
      return CodeSlashOutline
    default:
      return DocumentOutline
  }
})
</script>

<template>
  <n-icon :size="size || 18" class="opacity-70">
    <component :is="icon" />
  </n-icon>
</template>
