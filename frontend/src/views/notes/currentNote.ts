import { ref } from 'vue'

export type CurrentNote = { id: string; title: string }

/** Last note opened or created in this page visit. Not stored on disk. */
export const currentNote = ref<CurrentNote | null>(null)

export function rememberCurrentNote(id: string, title: string) {
  if (!id) return
  currentNote.value = { id, title }
}

export function forgetCurrentNote(id?: string) {
  if (!id || currentNote.value?.id === id) currentNote.value = null
}
