<template>
    <div class="ann-page">
        <!-- 发布表单（contributor+） -->
        <div v-if="canPost" class="ann-composer">
            <t-input v-model="form.title" :placeholder="$t('announcements.titlePlaceholder')" size="large" />
            <textarea v-model="form.content" class="ann-composer__content"
                :placeholder="$t('announcements.contentPlaceholder')"></textarea>
            <div class="ann-composer__row">
                <t-button variant="text" size="small" @click="fileInputRef?.click()">
                    <template #icon><t-icon name="attach" /></template>
                    {{ $t('announcements.addFiles') }}
                </t-button>
                <input ref="fileInputRef" type="file" multiple style="display: none"
                    accept=".pdf,.doc,.docx,.ppt,.pptx,.xls,.xlsx,.zip,.rar,.7z,.txt,.md" @change="onFilesPicked" />
                <span class="ann-composer__files">
                    <t-tag v-for="(f, i) in form.files" :key="i" theme="default" size="small" closable
                        @close="form.files.splice(i, 1)">
                        {{ f.name }}
                    </t-tag>
                </span>
                <span class="ann-composer__spacer"></span>
                <t-button theme="primary" size="small" :loading="posting" @click="handlePost">
                    {{ $t('announcements.publish') }}
                </t-button>
            </div>
        </div>

        <!-- 公告流 -->
        <div v-if="loading" class="ann-empty">{{ $t('announcements.loading') }}</div>
        <div v-else-if="items.length === 0" class="ann-empty">{{ $t('announcements.empty') }}</div>
        <div v-for="item in items" :key="item.id" class="ann-card">
            <div class="ann-card__head">
                <span class="ann-card__title">{{ item.title }}</span>
                <t-popconfirm v-if="canDelete(item)" :content="$t('announcements.deleteConfirm')"
                    :confirm-btn="{ content: $t('common.delete'), theme: 'danger' }"
                    :cancel-btn="$t('common.cancel')" placement="left" @confirm="handleDelete(item.id)">
                    <t-button theme="danger" variant="text" shape="square" size="small">
                        <template #icon><t-icon name="delete" size="14px" /></template>
                    </t-button>
                </t-popconfirm>
            </div>
            <div class="ann-card__meta">{{ item.author_name || $t('announcements.unknownAuthor') }} ·
                {{ formatDate(item.created_at) }}</div>
            <div class="ann-card__body markdown-body" v-html="renderMarkdown(item)"></div>

            <!-- 附件 -->
            <div v-if="item.attachments.length" class="ann-card__files">
                <div v-for="(att, idx) in item.attachments" :key="idx" class="ann-file" @click="downloadFile(item.id, idx, att.name)">
                    <t-icon name="file" size="14px" />
                    <span class="ann-file__name">{{ att.name }}</span>
                    <span class="ann-file__size">{{ formatSize(att.size) }}</span>
                    <t-icon name="download" size="14px" />
                </div>
            </div>

            <!-- 留言 -->
            <div class="ann-card__comments">
                <t-button variant="text" size="small" @click="toggleComments(item.id)">
                    <template #icon><t-icon name="chat" /></template>
                    {{ $t('announcements.comments') }}
                </t-button>
                <template v-if="openComments[item.id]">
                    <div v-if="commentsLoading[item.id]" class="ann-comment-hint">{{ $t('announcements.loading') }}</div>
                    <div v-for="c in comments[item.id] || []" :key="c.id" class="ann-comment">
                        <div class="ann-comment__meta">
                            <span class="ann-comment__author">{{ c.author_name || c.user_id }}</span>
                            <span>{{ formatDate(c.created_at) }}</span>
                            <t-button v-if="canDeleteComment(c)" theme="danger" variant="text" size="small"
                                @click="handleDeleteComment(item.id, c.id)">
                                {{ $t('common.delete') }}
                            </t-button>
                        </div>
                        <div class="ann-comment__content">{{ c.content }}</div>
                    </div>
                    <div v-if="!commentsLoading[item.id] && !(comments[item.id] || []).length" class="ann-comment-hint">
                        {{ $t('announcements.noComments') }}
                    </div>
                    <div class="ann-comment-input">
                        <t-input v-model="commentDraft[item.id]" :placeholder="$t('announcements.commentPlaceholder')"
                            size="small" @enter="handleComment(item.id)" />
                        <t-button theme="primary" variant="text" size="small"
                            :loading="commentPosting[item.id]" @click="handleComment(item.id)">
                            {{ $t('announcements.send') }}
                        </t-button>
                    </div>
                </template>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { marked } from 'marked'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import {
    createAnnouncement, createComment, deleteAnnouncement, deleteComment,
    downloadAnnouncementAttachment, getAnnouncement, listAnnouncements, listComments,
    type AnnouncementCommentItem, type AnnouncementListItem,
} from '@/api/me/announcements'
import { safeMarkdownToHTML, sanitizeHTML } from '@/utils/security'

