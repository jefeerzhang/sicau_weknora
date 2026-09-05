# 新对话空状态居中还原 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore horizontal + vertical centering of the new-conversation empty state in the right pane for both teacher and student (same page).

**Architecture:** Move `.dialogue-wrap` / `.dialogue-answers` / `.dialogue-title` layout CSS out of `creatChat.vue`'s broken `:deep(...)` scoped rules into a co-located `newConversationEmptyState.less` owned by the empty-state shell. Import that stylesheet from `creatChat.vue` as an **unscoped** style block so the shell root actually receives the rules. Add a source-level regression test so a future `:deep` on the child root cannot silently drop centering again.

**Tech Stack:** Vue 3 SFC + Less, Node test runner via `tsx --test`, existing SSR render helpers in `creatChat.test.ts`.

**Spec:** `docs/新对话空状态居中规格.md` (approved)

## Global Constraints

- Teacher and student share one implementation; no role-forked UI.
- Change layout only; do not change title copy, suggested questions, composer, i18n, API, menu, or `chat/index.vue`.
- Keep `data-testid="new-conversation-empty"` and questions / composer slots.
- Empty-state content block stays `max-width: 960px`.
- Do not import `.less` from `newConversationEmptyState.ts` (tsx unit tests import that file directly and cannot load Less).
- Prefer reading the Less source in tests for the layout contract; do not add Playwright unless already required elsewhere for this change.

---

## File map

| File | Responsibility |
| --- | --- |
| `frontend/src/views/creatChat/newConversationEmptyState.less` | **Create.** Own centering + empty-state column/title layout CSS. |
| `frontend/src/views/creatChat/newConversationEmptyState.ts` | Unchanged markup/slots (unless a one-line comment pointing at the Less file helps). |
| `frontend/src/views/creatChat/creatChat.vue` | Remove broken `:deep(.dialogue-*)` rules; unscoped-import the new Less; keep suggested-questions styles. |
| `frontend/src/views/creatChat/creatChat.test.ts` | Add layout-contract assertions; keep existing title/slot tests. |

---

### Task 1: Failing layout-contract test

**Files:**
- Modify: `frontend/src/views/creatChat/creatChat.test.ts`
- Test: `frontend/src/views/creatChat/creatChat.test.ts`

**Interfaces:**
- Consumes: existing `readFileSync` / `join(here, ...)` pattern already used for `creatChat.vue` wiring
- Produces: tests that require `newConversationEmptyState.less` to exist and to declare centering; require `creatChat.vue` not to use `:deep(.dialogue-wrap)`

- [ ] **Step 1: Write the failing tests**

Append to `creatChat.test.ts`:

```ts
test('empty-state layout stylesheet owns centering contract', () => {
  const lessPath = join(here, 'newConversationEmptyState.less')
  const less = readFileSync(lessPath, 'utf8')
  assert.match(less, /\.dialogue-wrap\s*\{[^}]*flex:\s*1/s)
  assert.match(less, /\.dialogue-wrap\s*\{[^}]*display:\s*flex/s)
  assert.match(less, /\.dialogue-wrap\s*\{[^}]*justify-content:\s*center/s)
  assert.match(less, /\.dialogue-wrap\s*\{[^}]*align-items:\s*center/s)
  assert.match(less, /\.dialogue-answers\s*\{/s)
  assert.match(less, /\.dialogue-title\s*\{/s)
})

test('creatChat.vue does not reintroduce broken :deep dialogue-wrap layout', () => {
  const source = readFileSync(join(here, 'creatChat.vue'), 'utf8')
  assert.doesNotMatch(source, /:deep\(\s*\.dialogue-wrap\s*\)/)
  assert.doesNotMatch(source, /:deep\(\s*\.dialogue-answers\s*\)/)
  assert.doesNotMatch(source, /:deep\(\s*\.dialogue-title\s*\)/)
  assert.match(source, /newConversationEmptyState\.less/)
})
```

- [ ] **Step 2: Run tests and confirm the new ones fail**

Run:

```bash
cd frontend
npm test -- src/views/creatChat/creatChat.test.ts
```

Expected: existing title/slot tests PASS; new tests FAIL (missing Less file and/or still has `:deep(.dialogue-wrap)`).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/creatChat/creatChat.test.ts
git commit -m "$(cat <<'EOF'
test(新对话): 为空状态居中布局增加回归契约断言

