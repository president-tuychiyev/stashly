<script setup lang="ts">
import type { FormInst, FormRules } from 'naive-ui'
import type { ApiError } from '~/types/api'

definePageMeta({ layout: 'auth' })

const { t } = useI18n()
const api = useApi()
const route = useRoute()

const formRef = ref<FormInst | null>(null)
const form = reactive({ email: queryString(route.query.email) ?? '' })
const loading = ref(false)
const error = ref<ApiError | null>(null)
const sent = ref(false)

const rules: FormRules = {
  email: {
    required: true,
    trigger: ['blur', 'input'],
    validator: (_rule, value: string) => (isValidEmail(value) ? true : new Error(t('auth.emailInvalid'))),
  },
}

const resetLink = computed(() => ({
  path: '/auth/reset',
  query: form.email ? { email: form.email.trim() } : undefined,
}))

const onSubmit = async () => {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  loading.value = true
  error.value = null
  try {
    await api.request('/admin/auth/forgot', {
      method: 'POST',
      body: { email: form.email.trim() },
      skipAuthRedirect: true,
    })
    // Always report success: the API never reveals whether the address exists.
    sent.value = true
  } catch (e) {
    const apiError = e as ApiError
    // Same reason: a rate limit must not turn into an enumeration signal.
    if (apiError.status === 429) sent.value = true
    else error.value = apiError
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthCard :title="$t('auth.forgotTitle')" :subtitle="$t('auth.forgotSubtitle')">
    <n-alert v-if="error" type="error" class="mb-4">{{ error.message }}</n-alert>

    <template v-if="sent">
      <n-alert type="success" class="mb-4" :title="$t('auth.forgotSentTitle')">
        {{ $t('auth.forgotSent') }}
      </n-alert>
      <NuxtLink :to="resetLink">
        <n-button type="primary" block>{{ $t('auth.goToReset') }}</n-button>
      </NuxtLink>
    </template>

    <n-form v-else ref="formRef" :model="form" :rules="rules" size="large" @keydown.enter.prevent="onSubmit">
      <n-form-item :label="$t('auth.email')" path="email">
        <n-input
          v-model:value="form.email"
          inputmode="email"
          autocomplete="email"
          :placeholder="$t('auth.emailPlaceholder')"
        />
      </n-form-item>

      <n-button type="primary" block :loading="loading" @click="onSubmit">
        {{ $t('auth.sendCode') }}
      </n-button>
    </n-form>

    <div class="mt-4 text-center">
      <NuxtLink to="/auth/sign-in" class="text-sm text-teal-500 hover:underline">
        {{ $t('auth.backToSignIn') }}
      </NuxtLink>
    </div>
  </AuthCard>
</template>
