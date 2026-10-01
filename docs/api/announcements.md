# 公告板 API（/announcements）

> sicau-v1 课程公告板（设计：`docs/川农公告板设计.md`）。权威 schema 参考 Swagger（非 release 模式）。

## 概述

- **基础 URL**：`/api/v1/announcements`
- **认证**：Bearer JWT（web 路径；不对 API Key 开放）
- **读**：全员（Viewer+）；**发**：Contributor+；**删公告**：作者或 admin；**删留言**：留言作者或 admin
- **附件**：走 workspace FileService，单文件 ≤ 50MB、每公告 ≤ 5 个、扩展名白名单（pdf/doc/docx/ppt/pptx/xls/xlsx/zip/rar/7z/txt/md）
- **隔离**：按 tenant 过滤；作者名由服务端批量 hydrate

**部署注意**：前端 nginx `MAX_FILE_SIZE_MB` 需 ≥ 60（50MB 文件 + multipart 开销）。

## 端点

| 方法 | 路径 | 门禁 | 说明 |
|---|---|---|---|
| GET | `/announcements` | viewer+ | 列表（含全文、附件元数据、作者名），`created_at` 倒序；不含留言 |
| POST | `/announcements` | contributor+ | multipart：`title`（必填）+ `content` + `files[]`（≤5 个、单个 ≤50MB、扩展名白名单） |
| GET | `/announcements/:id` | viewer+ | 详情（全文 + 附件 + 作者名） |
| DELETE | `/announcements/:id` | 作者或 admin | 连带 FileService 删附件文件、级联删留言 |
| GET | `/announcements/:id/attachments/:index` | viewer+ | 按下标流式下载附件 |
| GET | `/announcements/:id/comments` | viewer+ | 平铺留言，时间正序，含作者名 |
| POST | `/announcements/:id/comments` | viewer+ | `{"content": "text"}`，纯文本，非空，≤4KiB |
| DELETE | `/announcements/:id/comments/:cid` | 留言作者或 admin | 删除单条留言 |

## 响应形状

公告对象：

```json
{
  "id": "b21c…",
  "user_id": "u-…",
  "title": "第一次作业",
  "content": "请完成第一章习题…",
  "attachments": [{ "name": "作业一.pdf", "size": 12345 }],
  "author_name": "张剑1234",
  "created_at": "…", "updated_at": "…"
}
```

附件对象只含 `name` / `size`（不含内部 FileService `path`；下载按下标）。
留言对象：`{ "id", "user_id", "author_name", "content", "created_at" }`。

## 错误码

| 状态码 | 场景 |
|---|---|
| 400 | 标题缺失、附件 >50MB / >5 个 / 白名单外扩展名、留言为空或 >4KiB |
| 401 | 未登录 |
| 403 | viewer 发公告；非作者且非 admin 删公告；非留言作者且非 admin 删留言 |
| 404 | 公告/附件/留言不存在（他人可见资源不存在时不区分） |

## 语义说明

- **无编辑**：公告发错删除重发（A-4）。PUT 端点不存在。
- **删除级联**：删公告 → FileService 删每个附件文件（best-effort，失败仅告警）+ 级联删留言。
- **数据保留**：成员移出空间后其公告/留言保留，随租户删除清理。
