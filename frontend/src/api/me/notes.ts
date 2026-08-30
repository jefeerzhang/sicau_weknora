/**
 * sicau-v1 notes: /me/notes CRUD (docs/川农笔记功能设计.md §4).
 * The backend derives the caller from the auth context — no user ids in
 * payloads. List items carry a server-derived title, never the content.
 */
import { del, get, getDown, post, postUpload, put } from '@/utils/request'

export interface MyNoteListItem {
  id: string
  title: string
  updated_at: string
}

export interface MyNote {
  id: string
  content: string
  created_at: string
  updated_at: string
}

type NotesResponse<T> = { success: boolean; data?: T; message?: string }

export async function listNotes(): Promise<NotesResponse<{ notes: MyNoteListItem[] }>> {
  return (await get('/api/v1/me/notes')) as unknown as NotesResponse<{ notes: MyNoteListItem[] }>
}

export async function createNote(content: string): Promise<NotesResponse<MyNote>> {
  return (await post('/api/v1/me/notes', { content })) as unknown as NotesResponse<MyNote>
}

export async function getNote(noteId: string): Promise<NotesResponse<MyNote>> {
  return (await get(`/api/v1/me/notes/${noteId}`)) as unknown as NotesResponse<MyNote>
}

export async function updateNote(noteId: string, content: string): Promise<NotesResponse<null>> {
  return (await put(`/api/v1/me/notes/${noteId}`, { content })) as unknown as NotesResponse<null>
}

export async function deleteNote(noteId: string): Promise<NotesResponse<null>> {
  return (await del(`/api/v1/me/notes/${noteId}`)) as unknown as NotesResponse<null>
}

/**
 * sicau-v1 ticket 09: note image upload (multipart, ≤2MB, sniffed type
 * server-side). Returns the capability-URL-shaped markdown source plus the
 * image id for quota bookkeeping.
 */
export async function uploadNoteImage(
  file: File,
): Promise<{ success: boolean; data?: { id: string; url: string; mime: string }; message?: string }> {
  const form = new FormData();
  form.append('file', file);
  return (await postUpload('/api/v1/me/notes/images', form)) as unknown as {
    success: boolean
    data?: { id: string; url: string; mime: string }
    message?: string
  }
}

/**
 * Authed fetch of a note image as a Blob — the <img> pipeline must go
 * through this (bearer token per request), never a bare <img src>.
 * Mirrors utils/request getDown.
 */
export async function fetchNoteImageBlob(url: string): Promise<Blob> {
  return (await getDown(url)) as unknown as Blob
}
