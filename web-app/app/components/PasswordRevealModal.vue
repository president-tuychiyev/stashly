<script setup lang="ts">
import { CopyOutline } from '@vicons/ionicons5'

const show = defineModel<boolean>('show', { default: false })
const props = defineProps<{ password: string | null; username?: string | null }>()

const message = useMessage()
const { t } = useI18n()

const copy = async () => {
  if (!props.password) return
  try {
    await navigator.clipboard.writeText(props.password)
    message.success(t('common.copied'))
  } catch {
    message.error(t('common.copyFailed'))
  }
}
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$t('clients.passwordModalTitle')"
    class="w-[95vw] max-w-md"
    :mask-closable="false"
  >
    <n-alert type="warning" :show-icon="true" class="mb-4">
      {{ $t('clients.passwordWarning') }}
    </n-alert>

    <n-descriptions v-if="username" :column="1" label-placement="left" class="mb-3">
      <n-descriptions-item :label="$t('clients.username')">{{ username }}</n-descriptions-item>
    </n-descriptions>

    <n-input-group>
      <n-input :value="password || ''" readonly />
      <n-button type="primary" :aria-label="$t('common.copy')" @click="copy">
        <template #icon>
          <CopyOutline />
        </template>
      </n-button>
    </n-input-group>

    <template #footer>
      <div class="flex justify-end">
        <n-button @click="show = false">{{ $t('common.close') }}</n-button>
      </div>
    </template>
  </n-modal>
</template>
