<template>
  <div class="my-personal-models">
    <div class="section-header">
      <h2>{{ t('personalModels.myTitle') }}</h2>
      <p class="section-description">{{ t('personalModels.myDescription') }}</p>
    </div>

    <div v-if="!workspaceEnabled" class="notice">
      <t-icon name="info-circle" />
      <span>{{ t('personalModels.workspaceDisabled') }}</span>
    </div>

    <template v-else>
      <div class="toolbar">
        <t-button theme="primary" size="small" @click="openCreate">
          <template #icon><t-icon name="add" /></template>
          {{ t('personalModels.add') }}
        </t-button>
      </div>

      <t-loading :loading="loading" size="small">
        <t-empty v-if="!loading && models.length === 0" :description="t('personalModels.empty')" />
        <div v-else class="model-list">
          <div v-for="m in models" :key="m.id" class="model-row">
            <div class="model-row__body">
              <div class="model-row__title">{{ m.name || m.model_name }}</div>
              <div class="model-row__meta">{{ m.model_name }} · {{ m.base_url }}</div>
            </div>
            <div class="model-row__actions">
              <t-switch
                :value="m.enabled"
                size="small"
                :loading="togglingId === m.id"
                @change="(val: boolean) => onToggleEnabled(m, val)"
              />
              <t-button theme="default" variant="text" size="small" @click="openEdit(m)">
                {{ t('personalModels.edit') }}
              </t-button>
              <t-button theme="danger" variant="text" size="small" @click="onDelete(m.id)">
                {{ t('common.delete') }}
              </t-button>
            </div>
          </div>
        </div>
      </t-loading>
    </template>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="editingId ? t('personalModels.edit') : t('personalModels.add')"
      :confirm-btn="{ content: t('common.save'), loading: saving }"
      @confirm="onSave"
    >
      <t-form label-align="top">
        <t-form-item :label="t('personalModels.fields.modelName')">
          <t-input v-model="form.model_name" />
        </t-form-item>
        <t-form-item :label="t('personalModels.fields.baseUrl')">
          <t-input v-model="form.base_url" placeholder="https://api.siliconflow.cn/v1" />
        </t-form-item>
        <t-form-item :label="t('personalModels.fields.apiKey')">
          <t-input
            v-model="form.api_key"
            type="password"
            :placeholder="editingId ? t('personalModels.fields.apiKeyKeep') : ''"
          />
        </t-form-item>
        <t-form-item :label="t('personalModels.fields.displayName')">
          <t-input v-model="form.name" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  createPersonalModel,
  deletePersonalModel,
  getStudentPersonalModelsConfig,
  listPersonalModels,
  updatePersonalModel,
  type PersonalModelItem,
} from '@/api/personal-models'

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editingId = ref<string | null>(null)
const togglingId = ref<string | null>(null)
const workspaceEnabled = ref(false)
const models = ref<PersonalModelItem[]>([])
const form = reactive({
  name: '',
  model_name: '',
  base_url: '',
  api_key: '',
})

function resetForm() {
  form.name = ''
  form.model_name = ''
  form.base_url = ''
  form.api_key = ''
}

function openCreate() {
  editingId.value = null
  resetForm()
  dialogVisible.value = true
}

function openEdit(m: PersonalModelItem) {
  editingId.value = m.id
  form.name = m.name || ''
  form.model_name = m.model_name
  form.base_url = m.base_url
  form.api_key = ''
  dialogVisible.value = true
}

async function refresh() {
  loading.value = true
  try {
    const cfg = await getStudentPersonalModelsConfig()
    workspaceEnabled.value = !!cfg?.data?.enabled
    if (!workspaceEnabled.value) {
      models.value = []
      return
    }
    const res = await listPersonalModels()
    models.value = res?.data?.models || []
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('personalModels.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function onSave() {
  if (!form.model_name.trim() || !form.base_url.trim()) {
    MessagePlugin.warning(t('personalModels.requiredFieldsEdit'))
    return false
  }
  if (!editingId.value && !form.api_key.trim()) {
    MessagePlugin.warning(t('personalModels.requiredFields'))
    return false
  }
  saving.value = true
  try {
    if (editingId.value) {
      const body: Parameters<typeof updatePersonalModel>[1] = {
        name: form.name.trim(),
        model_name: form.model_name.trim(),
        base_url: form.base_url.trim(),
      }
      if (form.api_key.trim()) {
        body.api_key = form.api_key.trim()
      }
      await updatePersonalModel(editingId.value, body)
    } else {
      await createPersonalModel({
        name: form.name.trim(),
        model_name: form.model_name.trim(),
        base_url: form.base_url.trim(),
        api_key: form.api_key.trim(),
      })
    }
    dialogVisible.value = false
    editingId.value = null
    resetForm()
    MessagePlugin.success(t('personalModels.saved'))
    await refresh()
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('personalModels.saveFailed'))
    return false
  } finally {
    saving.value = false
  }
}

async function onToggleEnabled(m: PersonalModelItem, enabled: boolean) {
  togglingId.value = m.id
  const prev = m.enabled
  m.enabled = enabled
  try {
    await updatePersonalModel(m.id, { enabled })
    MessagePlugin.success(t('personalModels.saved'))
  } catch (e: any) {
    m.enabled = prev
    MessagePlugin.error(e?.message || t('personalModels.saveFailed'))
  } finally {
    togglingId.value = null
  }
}

async function onDelete(id: string) {
  try {
    await deletePersonalModel(id)
    MessagePlugin.success(t('personalModels.deleted'))
    await refresh()
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('personalModels.deleteFailed'))
  }
}

onMounted(refresh)
</script>

<style scoped>
.section-header h2 {
  margin: 0 0 4px;
  font-size: 18px;
}
.section-description {
  margin: 0 0 16px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}
.notice {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 12px;
  border-radius: 8px;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
}
.toolbar {
  margin-bottom: 12px;
}
.model-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.model-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--td-border-level-1-color);
  border-radius: 8px;
}
.model-row__title {
  font-weight: 600;
}
.model-row__meta {
  font-size: 12px;
  color: var(--td-text-color-secondary);
  word-break: break-all;
}
.model-row__actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
</style>
