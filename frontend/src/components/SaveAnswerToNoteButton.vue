<template>
    <t-dropdown v-if="currentNote" trigger="click" placement="bottom-left" :options="menuOptions" @click="onMenu">
        <t-button size="small" variant="outline" class="answer-toolbar__labeled answer-toolbar__labeled--note"
            :loading="busy" @click.stop>
            <t-icon name="file-add" />
            <span>{{ t('notes.saveAnswer') }}</span>
        </t-button>
    </t-dropdown>
    <t-button v-else size="small" variant="outline" class="answer-toolbar__labeled answer-toolbar__labeled--note"
        :loading="busy" @click.stop="createNew">
        <t-icon name="file-add" />
        <span>{{ t('notes.saveAnswer') }}</span>
    </t-button>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { createNote, getNote, updateNote } from '@/api/me/notes'
import { appendAnswerNote, composeAnswerNote, answerNoteTitle } from '@/views/notes/noteFromAnswer'
import { currentNote, forgetCurrentNote, rememberCurrentNote } from '@/views/notes/currentNote'

const props = defineProps<{
    question?: string
    answer?: string
}>()

const { t } = useI18n()
const busy = ref(false)

const menuOptions = computed(() => [
    { content: t('notes.saveAsNew'), value: 'new' },
    {
        content: t('notes.appendToCurrent', {
            title: currentNote.value?.title || t('notes.untitled'),
        }),
        value: 'append',
    },
])

function onMenu(item: { value: string }) {
    if (item.value === 'append') void appendCurrent()
    else void createNew()
}

async function createNew() {
    const block = composeAnswerNote(props.question || '', props.answer || '')
    if (!block.trim()) return
    busy.value = true
    try {
        const resp = await createNote(block)
        if (!resp.success || !resp.data) {
            MessagePlugin.error(resp.message || t('notes.saveFailed'))
            return
        }
        rememberCurrentNote(resp.data.id, answerNoteTitle(block) || t('notes.untitled'))
        MessagePlugin.success(t('notes.savedToNotes'))
    } catch (err: any) {
        MessagePlugin.error(err?.message || t('notes.saveFailed'))
    } finally {
        busy.value = false
    }
}

async function appendCurrent() {
    const target = currentNote.value
    if (!target) {
        await createNew()
        return
    }
    busy.value = true
    try {
        const got = await getNote(target.id)
        if (!got.success || !got.data) {
            forgetCurrentNote(target.id)
            MessagePlugin.error(got.message || t('notes.loadFailed'))
            return
        }
        const next = appendAnswerNote(got.data.content, props.question || '', props.answer || '')
        const resp = await updateNote(target.id, next)
        if (!resp.success) {
            MessagePlugin.error(resp.message || t('notes.saveFailed'))
            return
        }
        rememberCurrentNote(target.id, answerNoteTitle(next) || target.title)
        MessagePlugin.success(t('notes.savedToNotes'))
    } catch (err: any) {
        MessagePlugin.error(err?.message || t('notes.saveFailed'))
    } finally {
        busy.value = false
    }
}
</script>
