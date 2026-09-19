import { deriveLocalTitle } from './deriveLocalTitle'

const TITLE_LIMIT = 50

function condenseQuestion(question: string): string {
  const condensed = (question || '').replace(/\s+/g, ' ').trim()
  return [...condensed].slice(0, TITLE_LIMIT).join('')
}

function hasHeading(text: string): boolean {
  return text.split('\n').some((line) => {
    const trimmed = line.trim()
    return trimmed.startsWith('#') && trimmed.replace(/^#+/, '').trim().length > 0
  })
}

/** Markdown block stored for one answer. A question becomes the list title. */
export function composeAnswerNote(question: string, answer: string): string {
  const body = (answer || '').trim()
  const heading = condenseQuestion(question)
  if (!heading) return body
  return `# ${heading}\n\n${body}`
}

/**
 * Append an answer without stealing the list title.
 * A later heading would win under the title rule when the note has none,
 * so that case keeps the question as plain text.
 */
export function appendAnswerNote(existing: string, question: string, answer: string): string {
  const base = (existing || '').replace(/\s+$/, '')
  const block = (base && !hasHeading(base)
    ? plainAnswerNote(question, answer)
    : composeAnswerNote(question, answer)
  ).trim()
  if (!base) return block
  if (!block) return base
  return `${base}\n\n${block}`
}

function plainAnswerNote(question: string, answer: string): string {
  const body = (answer || '').trim()
  const heading = condenseQuestion(question)
  if (!heading) return body
  if (!body) return heading
  return `${heading}\n\n${body}`
}

export function answerNoteTitle(text: string): string {
  return deriveLocalTitle(text)
}
