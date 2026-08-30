<template>
    <div class="notes-page">
        <!-- 左：笔记列表 -->
        <aside class="notes-list-pane">
            <div class="notes-list-header">
                <span class="notes-list-title">{{ $t('notes.title') }}</span>
                <t-button theme="primary" variant="text" size="small" @click="handleCreate">
                    <template #icon><t-icon name="add" /></template>
                    {{ $t('notes.newNote') }}
                </t-button>
            </div>
            <div class="notes-list-body">
                <div v-if="listLoading" class="notes-empty-hint">{{ $t('notes.loading') }}</div>
                <div v-else-if="notes.length === 0 && !isNewDraft" class="notes-empty-hint">
                    {{ $t('notes.empty') }}
                </div>
                <!-- sicau-v1 N-4(v2)：草稿态也在列表顶部呈现（未落库前即可见） -->
                <div v-if="isNewDraft" class="note-list-item is-active">
                    <div class="note-item-title">{{ deriveLocalTitle(content) || $t('notes.untitled') }}<span
                            class="note-dirty-dot">●</span></div>
                    <div class="note-item-meta">
                        <span>{{ $t('notes.draft') }}</span>
                        <t-button theme="danger" variant="text" shape="square" size="small"
                            @click.stop="discardDraft">
                            <template #icon><t-icon name="delete" size="13px" /></template>
                        </t-button>
                    </div>
                </div>
                <div v-for="item in notes" :key="item.id" class="note-list-item" :class="{
                    'is-active': item.id === selectedKey,
                    'is-dirty': item.id === selectedKey && dirty
                }" @click="switchTo(item.id)">
                    <div class="note-item-title">{{ item.title || $t('notes.untitled') }}<span
                            v-if="item.id === selectedKey && dirty" class="note-dirty-dot">●</span></div>
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
            <template v-if="currentId || isNewDraft">
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
                    <!-- sicau-v1 ticket 09: 贴图（仅编辑模式可用） -->
                    <t-button v-if="!previewMode" variant="text" size="small" :loading="uploadingImage"
                        :title="$t('notes.insertImage')" @click="imageInputRef?.click()">
                        <template #icon><t-icon name="image" /></template>
                    </t-button>
                    <input ref="imageInputRef" type="file" accept="image/png,image/jpeg,image/gif,image/webp"
                        style="display: none" @change="onImageFileChange" />
                    <span class="notes-editor-spacer"></span>
                    <!-- sicau-v1 N-4(v2)：自动保存状态机 -->
                    <span v-if="saving" class="notes-unsaved">{{ $t('notes.savingNow') }}</span>
                    <span v-else-if="autoSaveFailed && dirty" class="notes-unsaved notes-unsaved--error">
                        {{ $t('notes.autoSaveRetry') }}
                    </span>
                    <span v-else-if="justSaved" class="notes-saved">{{ $t('notes.saved') }}</span>
                    <span v-else-if="dirty" class="notes-unsaved">{{ $t('notes.unsaved') }}</span>
                    <t-button theme="primary" size="small" :loading="saving" @click="flushSave()">
                        {{ $t('notes.save') }}
                    </t-button>
                </div>
                <textarea v-if="!previewMode" ref="editorRef" v-model="content" class="notes-editor"
                    :placeholder="$t('notes.placeholder')" @keydown="onEditorKeydown"
                    @paste="onEditorPaste" @input="scheduleAutoSave"></textarea>
                <div v-else class="notes-preview markdown-body" v-html="previewHTML"></div>
            </template>
            <div v-else class="notes-empty-hint notes-editor-empty">
                {{ notes.length === 0 && !isNewDraft ? $t('notes.empty') : $t('notes.selectHint') }}
            </div>
        </section>
    </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { marked } from 'marked'
import { useI18n } from 'vue-i18n'
import { createNote, deleteNote, getNote, listNotes, updateNote, type MyNoteListItem } from '@/api/me/notes'
import { safeMarkdownToHTML, sanitizeHTML } from '@/utils/security'
import { fetchNoteImageBlob, uploadNoteImage } from '@/api/me/notes'

const { t } = useI18n()

