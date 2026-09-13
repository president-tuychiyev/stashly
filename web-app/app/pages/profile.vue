<script setup lang="ts">
import type { ApiError, User } from '~/types/api'

const { t } = useI18n()
const api = useApi()
const message = useMessage()
const { user, ensureUser } = useAuth()
const resend = useCountdown()

const loading = ref(true)
const loadError = ref<ApiError | null>(null)

/** First message the API reported for a field, or null. */
const fieldError = (error: ApiError | null, field: string): string | null => {
  const value = error?.errors?.[field]
  const text = joinErrorMessages(value)
  return text || null
}

const load = async () => {
  loading.value = true
  loadError.value = null
  try {
    const response = await api.get<{ data: User }>('/admin/profile')
    user.value = response.data
    nameForm.name = response.data.name
  } catch (e) {
    loadError.value = e as ApiError
    // Fall back to whatever the session already knows.
    try {
      const current = await ensureUser()
      if (current) nameForm.name = current.name
    } catch {
      // Nothing else to try.
    }
  } finally {
    loading.value = false
  }
}

const roleLabel = computed(() => {
  const slug = user.value?.role?.slug
  if (!slug) return ''
  const key = `users.roles.${slug}`
  const translated = t(key)
  return translated === key ? (user.value?.role?.name ?? slug) : translated
})

/* ---------------------------------------------------------------- name */

const nameForm = reactive({ name: '' })
const nameSaving = ref(false)
const nameError = ref<ApiError | null>(null)

const saveName = async () => {
  const name = nameForm.name.trim()
  if (!name) {
    nameError.value = { message: t('profile.nameRequired'), errors: { name: [t('profile.nameRequired')] } }
    return
  }
  nameSaving.value = true
  nameError.value = null
  try {
    const response = await api.put<{ data: User }>('/admin/profile', { name })
    user.value = response.data
    nameForm.name = response.data.name
    message.success(t('profile.nameUpdated'))
  } catch (e) {
    nameError.value = e as ApiError
  } finally {
    nameSaving.value = false
  }
}

/* ---------------------------------------------------------------- password */

const passwordForm = reactive({ current: '', next: '', confirmation: '' })
const passwordSaving = ref(false)
const passwordError = ref<ApiError | null>(null)

const passwordRuleMessage = computed(() => {
  const issue = passwordIssue(passwordForm.next)
  if (!issue || issue === 'required') return null
  return t(`auth.password${issue.charAt(0).toUpperCase()}${issue.slice(1)}`)
})

const savePassword = async () => {
  passwordError.value = null

  const localErrors: Record<string, string[]> = {}
  if (!passwordForm.current) localErrors.current_password = [t('auth.passwordRequired')]
  if (!isStrongPassword(passwordForm.next)) {
    const issue = passwordIssue(passwordForm.next)!
    localErrors.password = [t(`auth.password${issue.charAt(0).toUpperCase()}${issue.slice(1)}`)]
  }
  if (passwordForm.confirmation !== passwordForm.next) {
    localErrors.password_confirmation = [t('auth.passwordMismatch')]
  }
  if (Object.keys(localErrors).length) {
    passwordError.value = { message: t('common.errorTitle'), errors: localErrors }
    return
  }

  passwordSaving.value = true
  try {
    await api.put('/admin/profile/password', {
      current_password: passwordForm.current,
      password: passwordForm.next,
      password_confirmation: passwordForm.confirmation,
    })
    passwordForm.current = ''
    passwordForm.next = ''
    passwordForm.confirmation = ''
    message.success(t('profile.passwordUpdated'))
  } catch (e) {
    passwordError.value = e as ApiError
  } finally {
    passwordSaving.value = false
  }
}

/* ---------------------------------------------------------------- email */

const emailForm = reactive({ email: '', currentPassword: '', code: '' })
const emailStep = ref<'request' | 'confirm'>('request')
const emailSaving = ref(false)
const emailError = ref<ApiError | null>(null)

