<template>
  <div class="user-directory">
    <header class="section-header">
      <div>
        <h2>{{ t('userDirectory.title') }}</h2>
        <p class="section-description">{{ t('userDirectory.description') }}</p>
      </div>
      <button
        type="button"
        class="refresh-btn"
        :disabled="loading"
        :aria-label="t('userDirectory.refresh')"
        @click="load"
      >
        <t-icon :name="loading ? 'loading' : 'refresh'" :class="{ spin: loading }" />
      </button>
    </header>

    <div class="toolbar">
      <t-input
        v-model="keyword"
        clearable
        :placeholder="t('userDirectory.searchPlaceholder')"
      >
        <template #prefix-icon><t-icon name="search" /></template>
      </t-input>
    </div>

    <t-alert v-if="error" theme="error" :message="error" class="page-alert">
      <template #operation>
        <t-button size="small" @click="load">{{ t('userDirectory.retry') }}</t-button>
      </template>
    </t-alert>

    <t-table
      v-else
      row-key="id"
      :data="users"
      :columns="columns"
      :loading="loading"
      size="medium"
      hover
    >
      <template #username="{ row }">
        <span>{{ row.username || '—' }}</span>
        <span v-if="row.id === currentUserId" class="self-mark">{{ t('userDirectory.self') }}</span>
      </template>
      <template #identity="{ row }">
        <t-select
          :model-value="identityOf(row)"
          :options="identityOptions"
          :disabled="row.id === currentUserId || savingId === row.id"
          :loading="savingId === row.id"
          size="small"
          @change="(value) => onIdentityChange(row, String(value))"
        />
      </template>
      <template #created_at="{ row }">
        {{ formatCreated(row.created_at) }}
      </template>
      <template #empty>
        <t-empty :description="t('userDirectory.empty')" />
      </template>
    </t-table>

    <div v-if="total > pageSize" class="pager">
      <t-pagination
        :current="page"
        :page-size="pageSize"
        :total="total"
        :show-page-size="false"
        @current-change="onPage"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import {
  appointTeacher,
  listRegisteredUsers,
  promoteUserToSystemAdmin,
  revokeSystemAdmin,
  revokeTeacher,
  type DirectoryUser,
} from '@/api/system'
import { useAuthStore } from '@/stores/auth'

type Identity = 'student' | 'teacher' | 'superadmin'

const { t, locale } = useI18n()
const authStore = useAuthStore()
const currentUserId = computed(() => authStore.currentUserId)

const users = ref<DirectoryUser[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const keyword = ref('')
const loading = ref(false)
const error = ref('')
const savingId = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null

const identityOptions = computed(() => [
  { label: t('userDirectory.identity.student'), value: 'student' },
  { label: t('userDirectory.identity.teacher'), value: 'teacher' },
  { label: t('userDirectory.identity.superadmin'), value: 'superadmin' },
])

const columns = computed(() => [
  { colKey: 'username', title: t('userDirectory.columns.username'), width: 180 },
  { colKey: 'email', title: t('userDirectory.columns.email'), minWidth: 220 },
  { colKey: 'identity', title: t('userDirectory.columns.identity'), width: 180 },
  { colKey: 'created_at', title: t('userDirectory.columns.createdAt'), width: 180 },
])

function identityOf(user: DirectoryUser): Identity {
  if (user.platform_identity === 'superadmin' || user.is_system_admin) return 'superadmin'
  if (user.platform_identity === 'teacher' || user.is_teacher) return 'teacher'
  return 'student'
}

function formatCreated(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale.value || 'zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function friendlyError(err: unknown): string {
  const raw = err instanceof Error ? err.message : ''
  if (raw.includes('Cannot revoke your own')) return t('userDirectory.errors.self')
  if (raw.includes('last remaining')) return t('userDirectory.errors.lastAdmin')
  if (raw.includes('User not found')) return t('userDirectory.errors.notFound')
  return raw || t('userDirectory.saveFailed')
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const resp = await listRegisteredUsers({
      offset: (page.value - 1) * pageSize,
      limit: pageSize,
      q: keyword.value.trim(),
    })
    users.value = resp.users ?? []
    total.value = resp.total ?? 0
  } catch (err: unknown) {
    error.value = friendlyError(err)
    users.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function onPage(next: number) {
  page.value = next
  void load()
}

watch(keyword, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    page.value = 1
    void load()
  }, 250)
})

function confirmChange(email: string, next: Identity): Promise<boolean> {
  return new Promise((resolve) => {
    let settled = false
    const done = (ok: boolean) => {
      if (settled) return
      settled = true
      dialog.hide()
      resolve(ok)
    }
    const dialog = DialogPlugin.confirm({
      header: t('userDirectory.confirm.header'),
      body: t('userDirectory.confirm.body', {
        email,
        identity: t(`userDirectory.identity.${next}`),
      }),
      confirmBtn: t('userDirectory.confirm.confirmBtn'),
      cancelBtn: t('userDirectory.confirm.cancelBtn'),
      onConfirm: () => done(true),
      onClose: () => done(false),
    })
  })
}

async function applyIdentity(user: DirectoryUser, next: Identity) {
  const current = identityOf(user)
  if (next === 'superadmin') {
    await promoteUserToSystemAdmin({ user_id: user.id })
    return
  }
  if (next === 'teacher' && current === 'superadmin') {
    await appointTeacher({ user_id: user.id })
    await revokeSystemAdmin(user.id)
    return
  }
  if (current === 'superadmin') {
    await revokeSystemAdmin(user.id)
  }
  if (next === 'teacher') {
    await appointTeacher({ user_id: user.id })
    return
  }
  if (user.is_teacher || current === 'teacher') {
    await revokeTeacher(user.id)
  }
}

async function onIdentityChange(user: DirectoryUser, next: string) {
  if (next !== 'student' && next !== 'teacher' && next !== 'superadmin') return
  if (identityOf(user) === next || savingId.value) return
  const ok = await confirmChange(user.email, next)
  if (!ok) return
  savingId.value = user.id
  try {
    await applyIdentity(user, next)
    MessagePlugin.success(t('userDirectory.saveSuccess'))
    await load()
  } catch (err: unknown) {
    MessagePlugin.error(friendlyError(err))
    await load()
  } finally {
    savingId.value = ''
  }
}

onMounted(() => {
  void load()
})

onUnmounted(() => {
  if (searchTimer) clearTimeout(searchTimer)
})
</script>

<style lang="less" scoped>
.user-directory {
  width: 100%;
}

.section-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;

  h2 {
    margin: 0 0 6px;
    font-size: 20px;
    font-weight: 600;
  }
}

.section-description {
  margin: 0;
  color: var(--td-text-color-secondary);
  line-height: 1.5;
}

.refresh-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  background: transparent;
  color: inherit;
  cursor: pointer;
}

.spin {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.toolbar {
  max-width: 360px;
  margin-bottom: 16px;
}

.page-alert {
  margin-bottom: 16px;
}

.self-mark {
  margin-left: 8px;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
