<script setup lang="ts">
import type { FormInst, FormRules } from 'naive-ui'
import type { ApiError, User } from '~/types/api'

definePageMeta({ layout: 'auth' })

const { t } = useI18n()
const route = useRoute()
const api = useApi()
const message = useMessage()
const token = useTokenCookie()
const user = useState<User | null>('auth-user', () => null)
const resend = useCountdown()

const formRef = ref<FormInst | null>(null)
const form = reactive({
  email: queryString(route.query.email) ?? '',
  code: '',
  password: '',
  passwordConfirmation: '',
})
const loading = ref(false)
const resending = ref(false)
const error = ref<ApiError | null>(null)

const passwordMessage = (value: string) => {
  const issue = passwordIssue(value)
  if (!issue) return null
  return t(`auth.password${issue.charAt(0).toUpperCase()}${issue.slice(1)}`)
}

/** First message the API reported for a field, or null. */
const fieldError = (apiError: ApiError | null, field: string): string | null => {
  const text = joinErrorMessages(apiError?.errors?.[field])
  return text || null
}

// The cooldown belongs to whichever address the code was sent to; a new
// address means the old cooldown is meaningless.
watch(() => form.email, () => resend.stop())

const rules: FormRules = {
  email: {
    required: true,
    trigger: ['blur', 'input'],
    validator: (_rule, value: string) => (isValidEmail(value) ? true : new Error(t('auth.emailInvalid'))),
  },
  code: {
    required: true,
    trigger: ['blur', 'change'],
    validator: (_rule, value: string) => (isValidOtp(value) ? true : new Error(t('auth.codeInvalid'))),
  },
  password: {
    required: true,
    trigger: ['blur', 'input'],
    validator: (_rule, value: string) => (isStrongPassword(value) ? true : new Error(passwordMessage(value)!)),
  },
  passwordConfirmation: {
    required: true,
    trigger: ['blur', 'input'],
    validator: (_rule, value: string) =>
      value === form.password ? true : new Error(t('auth.passwordMismatch')),
  },
}

const onSubmit = async () => {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  loading.value = true
  error.value = null
  try {
    const response = await api.request<{ token: string; user: User }>('/admin/auth/verify', {
      method: 'POST',
      body: {
        email: form.email.trim(),
        code: form.code,
        password: form.password,
        password_confirmation: form.passwordConfirmation,
      },
      skipAuthRedirect: true,
    })
    token.value = response.token
    user.value = response.user
    message.success(t('auth.verified'))
    await navigateTo('/')
  } catch (e) {
    error.value = e as ApiError
  } finally {
    loading.value = false
  }
}

const onResend = async () => {
  if (resend.active.value || resending.value) return
  if (!isValidEmail(form.email)) {
    error.value = { message: t('auth.emailInvalid') }
    return
  }
  resending.value = true
  error.value = null
  try {
    await api.request('/admin/auth/verify/resend', {
      method: 'POST',
      body: { email: form.email.trim() },
      skipAuthRedirect: true,
    })
    // The API answers 204 regardless, so the cooldown always starts.
    resend.start(OTP_RESEND_SECONDS)
    message.success(t('auth.codeSent'))
  } catch (e) {
    const apiError = e as ApiError
    // 429 means a code was already sent recently; treat it the same as
    // success (the cooldown starts either way) instead of a red error.
    if (apiError.status === 429) {
      resend.start(OTP_RESEND_SECONDS)
      message.success(t('auth.codeSent'))
    } else {
      error.value = apiError
    }
  } finally {
    resending.value = false
  }
}
</script>

<template>
  <AuthCard :title="$t('auth.verifyTitle')" :subtitle="$t('auth.verifySubtitle')">
    <n-alert v-if="error" type="error" class="mb-4">{{ error.message }}</n-alert>

    <n-form ref="formRef" :model="form" :rules="rules" size="large" @keydown.enter.prevent="onSubmit">
      <n-form-item
        :label="$t('auth.email')"
        path="email"
        :validation-status="fieldError(error, 'email') ? 'error' : undefined"
        :feedback="fieldError(error, 'email') ?? undefined"
      >
        <n-input v-model:value="form.email" inputmode="email" autocomplete="email" />
      </n-form-item>

      <n-form-item
        :label="$t('auth.code')"
        path="code"
        :validation-status="fieldError(error, 'code') ? 'error' : undefined"
        :feedback="fieldError(error, 'code') ?? undefined"
      >
        <OtpInput v-model:value="form.code" :aria-label="$t('auth.code')" @finish="onSubmit" />
      </n-form-item>

      <n-form-item
        :label="$t('auth.newPassword')"
        path="password"
        :validation-status="fieldError(error, 'password') ? 'error' : undefined"
        :feedback="fieldError(error, 'password') ?? undefined"
      >
        <n-input
          v-model:value="form.password"
          type="password"
          show-password-on="click"
          autocomplete="new-password"
        />
      </n-form-item>

      <n-form-item
        :label="$t('auth.confirmPassword')"
        path="passwordConfirmation"
        :validation-status="fieldError(error, 'password_confirmation') ? 'error' : undefined"
        :feedback="fieldError(error, 'password_confirmation') ?? undefined"
      >
        <n-input
          v-model:value="form.passwordConfirmation"
          type="password"
          show-password-on="click"
          autocomplete="new-password"
        />
      </n-form-item>

      <p class="-mt-2 mb-4 text-xs opacity-60">{{ $t('auth.passwordHint') }}</p>

      <n-button type="primary" block :loading="loading" @click="onSubmit">
        {{ $t('auth.verifyAction') }}
      </n-button>
    </n-form>

    <div class="mt-4 flex flex-col items-center gap-2 sm:flex-row sm:justify-between">
      <n-button
        quaternary
        size="small"
        :loading="resending"
        :disabled="resend.active.value"
        @click="onResend"
      >
        {{ resend.active.value ? $t('auth.resendIn', { seconds: resend.remaining.value }) : $t('auth.resendCode') }}
      </n-button>
      <NuxtLink to="/auth/sign-in" class="text-sm text-teal-500 hover:underline">
        {{ $t('auth.backToSignIn') }}
      </NuxtLink>
    </div>
  </AuthCard>
</template>
