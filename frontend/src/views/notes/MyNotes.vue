<template>
    <div class="notes-page">
        <!-- 左：笔记列表 -->
        <aside class="notes-list-pane">
            <div class="notes-list-header">
                <span class="notes-list-title">{{ $t('notes.title') }}</span>
                <t-button theme="primary" variant="text" size="small" :loading="creating"
                    @click="handleCreate">
                    <template #icon><t-icon name="add" /></template>
                    {{ $t('notes.newNote') }}
                </t-button>
            </div>
            <div class="notes-list-body">
                <div v-if="listLoading" class="notes-empty-hint">{{ $t('notes.loading') }}</div>
                <div v-else-if="notes.length === 0" class="notes-empty-hint">
                    {{ $t('notes.empty') }}
                </div>
                <div v-for="item in notes" :key="item.id" class="note-list-item" :class="{
                    'is-active': item.id === currentId,
                    'is-dirty': item.id === currentId && dirty
                }" @click="switchTo(item.id)">
                    <div class="note-item-title">{{ item.title || $t('notes.untitled') }}<span v-if="item.id === currentId && dirty"
                            class="note-dirty-dot">●</span></div>
                    <div class="note-item-meta">
                        <span>{{ formatDate(item.updated_at) }}</span>
                        <t-popconfirm :content="$t('notes.deleteConfirm')"
                            :confirm-btn="{ content: $t('notes.delete'), theme: 'danger' }"
                            :cancel-btn="$t('common.cancel')" placement="bottom-right"
                            @confirm="handleDelete(item.id)">
                            <t-button theme="danger" variant="text" shape="square" size="small"
                                @click.stop>
                                <template #icon><t-icon name="delete" size="13px" /></template>
                            </t-button>
                        </t-popconfirm>
                    </div>
                </div>
            </div>
        </aside>

        <!-- 右：编辑/预览 -->
        <section class="notes-editor-pane">
            <template v-if="currentId">
                <div class="notes-editor-bar">
                    <t-button variant="text" size="small" :class="{ 'is-active': !previewMode }"
                        @click="previewMode = false">
                        <template #icon><t-icon name="edit" /></template>
                        {{ $t('notes.edit') }}
                    </t-button>
                    <t-button variant="text" size="small" :class="{ 'is-active': previewMode }"
                        @click="previewMode = true">
                        <template #icon><t-icon name="browse" /></template>
                        {{ $t('notes.preview') }}
                    </t-button>
                    <span class="notes-editor-spacer"></span>
                    <span v-if="dirty" class="notes-unsaved">{{ $t('notes.unsaved') }}</span>
                    <span v-else-if="justSaved" class="notes-saved">{{ $t('notes.saved') }}</span>
                    <t-button theme="primary" size="small" :loading="saving" @click="saveCurrent(true)">
                        {{ $t('notes.save') }}
                    </t-button>
                </div>
                <textarea v-if="!previewMode" ref="editorRef" v-model="content" class="notes-editor"
                    :placeholder="$t('notes.placeholder')" @keydown="onEditorKeydown"></textarea>
                <div v-else class="notes-preview markdown-body" v-html="previewHTML"></div>
            </template>
            <div v-else class="notes-empty-hint notes-editor-empty">
                {{ notes.length === 0 ? $t('notes.empty') : $t('notes.selectHint') }}
            </div>
        </section>
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, nextTick } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { marked } from 'marked'
import { useI18n } from 'vue-i18n'
import { createNote, deleteNote, getNote, listNotes, updateNote, type MyNoteListItem } from '@/api/me/notes'
import { safeMarkdownToHTML, sanitizeHTML } from '@/utils/security'

const { t } = useI18n()

const notes = ref<MyNoteListItem[]>([])
const currentId = ref('')
const content = ref('')
const savedContent = ref('')
const listLoading = ref(false)
const creating = ref(false)
const saving = ref(false)
const previewMode = ref(false)
const justSaved = ref(false)
const editorRef = ref<HTMLTextAreaElement | null>(null)
let savedTimer: ReturnType<typeof setTimeout> | null = null

const dirty = computed(() => content.value !== savedContent.value)

// sicau-v1 notes N-3：安全预览管线与 wiki/manual editor 同源
const previewHTML = computed(() => sanitizeHTML(safeMarkdownToHTML(content.value)))

