<script setup lang="ts">
import type { FormInst, FormRules } from 'naive-ui'
import type { ApiError } from '~/types/api'

definePageMeta({ layout: 'auth' })

const { t } = useI18n()
const { login } = useAuth()
const message = useMessage()
const route = useRoute()

/** Only ever navigate back to a same-app path; never an off-site or protocol-relative one. */
const safeRedirect = computed(() => {
  const target = queryString(route.query.redirect)
  return target && target.startsWith('/') && !target.startsWith('//') ? target : '/'
})

const formRef = ref<FormInst | null>(null)
const form = reactive({ email: '', password: '' })
const loading = ref(false)
const error = ref<ApiError | null>(null)

const rules: FormRules = {
  email: {
    required: true,
    trigger: ['blur', 'input'],
    validator: (_rule, value: string) => (isValidEmail(value) ? true : new Error(t('auth.emailInvalid'))),
  },
  password: { required: true, trigger: ['blur', 'input'], message: () => t('auth.passwordRequired') },
}

/** 403 splits into two very different situations: unverified vs blocked. */
const notVerified = computed(
  () => error.value?.status === 403 && /not\s*verified|verify/i.test(error.value?.message ?? ''),
)
const blocked = computed(
  () => error.value?.status === 403 && !notVerified.value && /blocked/i.test(error.value?.message ?? ''),
)

const verifyLink = computed(() => ({
  path: '/auth/verify',
  query: form.email ? { email: form.email } : undefined,
}))

const onSubmit = async () => {
  // `validate()` rejects when the form is invalid, so it must be awaited.
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  loading.value = true
  error.value = null
  try {
    await login(form.email.trim(), form.password)
    message.success(t('auth.welcome'))
    await navigateTo(safeRedirect.value)
  } catch (e) {
    error.value = e as ApiError
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthCard :title="$t('auth.signIn')">
    <n-alert v-if="notVerified" type="warning" class="mb-4" :title="$t('auth.notVerifiedTitle')">
      <p>{{ $t('auth.notVerifiedHint') }}</p>
      <NuxtLink :to="verifyLink" class="mt-2 inline-block font-medium text-teal-500 hover:underline">
        {{ $t('auth.goToVerify') }}
      </NuxtLink>
    </n-alert>
    <n-alert v-else-if="blocked" type="error" class="mb-4" :title="$t('auth.blockedTitle')">
      {{ $t('auth.blockedHint') }}
    </n-alert>
    <n-alert v-else-if="error" type="error" class="mb-4">{{ error.message }}</n-alert>

    <n-form ref="formRef" :model="form" :rules="rules" size="large" @keydown.enter.prevent="onSubmit">
      <n-form-item :label="$t('auth.email')" path="email">
        <n-input
          v-model:value="form.email"
          type="text"
          inputmode="email"
          autocomplete="email"
          :placeholder="$t('auth.emailPlaceholder')"
        />
      </n-form-item>
      <n-form-item :label="$t('auth.password')" path="password">
        <n-input
          v-model:value="form.password"
          type="password"
          show-password-on="click"
          autocomplete="current-password"
          :placeholder="$t('auth.passwordPlaceholder')"
        />
      </n-form-item>

      <n-button type="primary" block :loading="loading" @click="onSubmit">
        {{ $t('auth.signIn') }}
      </n-button>
    </n-form>

    <div class="mt-4 flex flex-col gap-2 text-center text-sm sm:flex-row sm:justify-between sm:text-left">
      <NuxtLink to="/auth/forgot" class="text-teal-500 hover:underline">{{ $t('auth.forgotPassword') }}</NuxtLink>
      <NuxtLink :to="verifyLink" class="text-teal-500 hover:underline">{{ $t('auth.haveInvitation') }}</NuxtLink>
    </div>
  </AuthCard>
</template>