const resetEmailForm = () => {
  emailForm.email = ''
  emailForm.currentPassword = ''
  emailForm.code = ''
  emailStep.value = 'request'
  emailError.value = null
  resend.stop()
}

/** Step 1: ask the API to mail a code to the new address. */
const requestEmailCode = async (isResend = false) => {
  emailError.value = null

  const localErrors: Record<string, string[]> = {}
  if (!isValidEmail(emailForm.email)) localErrors.email = [t('auth.emailInvalid')]
  if (!emailForm.currentPassword) localErrors.current_password = [t('auth.passwordRequired')]
  if (Object.keys(localErrors).length) {
    emailError.value = { message: t('common.errorTitle'), errors: localErrors }
    return
  }

  emailSaving.value = true
  try {
    await api.post('/admin/profile/email', {
      email: emailForm.email.trim(),
      current_password: emailForm.currentPassword,
    })
    emailStep.value = 'confirm'
    resend.start(OTP_RESEND_SECONDS)
    message.success(isResend ? t('auth.codeSent') : t('profile.emailCodeSent', { email: emailForm.email.trim() }))
  } catch (e) {
    const apiError = e as ApiError
    if (apiError.status === 429) resend.start(OTP_RESEND_SECONDS)
    emailError.value = apiError
  } finally {
    emailSaving.value = false
  }
}

/** Back to step 1, keeping the password already entered but dropping the code. */
const useDifferentAddress = () => {
  emailForm.code = ''
  emailStep.value = 'request'
  emailError.value = null
  resend.stop()
}