function formatDate(value: string): string {
    if (!value) return ''
    const d = new Date(value)
    if (Number.isNaN(d.getTime())) return ''
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function loadList() {
    listLoading.value = true
    try {
        const resp = await listNotes()
        if (resp.success) {
            notes.value = resp.data?.notes ?? []
        }
    } finally {
        listLoading.value = false
    }
}

async function openNote(id: string) {
    const resp = await getNote(id)
    if (!resp.success || !resp.data) {
        MessagePlugin.error(resp.message || t('notes.loadFailed'))
        return
    }
    currentId.value = id
    content.value = resp.data.content
    savedContent.value = resp.data.content
    previewMode.value = false
    nextTick(() => editorRef.value?.focus())
}

async function switchTo(id: string) {
    if (id === currentId.value) return
    if (dirty.value && !window.confirm(t('notes.leaveConfirm'))) return
    await openNote(id)
}

async function handleCreate() {
    if (creating.value) return
    creating.value = true
    try {
        const resp = await createNote('')
        if (!resp.success || !resp.data) {
            MessagePlugin.error(resp.message || t('notes.createFailed'))
            return
        }
        // 列表头部插入，避免整表重拉；标题由后端派生（空内容 → 无标题）
        notes.value.unshift({
            id: resp.data.id,
            title: '',
            updated_at: resp.data.updated_at,
        })
        currentId.value = resp.data.id
        content.value = ''
        savedContent.value = ''
        previewMode.value = false
        nextTick(() => editorRef.value?.focus())
    } finally {
        creating.value = false
    }
}

async function handleDelete(id: string) {
    if (id === currentId.value && dirty.value && !window.confirm(t('notes.leaveConfirm'))) return
    const resp = await deleteNote(id)
    if (!resp.success) {
        MessagePlugin.error(resp.message || t('notes.deleteFailed'))
        return
    }
    notes.value = notes.value.filter(n => n.id !== id)
    if (currentId.value === id) {
        currentId.value = ''
        content.value = ''
        savedContent.value = ''
    }
}

async function saveCurrent(explicit: boolean) {
    if (!currentId.value || saving.value) return
    saving.value = true
    try {
        const resp = await updateNote(currentId.value, content.value)
        if (!resp.success) {
            MessagePlugin.error(resp.message || t('notes.saveFailed'))
            return
        }
        savedContent.value = content.value
        // 本地同步列表标题与时间（后端派生规则：首个 # 行 / 首行）
        const item = notes.value.find(n => n.id === currentId.value)
        if (item) {
            item.title = deriveLocalTitle(content.value)
            item.updated_at = new Date().toISOString()
        }
        if (explicit) {
            justSaved.value = true
            if (savedTimer) clearTimeout(savedTimer)
            savedTimer = setTimeout(() => { justSaved.value = false }, 2000)
        }
    } finally {
        saving.value = false
    }
}

// 与后端派生规则一致的本地镜像（仅用于列表即时反馈）
function deriveLocalTitle(text: string): string {
    for (const line of text.split('\n')) {
        const trimmed = line.trim()
        if (!trimmed) continue
        const stripped = trimmed.replace(/^#+\s*/, '').trim()
        return (stripped || trimmed).slice(0, 50)
    }
    return ''
}

function onEditorKeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
        e.preventDefault()
        saveCurrent(true)
    }
}

function beforeUnload(e: BeforeUnloadEvent) {
    if (dirty.value) {
        e.preventDefault()
        e.returnValue = ''
    }
}

onMounted(() => {
    loadList()
    window.addEventListener('beforeunload', beforeUnload)
})

onBeforeUnmount(() => {
    window.removeEventListener('beforeunload', beforeUnload)
})

onBeforeRouteLeave(() => {
    if (dirty.value && !window.confirm(t('notes.leaveConfirm'))) return false
    return true
})
</script>

<style lang="less" scoped>
.notes-page {
    display: flex;
    width: 100%;
    height: 100%;
    min-height: 0;
    background: var(--td-bg-color-container);
}

.notes-list-pane {
    width: 280px;
    flex-shrink: 0;
    border-right: 1px solid var(--td-component-stroke);
    display: flex;
    flex-direction: column;
    min-height: 0;
}

.notes-list-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 14px;
    border-bottom: 1px solid var(--td-component-stroke);

    .notes-list-title {
        font-size: 14px;
        font-weight: 600;
        color: var(--td-text-color-primary);
    }
}

.notes-list-body {
    flex: 1;
    overflow-y: auto;
    padding: 6px;
}

.note-list-item {
    padding: 9px 10px;
    border-radius: 8px;
    cursor: pointer;
    transition: background 0.15s ease;

    &:hover {
        background: var(--td-bg-color-container-hover);
    }

    &.is-active {
        background: var(--td-brand-color-light);

        .note-item-title {
            color: var(--td-brand-color);
        }
    }

    .note-item-title {
        font-size: 13px;
        font-weight: 500;
        color: var(--td-text-color-primary);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        display: flex;
        align-items: center;
        gap: 6px;
    }

    .note-item-meta {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-top: 3px;
        font-size: 11px;
        color: var(--td-text-color-placeholder);
    }

    .note-dirty-dot {
        color: var(--td-warning-color);
        font-size: 10px;
    }
}

.notes-empty-hint {
    padding: 24px 14px;
    text-align: center;
    font-size: 12px;
    color: var(--td-text-color-placeholder);
}

.notes-editor-pane {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    min-height: 0;
}

.notes-editor-bar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 14px;
    border-bottom: 1px solid var(--td-component-stroke);

    .is-active {
        color: var(--td-brand-color);
        background: var(--td-brand-color-light);
    }

    .notes-editor-spacer {
        flex: 1;
    }

    .notes-unsaved {
        font-size: 11px;
        color: var(--td-warning-color);
    }

    .notes-saved {
        font-size: 11px;
        color: var(--td-text-color-placeholder);
    }
}

.notes-editor {
    flex: 1;
    width: 100%;
    box-sizing: border-box;
    border: none;
    outline: none;
    resize: none;
    padding: 18px 24px;
    background: var(--td-bg-color-container);
    color: var(--td-text-color-primary);
    font-size: 14px;
    line-height: 1.7;
    font-family: var(--app-font-family);
}

.notes-preview {
    flex: 1;
    overflow-y: auto;
    padding: 18px 24px;
    font-size: 14px;
    line-height: 1.7;
    color: var(--td-text-color-primary);
}

.notes-editor-empty {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
}

@media (max-width: 768px) {
    .notes-list-pane {
        width: 200px;
    }
}
</style>