const notes = ref<MyNoteListItem[]>([])
const currentId = ref('')
// sicau-v1 notes N-4(v2)：延迟创建——「新建」先进草稿态，
// 首次自动保存才真正 POST，空草稿切走不留痕。
const isNewDraft = ref(false)
const content = ref('')
const savedContent = ref('')
const listLoading = ref(false)
const saving = ref(false)
const previewMode = ref(false)
const justSaved = ref(false)
// 自动保存失败：保留 dirty + 一次性提示，继续输入自动重试
const autoSaveFailed = ref(false)
const editorRef = ref<HTMLTextAreaElement | null>(null)
const imageInputRef = ref<HTMLInputElement | null>(null)
const uploadingImage = ref(false)
let savedTimer: ReturnType<typeof setTimeout> | null = null
let autoSaveTimer: ReturnType<typeof setTimeout> | null = null
let inFlight: Promise<boolean> | null = null
// 预览里笔记图片经认证取回后的 objectURL，统一回收
const noteImageObjectURLs = ref<string[]>([])

const dirty = computed(() => content.value !== savedContent.value)
// 当前激活键：草稿态为 'new'，真实笔记为其 id
const selectedKey = computed(() => (isNewDraft.value ? 'new' : currentId.value))

// N-3：预览管线与 manual editor 同源——
// 去脚本标签 → marked 渲染 → DOMPurify 清理
const previewHTML = computed(() => {
    if (!content.value) return ''
    const html = marked.parse(safeMarkdownToHTML(content.value), { async: false })
    return sanitizeHTML(html as string)
})

function formatDate(value: string): string {
    if (!value) return ''
    const d = new Date(value)
    if (Number.isNaN(d.getTime())) return ''
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
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

// ---------- 自动保存核心（N-4 v2） ----------

async function persist(): Promise<boolean> {
    if (saving.value) return true
    saving.value = true
    try {
        if (isNewDraft.value) {
            const resp = await createNote(content.value)
            if (!resp.success || !resp.data) {
                throw new Error(resp.message || t('notes.saveFailed'))
            }
            isNewDraft.value = false
            currentId.value = resp.data.id
            savedContent.value = content.value
            notes.value.unshift({
                id: resp.data.id,
                title: deriveLocalTitle(content.value),
                updated_at: resp.data.updated_at,
            })
        } else {
            const resp = await updateNote(currentId.value, content.value)
            if (!resp.success) {
                throw new Error(resp.message || t('notes.saveFailed'))
            }
            savedContent.value = content.value
            const item = notes.value.find(n => n.id === currentId.value)
            if (item) {
                item.title = deriveLocalTitle(content.value)
                item.updated_at = new Date().toISOString()
            }
        }
        autoSaveFailed.value = false
        justSaved.value = true
        if (savedTimer) clearTimeout(savedTimer)
        savedTimer = setTimeout(() => { justSaved.value = false }, 2000)
        return true
    } catch (err: any) {
        // Q3(a)：保留 dirty + 一次性错误提示，继续输入自动重试
        autoSaveFailed.value = true
        MessagePlugin.error(err?.message || t('notes.saveFailed'))
        return false
    } finally {
        saving.value = false
    }
}

function scheduleAutoSave() {
    if (isNewDraft.value && content.value === '') return // 空草稿不触发
    if (!dirty.value) return
    if (autoSaveTimer) clearTimeout(autoSaveTimer)
    autoSaveTimer = setTimeout(() => {
        autoSaveTimer = null
        if (inFlight) {
            scheduleAutoSave() // 保存中又有改动 → 顺延
            return
        }
        if (dirty.value) void persist()
    }, 1500)
}

async function flushSave() {
    if (autoSaveTimer) {
        clearTimeout(autoSaveTimer)
        autoSaveTimer = null
    }
    if (inFlight) await inFlight
    if (isNewDraft.value && content.value === '') return
    if (!dirty.value) return
    await persist()
}

async function ensurePersisted(): Promise<boolean> {
    if (autoSaveTimer) {
        clearTimeout(autoSaveTimer)
        autoSaveTimer = null
    }
    if (isNewDraft.value && content.value === '') {
        resetDraft()
        return true
    }
    if (inFlight) await inFlight
    if (!dirty.value) return true
    return await persist()
}

function resetDraft() {
    isNewDraft.value = false
    currentId.value = ''
    content.value = ''
    savedContent.value = ''
    autoSaveFailed.value = false
}

// ---------- 交互 ----------

async function handleCreate() {
    if (selectedKey.value === 'new') return
    if (!(await ensurePersisted())) return
    revokeNoteImageURLs()
    isNewDraft.value = true
    currentId.value = ''
    content.value = ''
    savedContent.value = ''
    previewMode.value = false
    autoSaveFailed.value = false
    nextTick(() => editorRef.value?.focus())
}

function discardDraft() {
    resetDraft()
}

async function switchTo(id: string) {
    if (id === selectedKey.value) return
    if (!(await ensurePersisted())) return
    await openNote(id)
}

async function openNote(id: string) {
    if (autoSaveTimer) {
        clearTimeout(autoSaveTimer)
        autoSaveTimer = null
    }
    isNewDraft.value = false
    autoSaveFailed.value = false
    revokeNoteImageURLs()
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

async function handleDelete(id: string) {
    if (id === selectedKey.value) {
        if (!(await ensurePersisted())) return
        if (isNewDraft.value) {
            discardDraft()
            return
        }
    }
    const resp = await deleteNote(id)
    if (!resp.success) {
        MessagePlugin.error(resp.message || t('notes.deleteFailed'))
        return
    }
    notes.value = notes.value.filter(n => n.id !== id)
    if (currentId.value === id) {
        resetDraft()
    }
}

function onEditorKeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
        e.preventDefault()
        void flushSave()
    }
}

