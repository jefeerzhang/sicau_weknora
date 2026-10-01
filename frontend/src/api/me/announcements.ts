/**
 * sicau-v1 announcements: /announcements API (docs/川农公告板设计.md §3).
 * Reads are Viewer+; posting is Contributor+ (multipart with files[]);
 * comment deletes are author-or-admin server-side.
 */
import { del, get, getDown, post, postUpload } from '@/utils/request'

export interface AnnouncementAttachment {
  name: string
  size: number
}

export interface Announcement {
  id: string
  title: string
  content: string
  attachments: AnnouncementAttachment[]
  user_id?: string
  author_name?: string
  created_at: string
  updated_at: string
}

export interface AnnouncementListItem {
  id: string
  title: string
  content: string
  user_id?: string
  author_name?: string
  attachments: AnnouncementAttachment[]
  created_at: string
  updated_at: string
}

export interface AnnouncementCommentItem {
  id: string
  user_id: string
  author_name: string
  content: string
  created_at: string
}

type Resp<T> = { success: boolean; data?: T; message?: string }

export async function listAnnouncements(): Promise<Resp<{ announcements: AnnouncementListItem[] }>> {
  return (await get('/api/v1/announcements')) as unknown as Resp<{ announcements: AnnouncementListItem[] }>
}

export async function getAnnouncement(id: string): Promise<Resp<Announcement>> {
  return (await get(`/api/v1/announcements/${id}`)) as unknown as Resp<Announcement>
}

/** multipart 发布：title + content + files[]（≤5 个、单个 ≤50MB、白名单类型） */
export async function createAnnouncement(
  title: string,
  content: string,
  files: File[],
): Promise<Resp<Announcement>> {
  const form = new FormData()
  form.append('title', title)
  form.append('content', content)
  for (const f of files) form.append('files', f)
  return (await postUpload('/api/v1/announcements', form)) as unknown as Resp<Announcement>
}

export async function deleteAnnouncement(id: string): Promise<Resp<null>> {
  return (await del(`/api/v1/announcements/${id}`)) as unknown as Resp<null>
}

export async function listComments(announcementId: string): Promise<Resp<{ comments: AnnouncementCommentItem[] }>> {
  return (await get(`/api/v1/announcements/${announcementId}/comments`)) as unknown as Resp<{ comments: AnnouncementCommentItem[] }>
}

export async function createComment(announcementId: string, content: string): Promise<Resp<AnnouncementCommentItem>> {
  return (await post(`/api/v1/announcements/${announcementId}/comments`, { content })) as unknown as Resp<AnnouncementCommentItem>
}

export async function deleteComment(announcementId: string, commentId: string): Promise<Resp<null>> {
  return (await del(`/api/v1/announcements/${announcementId}/comments/${commentId}`)) as unknown as Resp<null>
}

/** 认证下载公告附件（Bearer 随请求），返回 Blob 供浏览器保存 */
export async function downloadAnnouncementAttachment(announcementId: string, index: number): Promise<Blob> {
  return (await getDown(`/api/v1/announcements/${announcementId}/attachments/${index}`)) as unknown as Blob
}
