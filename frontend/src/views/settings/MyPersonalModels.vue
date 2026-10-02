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
        <t-button theme="primary" size="small" @click="showAdd = true">
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
            <t-button theme="danger" variant="text" size="small" @click="onDelete(m.id)">
              {{ t('common.delete') }}
            </t-button>
          </div>
        </div>
      </t-loading>
    </template>

    <t-dialog
      v-model:visible="showAdd"
      :header="t('personalModels.add')"
      :confirm-btn="{ content: t('common.save'), loading: saving }"
      @confirm="onCreate"
    >
      <t-form label-align="top">
        <t-form-item :label="t('personalModels.fields.modelName')">
          <t-input v-model="form.model_name" />
        </t-form-item>
        <t-form-item :label="t('personalModels.fields.baseUrl')">
          <t-input v-model="form.base_url" placeholder="https://api.siliconflow.cn/v1" />
        </t-form-item>
        <t-form-item :label="t('personalModels.fields.apiKey')">
          <t-input v-model="form.api_key" type="password" />
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
  type PersonalModelItem,
} from '@/api/personal-models'

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)
const showAdd = ref(false)
const workspaceEnabled = ref(false)
const models = ref<PersonalModelItem[]>([])
const form = reactive({
  name: '',
  model_name: '',
  base_url: '',
  api_key: '',
})

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

async function onCreate() {
  if (!form.model_name.trim() || !form.base_url.trim() || !form.api_key.trim()) {
    MessagePlugin.warning(t('personalModels.requiredFields'))
    return false
  }
  saving.value = true
  try {
    await createPersonalModel({
      name: form.name.trim(),
      model_name: form.model_name.trim(),
      base_url: form.base_url.trim(),
      api_key: form.api_key.trim(),
    })
    showAdd.value = false
    form.name = ''
    form.model_name = ''
    form.base_url = ''
    form.api_key = ''
    MessagePlugin.success(t('personalModels.saved'))
    await refresh()
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('personalModels.saveFailed'))
    return false
  } finally {
    saving.value = false
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
</style>