// ---------- 贴图（ticket 09） ----------

function insertAtCursor(text: string) {
    const el = editorRef.value
    if (!el) {
        content.value += text
        return
    }
    const start = el.selectionStart ?? content.value.length
    const end = el.selectionEnd ?? start
    content.value = content.value.slice(0, start) + text + content.value.slice(end)
    nextTick(() => {
        el.focus()
        const pos = start + text.length
        el.setSelectionRange(pos, pos)
    })
}

async function uploadAndInsertImage(file: File) {
    if (uploadingImage.value) return
    uploadingImage.value = true
    try {
        const resp = await uploadNoteImage(file)
        if (!resp.success || !resp.data) {
            MessagePlugin.error(resp.message || t('notes.imageUploadFailed'))
            return
        }
        insertAtCursor(`\n![](${resp.data.url})\n`)
        MessagePlugin.success(t('notes.imageInserted'))
    } finally {
        uploadingImage.value = false
    }
}

function onImageFileChange(e: Event) {
    const input = e.target as HTMLInputElement
    const file = input.files?.[0]
    if (file) void uploadAndInsertImage(file)
    input.value = ''
}

function onEditorPaste(e: ClipboardEvent) {
    const items = e.clipboardData?.items
    if (!items) return
    for (const item of items) {
        if (item.type.startsWith('image/')) {
            const file = item.getAsFile()
            if (file) {
                e.preventDefault()
                void uploadAndInsertImage(file)
            }
            return
        }
    }
}

// ---------- 预览图片 hydration（ticket 09） ----------

async function hydrateNoteImages() {
    if (!previewMode.value) return
    await nextTick()
    const pane = document.querySelector('.notes-preview')
    if (!pane) return
    const imgs = Array.from(pane.querySelectorAll('img')) as HTMLImageElement[]
    for (const img of imgs) {
        const src = img.getAttribute('src') || ''
        if (!src.startsWith('/api/v1/me/notes/images/')) continue
        img.setAttribute('src', '')
        try {
            const blob = await fetchNoteImageBlob(src)
            const url = URL.createObjectURL(blob)
            noteImageObjectURLs.value.push(url)
            img.src = url
        } catch {
            img.alt = t('notes.imageLoadFailed')
        }
    }
}

watch(previewHTML, () => { void hydrateNoteImages() })
watch(previewMode, (on) => { if (on) void hydrateNoteImages() })

function revokeNoteImageURLs() {
    for (const url of noteImageObjectURLs.value) URL.revokeObjectURL(url)
    noteImageObjectURLs.value = []
}

// ---------- 生命周期与守卫 ----------

function beforeUnload(e: BeforeUnloadEvent) {
    if (dirty.value || inFlight || (isNewDraft.value && content.value !== '')) {
        e.preventDefault()
        e.returnValue = ''
    }
}

onMounted(() => {
    loadList()
    window.addEventListener('beforeunload', beforeUnload)
})

onBeforeUnmount(() => {
    if (autoSaveTimer) clearTimeout(autoSaveTimer)
    revokeNoteImageURLs()
    window.removeEventListener('beforeunload', beforeUnload)
})

onBeforeRouteLeave(async () => {
    // 冲刷优先：离开前落库；仅自动保存失败时拦截
    return await ensurePersisted()
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

    .notes-unsaved--error {
        color: var(--td-error-color);
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
