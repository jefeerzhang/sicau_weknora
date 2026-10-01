export type TextEdit = {
  value: string
  selectionStart: number
  selectionEnd: number
}

const LIST_PREFIX = /^(\s*(?:[-*+]|\d+\.)\s+|\s*-\s+\[[ xX]\]\s+)/

function clamp(start: number, end: number, length: number) {
  const safeStart = Math.max(0, Math.min(start, length))
  const safeEnd = Math.max(safeStart, Math.min(end, length))
  return { safeStart, safeEnd }
}

function lineStart(value: string, index: number) {
  if (index <= 0) return 0
  const lastNewline = value.lastIndexOf('\n', index - 1)
  return lastNewline === -1 ? 0 : lastNewline + 1
}

function lineEnd(value: string, index: number) {
  if (index >= value.length) return value.length
  const newlineIndex = value.indexOf('\n', index)
  return newlineIndex === -1 ? value.length : newlineIndex
}

export function wrapRange(
  value: string,
  start: number,
  end: number,
  prefix: string,
  suffix: string,
  placeholder: string,
): TextEdit {
  const { safeStart, safeEnd } = clamp(start, end, value.length)
  const selected = safeEnd > safeStart ? value.slice(safeStart, safeEnd) : placeholder
  const selectionStart = safeStart + prefix.length
  return {
    value: value.slice(0, safeStart) + prefix + selected + suffix + value.slice(safeEnd),
    selectionStart,
    selectionEnd: selectionStart + selected.length,
  }
}

export function mapSelectedLines(
  value: string,
  start: number,
  end: number,
  map: (line: string, index: number) => string,
): TextEdit {
  const { safeStart, safeEnd } = clamp(start, end, value.length)
  const from = lineStart(value, safeStart)
  const to = lineEnd(value, safeEnd)
  const result = value.slice(from, to).split('\n').map(map).join('\n')
  return {
    value: value.slice(0, from) + result + value.slice(to),
    selectionStart: from,
    selectionEnd: from + result.length,
  }
}

export function insertText(
  value: string,
  start: number,
  end: number,
  text: string,
  selectionStartOffset?: number,
  selectionEndOffset?: number,
): TextEdit {
  const { safeStart, safeEnd } = clamp(start, end, value.length)
  const selectionStart = selectionStartOffset !== undefined
    ? safeStart + selectionStartOffset
    : safeStart + text.length
  const selectionEnd = selectionEndOffset !== undefined
    ? safeStart + selectionEndOffset
    : selectionStart
  return {
    value: value.slice(0, safeStart) + text + value.slice(safeEnd),
    selectionStart,
    selectionEnd,
  }
}

export function applyHeading(
  value: string,
  start: number,
  end: number,
  level: number,
  placeholder: string,
): TextEdit {
  const hashes = '#'.repeat(Math.min(3, Math.max(1, level)))
  return mapSelectedLines(value, start, end, (line) => {
    const trimmed = line.replace(/^#+\s*/, '').trim()
    return `${hashes} ${trimmed || placeholder}`
  })
}

export function applyBulletList(value: string, start: number, end: number, placeholder: string): TextEdit {
  return mapSelectedLines(value, start, end, (line) => {
    const content = line.trim().replace(LIST_PREFIX, '').trim()
    return `- ${content || placeholder}`
  })
}

export function applyOrderedList(value: string, start: number, end: number, placeholder: string): TextEdit {
  return mapSelectedLines(value, start, end, (line, index) => {
    const content = line.trim().replace(LIST_PREFIX, '').trim()
    return `${index + 1}. ${content || placeholder}`
  })
}

export function applyTaskList(value: string, start: number, end: number, placeholder: string): TextEdit {
  return mapSelectedLines(value, start, end, (line) => {
    const content = line.trim().replace(LIST_PREFIX, '').trim()
    return `- [ ] ${content || placeholder}`
  })
}

export function applyBlockquote(value: string, start: number, end: number, placeholder: string): TextEdit {
  return mapSelectedLines(value, start, end, (line) => {
    const trimmed = line.trim().replace(/^>\s?/, '').trim()
    return `> ${trimmed || placeholder}`
  })
}

export function insertLink(
  value: string,
  start: number,
  end: number,
  placeholder: string,
): TextEdit {
  const { safeStart, safeEnd } = clamp(start, end, value.length)
  const selected = safeEnd > safeStart ? value.slice(safeStart, safeEnd) : placeholder
  const url = 'https://'
  const text = `[${selected}](${url})`
  const urlStart = selected.length + 3
  return insertText(value, safeStart, safeEnd, text, urlStart, urlStart + url.length)
}

export function insertCodeBlock(value: string, start: number, end: number, placeholder: string): TextEdit {
  const block = `\n\`\`\`\n${placeholder}\n\`\`\`\n`
  const offset = block.indexOf(placeholder)
  return insertText(value, start, end, block, offset, offset + placeholder.length)
}

export function insertTable(value: string, start: number, end: number, column1: string, column2: string, cell: string): TextEdit {
  const template = `\n| ${column1} | ${column2} |\n| --- | --- |\n| ${cell} | ${cell} |\n`
  const offset = template.indexOf(cell)
  return insertText(value, start, end, template, offset, offset + cell.length)
}

export function insertHorizontalRule(value: string, start: number, end: number): TextEdit {
  return insertText(value, start, end, '\n---\n\n')
}