/** Step 2: confirm the code; the API swaps the address and returns the user. */
const confirmEmail = async () => {
  emailError.value = null
  if (!isValidOtp(emailForm.code)) {
    emailError.value = { message: t('common.errorTitle'), errors: { code: [t('auth.codeInvalid')] } }
    return
  }

  emailSaving.value = true
  try {
    const response = await api.post<{ data: User }>('/admin/profile/email/confirm', { code: emailForm.code })
    user.value = response.data
    message.success(t('profile.emailUpdated'))
    resetEmailForm()
  } catch (e) {
    emailError.value = e as ApiError
  } finally {
    emailSaving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader :title="$t('profile.title')" :subtitle="$t('profile.subtitle')" />

    <ApiErrorAlert :error="loadError" @retry="load" />

    <n-spin :show="loading">
      <div class="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <!-- Account summary + name -->
        <n-card class="rounded-xl" :title="$t('profile.account')">
          <n-descriptions :column="1" label-placement="left" class="mb-4">
            <n-descriptions-item :label="$t('auth.email')">{{ user?.email || '—' }}</n-descriptions-item>
            <n-descriptions-item :label="$t('users.role')">{{ roleLabel || '—' }}</n-descriptions-item>
            <n-descriptions-item :label="$t('users.lastLogin')">
              {{ formatDate(user?.last_login_at) }}
            </n-descriptions-item>
          </n-descriptions>

          <n-alert v-if="nameError && !nameError.errors" type="error" class="mb-3">{{ nameError.message }}</n-alert>

          <n-form label-placement="top" @keydown.enter.prevent="saveName">
            <n-form-item
              :label="$t('profile.name')"
              :validation-status="fieldError(nameError, 'name') ? 'error' : undefined"
              :feedback="fieldError(nameError, 'name') ?? undefined"
            >
              <n-input v-model:value="nameForm.name" autocomplete="name" />
            </n-form-item>
            <n-button type="primary" :loading="nameSaving" @click="saveName">{{ $t('common.save') }}</n-button>
          </n-form>
        </n-card>

        <!-- Password -->
        <n-card class="rounded-xl" :title="$t('profile.changePassword')">
          <n-alert v-if="passwordError && !passwordError.errors" type="error" class="mb-3">
            {{ passwordError.message }}
          </n-alert>

          <n-form label-placement="top">
            <n-form-item
              :label="$t('profile.currentPassword')"
              :validation-status="fieldError(passwordError, 'current_password') ? 'error' : undefined"
              :feedback="fieldError(passwordError, 'current_password') ?? undefined"
            >
              <n-input
                v-model:value="passwordForm.current"
                type="password"
                show-password-on="click"
                autocomplete="current-password"
              />
            </n-form-item>

            <n-form-item
              :label="$t('auth.newPassword')"
              :validation-status="fieldError(passwordError, 'password') ? 'error' : undefined"
              :feedback="fieldError(passwordError, 'password') ?? passwordRuleMessage ?? undefined"
            >
              <n-input
                v-model:value="passwordForm.next"
                type="password"
                show-password-on="click"
                autocomplete="new-password"
              />
            </n-form-item>

            <n-form-item
              :label="$t('auth.confirmPassword')"
              :validation-status="fieldError(passwordError, 'password_confirmation') ? 'error' : undefined"
              :feedback="fieldError(passwordError, 'password_confirmation') ?? undefined"
            >
              <n-input
                v-model:value="passwordForm.confirmation"
                type="password"
                show-password-on="click"
                autocomplete="new-password"
              />
            </n-form-item>

            <p class="-mt-2 mb-3 text-xs opacity-60">{{ $t('auth.passwordHint') }}</p>

            <n-button type="primary" :loading="passwordSaving" @click="savePassword">
              {{ $t('profile.changePassword') }}
            </n-button>
          </n-form>
        </n-card>

        <!-- Email -->
        <n-card class="rounded-xl xl:col-span-2" :title="$t('profile.changeEmail')">
          <n-alert v-if="emailError && !emailError.errors" type="error" class="mb-3">
            {{ emailError.message }}
          </n-alert>

          <n-form v-if="emailStep === 'request'" label-placement="top">
            <div class="grid grid-cols-1 gap-x-4 sm:grid-cols-2">
              <n-form-item
                :label="$t('profile.newEmail')"
                :validation-status="fieldError(emailError, 'email') ? 'error' : undefined"
                :feedback="fieldError(emailError, 'email') ?? undefined"
              >
                <n-input v-model:value="emailForm.email" inputmode="email" autocomplete="email" />
              </n-form-item>
              <n-form-item
                :label="$t('profile.currentPassword')"
                :validation-status="fieldError(emailError, 'current_password') ? 'error' : undefined"
                :feedback="fieldError(emailError, 'current_password') ?? undefined"
              >
                <n-input
                  v-model:value="emailForm.currentPassword"
                  type="password"
                  show-password-on="click"
                  autocomplete="current-password"
                />
              </n-form-item>
            </div>

            <n-button type="primary" :loading="emailSaving" @click="requestEmailCode(false)">
              {{ $t('auth.sendCode') }}
            </n-button>
          </n-form>

          <div v-else>
            <n-alert type="info" class="mb-4">{{ $t('profile.emailCodeHint', { email: emailForm.email }) }}</n-alert>

            <n-form label-placement="top">
              <n-form-item
                :label="$t('auth.code')"
                :validation-status="fieldError(emailError, 'code') ? 'error' : undefined"
                :feedback="fieldError(emailError, 'code') ?? undefined"
              >
                <OtpInput v-model:value="emailForm.code" :aria-label="$t('auth.code')" @finish="confirmEmail" />
              </n-form-item>
            </n-form>

            <div class="flex flex-wrap items-center gap-2">
              <n-button type="primary" :loading="emailSaving" @click="confirmEmail">
                {{ $t('profile.confirmEmail') }}
              </n-button>
              <n-button
                quaternary
                :disabled="resend.active.value || emailSaving"
                @click="requestEmailCode(true)"
              >
                {{
                  resend.active.value
                    ? $t('auth.resendIn', { seconds: resend.remaining.value })
                    : $t('auth.resendCode')
                }}
              </n-button>
              <n-button quaternary @click="resetEmailForm">{{ $t('common.cancel') }}</n-button>
            </div>

            <button type="button" class="mt-3 block text-sm text-teal-500 hover:underline" @click="useDifferentAddress">
              {{ $t('profile.useDifferentAddress') }}
            </button>
          </div>
        </n-card>
      </div>
    </n-spin>
  </div>
</template>
