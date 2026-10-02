import { get, put, post, del } from '@/utils/request'

export interface StudentPersonalModelsConfig {
  enabled: boolean
  allowed_hosts?: string[]
}

export interface PersonalModelItem {
  id: string
  user_id?: string
  name: string
  model_name: string
  base_url: string
  provider: string
  enabled: boolean
  has_credential: boolean
  created_at: string
  updated_at: string
}

export async function getStudentPersonalModelsConfig() {
  return get<{ success: boolean; data: StudentPersonalModelsConfig }>(
    '/api/v1/tenants/kv/student-personal-models',
  )
}

export async function updateStudentPersonalModelsConfig(config: StudentPersonalModelsConfig) {
  return put<{ success: boolean; data: StudentPersonalModelsConfig }>(
    '/api/v1/tenants/kv/student-personal-models',
    config,
  )
}

export async function listPersonalModels() {
  return get<{ success: boolean; data: { models: PersonalModelItem[] } }>(
    '/api/v1/me/personal-models',
  )
}

export async function createPersonalModel(body: {
  name?: string
  model_name: string
  base_url: string
  provider?: string
  api_key: string
  enabled?: boolean
}) {
  return post<{ success: boolean; data: PersonalModelItem }>('/api/v1/me/personal-models', body)
}

export async function deletePersonalModel(id: string) {
  return del<{ success: boolean }>(`/api/v1/me/personal-models/${id}`)
}

export async function listTenantPersonalModelMeta() {
  return get<{ success: boolean; data: { models: PersonalModelItem[] } }>(
    '/api/v1/tenants/personal-models',
  )
}