EOF
)"
```

---

### Task 2: Move layout CSS onto the empty-state stylesheet

**Files:**
- Create: `frontend/src/views/creatChat/newConversationEmptyState.less`
- Modify: `frontend/src/views/creatChat/creatChat.vue` (style section only)
- Test: `frontend/src/views/creatChat/creatChat.test.ts`

**Interfaces:**
- Consumes: Task 1 failing assertions
- Produces: runtime styles that apply to `.dialogue-wrap` root without `:deep` descendant mismatch

- [ ] **Step 1: Create `newConversationEmptyState.less`**

Create `frontend/src/views/creatChat/newConversationEmptyState.less` with the exact layout rules currently intended for the empty state (copied from today's `creatChat.vue` `:deep` blocks, without `:deep`):

```less
.dialogue-wrap {
    flex: 1;
    display: flex;
    justify-content: center;
    align-items: center;
}

.dialogue-answers {
    display: flex;
    flex-flow: column;
    align-items: center;
    width: 100%;
    max-width: 960px;
    gap: 24px;

    .answers-input {
        position: static;
        transform: translateX(0);
    }
}

.dialogue-title {
    display: flex;
    color: var(--td-text-color-primary);
    font-family: var(--app-font-family);
    font-size: 28px;
    font-weight: 600;
    align-items: center;
    margin-bottom: 0;

    .icon {
        display: flex;
        width: 32px;
        height: 32px;
        justify-content: center;
        align-items: center;
        border-radius: 6px;
        background: var(--td-bg-color-container);
        box-shadow: var(--td-shadow-1);
        margin-right: 12px;

        .logo_img {
            height: 24px;
            width: 24px;
        }
    }
}
```

- [ ] **Step 2: Wire the Less file from `creatChat.vue` and delete broken `:deep` rules**

In `creatChat.vue`:

1. Delete the entire scoped blocks for `:deep(.dialogue-wrap)`, `:deep(.dialogue-answers)`, and `:deep(.dialogue-title)` (lines currently ~246–293).
2. Keep the remaining scoped styles (suggested-questions, media queries, etc.).
3. Add an **unscoped** style import so Vite injects the shell CSS (place after the scoped block, before or after the existing global `.del-menu-popup` block):

```vue
<style lang="less">
@import './newConversationEmptyState.less';
</style>
```

If the file already has a second `<style lang="less">` for `.del-menu-popup`, either merge the `@import` into that block or add a dedicated third block — both are fine; prefer merging into the existing unscoped block to avoid style-block sprawl:

```vue
<style lang="less">
@import './newConversationEmptyState.less';

.del-menu-popup {
    z-index: 99 !important;
    /* ...existing rules unchanged... */
}
</style>
```

- [ ] **Step 3: Run unit tests**

Run:

```bash
cd frontend
npm test -- src/views/creatChat/creatChat.test.ts
```

Expected: ALL tests PASS, including the new layout-contract tests and the existing title/slot/wiring tests.

- [ ] **Step 4: Manual smoke (same machine / browser)**

1. `npm run dev` in `frontend` (or use the running teaching stack).
2. Open「新对话」as teacher: empty-state block centered in the right pane (not top-left).
3. Open「新对话」as student: same.
4. Optional: open `knowledge-bases/:kbId/creatChat` once.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/creatChat/newConversationEmptyState.less frontend/src/views/creatChat/creatChat.vue
git commit -m "$(cat <<'EOF'
fix(新对话): 将空状态居中样式收回外壳 Less，修复贴左上角

EOF
)"
```

---

### Task 3: Spec status + closeout note

**Files:**
- Modify: `docs/新对话空状态居中规格.md` (status line only)

**Interfaces:**
- Consumes: Task 2 complete + manual smoke
- Produces: doc status reflects implementation

- [ ] **Step 1: Update status banner**

Change the top status line from `待评审` to:

```markdown
> 状态：已实现（方案 A：`newConversationEmptyState.less` + 布局契约测试）
```

- [ ] **Step 2: Commit**

```bash
git add docs/新对话空状态居中规格.md
git commit -m "$(cat <<'EOF'
docs(新对话): 标记空状态居中规格为已实现

EOF
)"
```

---

## Self-review

1. **Spec coverage:** T1→Ticket 2 tests; T2→Ticket 1 fix; T3 doc + Ticket 3 manual steps embedded in Task 2 Step 4.
2. **Placeholders:** none — exact CSS, exact asserts, exact commands.
3. **Consistency:** Less owns `.dialogue-wrap` centering; `creatChat.vue` must not use `:deep(.dialogue-wrap|answers|title)`.
