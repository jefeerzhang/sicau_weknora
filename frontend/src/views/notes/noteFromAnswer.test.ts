import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

import { deriveLocalTitle } from './deriveLocalTitle'
import { appendAnswerNote, composeAnswerNote } from './noteFromAnswer'
import {
  applyBulletList,
  applyHeading,
  insertText,
  wrapRange,
} from '../../utils/markdownSelection'

const here = dirname(fileURLToPath(import.meta.url))
const read = (rel: string) => readFileSync(join(here, rel), 'utf8')

test('a question becomes the note heading and the list title', () => {
  const block = composeAnswerNote('乡村建设怎么推进', '先看规划，再看案例。')
  assert.equal(block, '# 乡村建设怎么推进\n\n先看规划，再看案例。')
  assert.equal(deriveLocalTitle(block), '乡村建设怎么推进')
})

test('a long question is shortened to the title limit', () => {
  const block = composeAnswerNote('问'.repeat(80), '答案')
  assert.equal(deriveLocalTitle(block), '问'.repeat(50))
  assert.match(block, /^# 问{50}\n\n答案$/)
})

test('a missing question stores the answer alone', () => {
  assert.equal(composeAnswerNote('   ', '只有回答'), '只有回答')
  assert.equal(composeAnswerNote('', '只有回答'), '只有回答')
})

test('append keeps the original list title', () => {
  const original = '# 课程提纲\n\n第一节'
  const next = appendAnswerNote(original, '第二问', '第二答')
  assert.equal(deriveLocalTitle(next), '课程提纲')
  assert.match(next, /# 第二问\n\n第二答$/)
})

test('append does not invent a heading when the note has none', () => {
  const original = '随手记'
  const next = appendAnswerNote(original, '这节课的问题', '回答')
  assert.equal(deriveLocalTitle(next), '随手记')
  assert.doesNotMatch(next, /^# /m)
  assert.match(next, /这节课的问题\n\n回答$/)
})

test('append onto an empty note is a new note', () => {
  assert.equal(appendAnswerNote('', '问题', '回答'), composeAnswerNote('问题', '回答'))
})

test('writing actions wrap a selection and turn a line into a heading or list', () => {
  const wrapped = wrapRange('hello', 0, 5, '**', '**', 'bold')
  assert.equal(wrapped.value, '**hello**')
  assert.equal(wrapped.selectionStart, 2)
  assert.equal(wrapped.selectionEnd, 7)

  const placeholder = wrapRange('hi', 2, 2, '`', '`', 'code')
  assert.equal(placeholder.value, 'hi`code`')

  const heading = applyHeading('正文', 0, 2, 2, '标题')
  assert.equal(heading.value, '## 正文')

  const list = applyBulletList('一项', 0, 2, '条目')
  assert.equal(list.value, '- 一项')

  const inserted = insertText('ab', 1, 1, '\n---\n\n')
  assert.equal(inserted.value, 'a\n---\n\nb')
})

test('both finished-answer toolbars can save to notes, and embed cannot', () => {
  const bot = read('../chat/components/botmsg.vue')
  const agent = read('../chat/components/AgentStreamDisplay.vue')
  assert.match(bot, /SaveAnswerToNoteButton v-if="!embeddedMode"/)
  assert.match(bot, /answer-toolbar__labeled/)
  assert.match(agent, /answer-toolbar__labeled/)
  const noteButton = read('../../components/SaveAnswerToNoteButton.vue')
  assert.match(noteButton, /answer-toolbar__labeled--note/)
  assert.match(noteButton, /notes\.saveAnswer/)
  const agentAt = agent.indexOf('SaveAnswerToNoteButton')
  assert.ok(agentAt > 0)
  assert.match(agent.slice(Math.max(0, agentAt - 600), agentAt), /!embeddedMode/)
})

test('notes page keeps a preview switch and writing actions, not a rich-text surface', () => {
  const notes = read('./MyNotes.vue')
  assert.match(notes, /notes\.preview/)
  assert.match(notes, /textformat-bold/)
  assert.match(notes, /view-list/)
  assert.match(notes, /notes\.insertImage/)
  assert.doesNotMatch(notes, /contenteditable/)
  assert.doesNotMatch(notes, /!\[\$\{/)
})