const { t } = useI18n()
const authStore = useAuthStore()

const items = ref<AnnouncementListItem[]>([])
const loading = ref(false)
const posting = ref(false)
const form = reactive({ title: '', content: '', files: [] as File[] })
const fileInputRef = ref<HTMLInputElement | null>(null)

const openComments = reactive<Record<string, boolean>>({})
const comments = reactive<Record<string, AnnouncementCommentItem[]>>({})
const commentsLoading = reactive<Record<string, boolean>>({})
const commentDraft = reactive<Record<string, string>>({})
const commentPosting = reactive<Record<string, boolean>>({})

const canPost = computed(() => authStore.canAccessAllTenants || authStore.hasRole('contributor'))

function renderMarkdown(item: { content?: string }): string {
    return sanitizeHTML(marked.parse(safeMarkdownToHTML(item.content || ''), { async: false }) as string)
}

function formatDate(value: string): string {
    const d = new Date(value)
    if (Number.isNaN(d.getTime())) return ''
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function formatSize(bytes: number): string {
    if (bytes >= 1 << 20) return (bytes / (1 << 20)).toFixed(1) + ' MB'
    if (bytes >= 1 << 10) return (bytes / (1 << 10)).toFixed(0) + ' KB'
    return bytes + ' B'
}

// 删除权限与后端同语义：作者或 admin（前端只控制按钮可见性）
function canDelete(item: { author_name?: string }): boolean {
    if (authStore.canAccessAllTenants || authStore.hasRole('admin') || authStore.hasRole('owner')) return true
    // 后端按 user_id 判作者；卡片上没有 user_id，作者本人删除走确认失败兜底
    return item.author_name === authStore.user?.username
}

function canDeleteComment(c: { user_id: string }): boolean {
    return c.user_id === authStore.user?.id ||
        authStore.canAccessAllTenants || authStore.hasRole('admin') || authStore.hasRole('owner')
}

async function load() {
    loading.value = true
    try {
        const resp = await listAnnouncements()
        if (resp.success) items.value = resp.data?.announcements ?? []
    } finally {
        loading.value = false
    }
}

async function handlePost() {
    if (!form.title.trim()) {
        MessagePlugin.warning(t('announcements.titleRequired'))
        return
    }
    posting.value = true
    try {
        const resp = await createAnnouncement(form.title.trim(), form.content, form.files)
        if (!resp || !resp.success || !resp.data) {
            const msg = (resp && resp.message) || t('announcements.postFailed')
            MessagePlugin.error(msg)
            console.error('[announcements] post failed:', resp, msg)
            return
        }
        MessagePlugin.success(t('announcements.posted'))
        form.title = ''
        form.content = ''
        form.files = []
        await load()
    } catch (err) {
        // surface network/runtime errors that the backend response shape
        // can't capture (axios network error, FileService unreachable, ...)
        console.error('[announcements] post threw:', err)
        MessagePlugin.error(t('announcements.postFailed'))
    } finally {
        posting.value = false
    }
}

function onFilesPicked(e: Event) {
    const input = e.target as HTMLInputElement
    const picked = Array.from(input.files ?? [])
    for (const f of picked) {
        if (f.size > 50 << 20) {
            MessagePlugin.warning(t('announcements.fileTooLarge', { name: f.name }))
            continue
        }
        if (form.files.length >= 5) {
            MessagePlugin.warning(t('announcements.tooManyFiles'))
            break
        }
        form.files.push(f)
    }
    input.value = ''
}

async function handleDelete(id: string) {
    const resp = await deleteAnnouncement(id)
    if (!resp.success) {
        MessagePlugin.error(resp.message || t('announcements.deleteFailed'))
        return
    }
    items.value = items.value.filter(a => a.id !== id)
}

async function downloadFile(id: string, index: number, name: string) {
    try {
        const blob = await (await import('@/api/me/announcements')).downloadAnnouncementAttachment(id, index)
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = name
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        setTimeout(() => URL.revokeObjectURL(url), 1000)
    } catch {
        MessagePlugin.error(t('announcements.downloadFailed'))
    }
}

async function toggleComments(id: string) {
    if (openComments[id]) {
        openComments[id] = false
        return
    }
    openComments[id] = true
    if (!comments[id]) {
        commentsLoading[id] = true
        try {
            const resp = await listComments(id)
            comments[id] = resp.success ? resp.data?.comments ?? [] : []
        } finally {
            commentsLoading[id] = false
        }
    }
}

async function handleComment(id: string) {
    const text = (commentDraft[id] || '').trim()
    if (!text || commentPosting[id]) return
    commentPosting[id] = true
    try {
        const resp = await createComment(id, text)
        if (!resp.success || !resp.data) {
            MessagePlugin.error(resp.message || t('announcements.commentFailed'))
            return
        }
        comments[id] = [...(comments[id] || []), resp.data]
        commentDraft[id] = ''
    } finally {
        commentPosting[id] = false
    }
}

async function handleDeleteComment(announcementId: string, commentId: string) {
    const resp = await deleteComment(announcementId, commentId)
    if (!resp.success) {
        MessagePlugin.error(resp.message || t('announcements.commentDeleteFailed'))
        return
    }
    comments[announcementId] = (comments[announcementId] || []).filter(c => c.id !== commentId)
}

// 展开留言时若公告详情未拉过，先取详情（留言端点要求公告存在）
async function ensureDetail(id: string) {
    if (openComments[id]) return
    await getAnnouncement(id).catch(() => null)
}

onMounted(() => { load() })
</script>

<style lang="less" scoped>
.ann-page {
    width: 100%;
    max-width: 860px;
    margin: 0 auto;
    padding: 20px 24px 40px;
    box-sizing: border-box;
    overflow-y: auto;
    height: 100%;
}

.ann-composer {
    border: 1px solid var(--td-component-stroke);
    border-radius: 12px;
    padding: 14px;
    margin-bottom: 20px;
    display: flex;
    flex-direction: column;
    gap: 10px;

    &__content {
        min-height: 72px;
        border: 1px solid var(--td-component-stroke);
        border-radius: 8px;
        padding: 10px 12px;
        resize: vertical;
        font-size: 13px;
        line-height: 1.6;
        background: var(--td-bg-color-container);
        color: var(--td-text-color-primary);
        outline: none;

        &:focus {
            border-color: var(--td-brand-color);
        }
    }

    &__row {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    &__files {
        display: flex;
        gap: 6px;
        flex-wrap: wrap;
        flex: 1;
    }

    &__spacer {
        flex: 0 1 auto;
    }
}

.ann-empty {
    padding: 48px 0;
    text-align: center;
    color: var(--td-text-color-placeholder);
    font-size: 13px;
}

.ann-card {
    border: 1px solid var(--td-component-stroke);
    border-radius: 12px;
    padding: 16px 18px;
    margin-bottom: 16px;

    &__head {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 8px;
    }

    &__title {
        font-size: 16px;
        font-weight: 600;
        color: var(--td-text-color-primary);
    }

    &__meta {
        font-size: 12px;
        color: var(--td-text-color-placeholder);
        margin: 4px 0 10px;
    }

    &__body {
        font-size: 13.5px;
        line-height: 1.7;
        color: var(--td-text-color-primary);
    }

    &__files {
        margin-top: 10px;
        display: flex;
        flex-direction: column;
        gap: 4px;
    }

    &__comments {
        margin-top: 12px;
        padding-top: 10px;
        border-top: 1px dashed var(--td-component-stroke);
        display: flex;
        flex-direction: column;
        gap: 8px;
    }
}

.ann-file {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 10px;
    border: 1px solid var(--td-component-stroke);
    border-radius: 8px;
    cursor: pointer;
    font-size: 12px;
    color: var(--td-text-color-primary);

    &:hover {
        background: var(--td-bg-color-container-hover);
    }

    &__name {
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    &__size {
        color: var(--td-text-color-placeholder);
    }
}

.ann-comment {
    &__meta {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 11px;
        color: var(--td-text-color-placeholder);
    }

    &__author {
        font-weight: 600;
        color: var(--td-text-color-primary);
    }

    &__content {
        font-size: 12.5px;
        color: var(--td-text-color-primary);
        white-space: pre-wrap;
        margin-top: 2px;
    }
}

.ann-comment-hint {
    font-size: 11px;
    color: var(--td-text-color-placeholder);
}

.ann-comment-input {
    display: flex;
    gap: 6px;
    align-items: center;
}
</style>
