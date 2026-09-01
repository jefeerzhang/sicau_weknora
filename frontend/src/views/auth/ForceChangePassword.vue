<template>
  <main class="force-change-password">
    <section class="force-change-card">
      <div class="force-change-mark" aria-hidden="true">
        <t-icon name="lock-on" size="30px" />
      </div>
      <h1>{{ $t('auth.forceChangePassword.title') }}</h1>
      <p class="force-change-description">{{ $t('auth.forceChangePassword.description') }}</p>

      <t-form
        ref="formRef"
        :data="form"
        :rules="rules"
        label-align="top"
        class="force-change-form"
        @submit.prevent
      >
        <t-form-item :label="$t('userProfile.changePassword.currentLabel')" name="oldPassword">
          <t-input
            v-model="form.oldPassword"
            type="password"
            autocomplete="current-password"
            :disabled="submitting"
            :placeholder="$t('userProfile.changePassword.currentPlaceholder')"
          />
        </t-form-item>
        <t-form-item :label="$t('userProfile.changePassword.newLabel')" name="newPassword">
          <t-input
            v-model="form.newPassword"
            type="password"
            autocomplete="new-password"
            :disabled="submitting"
            :placeholder="$t('userProfile.changePassword.newPlaceholder')"
          />
        </t-form-item>
        <t-form-item :label="$t('userProfile.changePassword.confirmLabel')" name="confirmPassword">
          <t-input
            v-model="form.confirmPassword"
            type="password"
            autocomplete="new-password"
            :disabled="submitting"
            :placeholder="$t('userProfile.changePassword.confirmPlaceholder')"
            @enter="submit"
          />
        </t-form-item>
        <t-button theme="primary" block size="large" :loading="submitting" @click="submit">
          {{ $t('auth.forceChangePassword.submit') }}
        </t-button>
      </t-form>

      <button class="logout-link" type="button" :disabled="submitting" @click="handleLogout">
        {{ $t('auth.logout') }}
      </button>
    </section>
  </main>
</template>

<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule } from 'tdesign-vue-next'
import { changePassword, logout as logoutApi } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const formRef = ref<FormInstanceFunctions | null>(null)
const submitting = ref(false)
const form = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const rules = computed<Record<string, FormRule[]>>(() => ({
  oldPassword: [
    { required: true, message: t('userProfile.changePassword.currentRequired'), type: 'error' },
  ],
  newPassword: [
    { required: true, message: t('auth.passwordRequired'), type: 'error' },
    { min: 8, message: t('auth.passwordMinLength'), type: 'error' },
    { max: 32, message: t('auth.passwordMaxLength'), type: 'error' },
    { pattern: /[a-zA-Z]/, message: t('auth.passwordMustContainLetter'), type: 'error' },
    { pattern: /\d/, message: t('auth.passwordMustContainNumber'), type: 'error' },
    {
      validator: (val: string) => val !== form.oldPassword,
      message: t('userProfile.changePassword.sameAsCurrent'),
      type: 'error',
    },
  ],
  confirmPassword: [
    { required: true, message: t('auth.confirmPasswordRequired'), type: 'error' },
    {
      validator: (val: string) => val === form.newPassword,
      message: t('auth.passwordMismatch'),
      type: 'error',
      trigger: 'blur',
    },
  ],
}))

async function handleLogout() {
  if (submitting.value) return
  try {
    await logoutApi()
  } catch {
    /* ignore */
  }
  authStore.logout()
  router.push('/login')
}

async function submit() {
  if (submitting.value) return
  const result = await formRef.value?.validate?.()
  if (result !== true) return

  submitting.value = true
  try {
    const resp = await changePassword({
      old_password: form.oldPassword,
      new_password: form.newPassword,
    })
    if (!resp.success) {
      MessagePlugin.error(resp.message || t('userProfile.changePassword.failed'))
      return
    }

    MessagePlugin.success(t('auth.forceChangePassword.success'))

    const refreshed = await authStore.refreshFromAuthMe()
    if (refreshed && !authStore.mustChangePassword) {
      router.push(
        authStore.hasValidTenant ? '/platform/knowledge-bases' : '/onboarding/workspace',
      )
      return
    }

    // Backend revokes all sessions on success; fall back to a fresh login.
    try {
      await logoutApi()
    } catch {
      /* ignore */
    }
    authStore.logout()
    router.push('/login')
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('userProfile.changePassword.failed'))
  } finally {
    submitting.value = false
  }
}
</script>

<style lang="less" scoped>
.force-change-password {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--td-bg-color-page);
}

.force-change-card {
  width: 100%;
  max-width: 420px;
  padding: 40px 32px;
  border-radius: 16px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.06);
}

.force-change-mark {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  margin: 0 auto 20px;
  border-radius: 14px;
  background: var(--td-brand-color-light);
  color: var(--td-brand-color);
}

h1 {
  margin: 0 0 8px;
  font-size: 22px;
  font-weight: 600;
  text-align: center;
  color: var(--td-text-color-primary);
}

.force-change-description {
  margin: 0 0 24px;
  font-size: 14px;
  line-height: 1.6;
  text-align: center;
  color: var(--td-text-color-secondary);
}

.force-change-form {
  :deep(.t-form__item) {
    margin-bottom: 16px;
  }
}

.logout-link {
  display: block;
  width: 100%;
  margin-top: 20px;
  padding: 0;
  border: none;
  background: none;
  font-size: 13px;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  text-align: center;

  &:hover:not(:disabled) {
    color: var(--td-brand-color);
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
}
</style>
