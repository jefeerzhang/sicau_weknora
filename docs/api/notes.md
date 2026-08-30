# 笔记 API（/me/notes）

> sicau-v1 学生笔记（设计：`docs/川农笔记功能设计.md`）。
> 权威 schema 参考 Swagger（`/swagger/index.html`，非 release 模式），与本文档同步维护。

## 概述

- **基础 URL**：`/api/v1/me/notes`
- **认证**：Bearer JWT（web 登录路径；**不**对 API Key 开放，**不**挂 IM 认证——IM 合成账号共享 user_id）
- **身份来源**：tenant / user 均取自认证上下文，请求体与查询串中**不出现** user 标识
- **隔离**：所有读写按 (tenant_id, user_id) 过滤。不存在"列全空间笔记"的端点——教师与其他学生结构上不可见（框架 ADR-012 / 笔记设计 N-1）
- **限额**：200 篇/人；单篇内容 ≤ 1MB（UTF-8 字节）；图片 ≤ 2MB/张、200 张/人

## 错误码约定

| 状态码 | 语义 |
|---|---|
| 400 | 校验失败：内容超 1MB、第 201 篇、图片超 2MB、第 201 张图、类型非 png/jpeg/gif/webp |
| 401 | 未登录 / 缺工作区或用户上下文 |
| 404 | 笔记或图片不存在，**或属于他人**（不区分，避免存在性泄露） |

---

## 1. 笔记列表

```
GET /api/v1/me/notes
```

**响应**：本人笔记，按 `updated_at` 倒序。**不含正文**；`title` 由服务端派生（首个 `#` 标题 → 首个非空行截 50 字符 → 空串，前端显示"无标题"）。

```json
{
  "success": true,
  "data": {
    "notes": [
      { "id": "b21c…", "title": "乡村建设行动方案笔记", "updated_at": "2026-08-30T14:02:11Z" }
    ]
  }
}
```

## 2. 新建笔记

```
POST /api/v1/me/notes
Content-Type: application/json

{ "content": "# 标题\n正文…" }
```

**响应 201**：完整笔记对象（含服务端生成的 id 与时间戳）。第 201 篇或内容超 1MB → 400。

## 3. 读取笔记

```
GET /api/v1/me/notes/:id
```

**响应 200**：`{ "success": true, "data": { "id", "content", "created_at", "updated_at" } }`。
他人笔记 / 不存在 → **404**（同形响应，不泄露存在性）。

## 4. 保存笔记（全量覆盖）

```
PUT /api/v1/me/notes/:id
Content-Type: application/json

{ "content": "更新后的 Markdown…" }
```

**响应 200**：`{ "success": true }`。同一 `updated_at` 语义下**不做并发冲突检测**（last-write-wins，设计 §6 明确不做 If-Match）。

## 5. 删除笔记

```
DELETE /api/v1/me/notes/:id
```

**响应 200**：`{ "success": true }`。删除后触发**图片 GC**：仅被该笔记引用的图片会被回收（其余笔记仍引用的图片保留）——GC best-effort，失败不影响删除结果。

---

## 6. 上传笔记图片

```
POST /api/v1/me/notes/images
Content-Type: multipart/form-data

file: <二进制，≤2MB>
```

**校验**：大小 ≤ 2MB；类型按 **magic bytes 嗅探**（multipart 的 Content-Type 头可伪造，不采信），白名单 `image/png | image/jpeg | image/gif | image/webp`；每人 ≤ 200 张。

**响应 201**：

```json
{
  "success": true,
  "data": {
    "id": "9f2c…",
    "url": "/api/v1/me/notes/images/9f2c…",
    "mime": "image/png"
  }
}
```

Markdown 中以 `![](url)` 引用。

## 7. 读取笔记图片

```
GET /api/v1/me/notes/images/:id
```

- **仅 owner 可读**：非本人 → 404（capability URL 的 UUID 只是路由参数，权限始终服务端校验）
- 响应头：`Cache-Control: private, max-age=31536000, immutable`、`ETag: "<id>"`（`If-None-Match` 命中返回 304）
- 前端经认证 fetch → objectURL 渲染，不裸用 `<img src>`

## 8. 删除笔记图片

```
DELETE /api/v1/me/notes/images/:id
```

**响应 200**：`{ "success": true }`。删除即时释放该张配额。

---

## 数据保留

成员被移出空间后，其笔记与图片**保留**，随租户删除时一并清理（设计 §6）。
