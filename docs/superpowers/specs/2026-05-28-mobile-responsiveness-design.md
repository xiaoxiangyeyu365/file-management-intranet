# Mobile Responsiveness Design

**Goal:** Adapt CloudBox UI for mobile screens (<768px) with a bottom tab bar, mobile file card list, and responsive fixes across all views.

**Breakpoint:** 768px (Element Plus standard). Below 768px = mobile layout.

---

## 1. Global Infrastructure

**`web/src/styles/main.scss`:**
- Add breakpoint mixin: `@mixin mobile { @media (max-width: 767px) { @content; } }`
- All mobile overrides use this mixin

**Responsive detection composable (`web/src/composables/useResponsive.js`):**
- Returns `isMobile` ref based on `window.matchMedia('(max-width: 767px)')`
- Listens to `change` event, removes listener on component unmount (prevent memory leak)
- Used in FilesView and any component needing layout-level mobile detection

---

## 2. Sidebar → Bottom Tab Bar

**AppSidebar.vue:**
- `@include mobile { display: none }` — hidden entirely on mobile

**New component: `web/src/components/Layout/MobileTabBar.vue`:**
- Fixed at viewport bottom: `position: fixed; bottom: 0; left: 0; right: 0; z-index: 100`
- Height: 56px
- 4 tabs with icon + label:
  - 文件 (Document icon) → `/`
  - 剪贴板 (Clipboard icon) → `/clipboard`
  - 回收站 (Delete icon) → `/trash`
  - 管理 (Setting icon) → `/admin` — **only rendered when `userStore.role === 'admin'`**
- Active tab highlighted based on current route
- Tab click navigates via `router.push()`
- Hidden when BatchActionBar is visible (mutual exclusion)

**AppHeader.vue:**
- Mobile: hide desktop nav items, show simplified title ("CloudBox")
- Keep user dropdown (compact icon-only trigger on mobile)

---

## 3. Mobile File List

**New component: `web/src/components/Files/MobileFileList.vue`:**

**Interaction state machine:**

| State | Tap folder | Tap file | Long-press any | Tap right dot |
|-------|-----------|----------|----------------|---------------|
| Default | Navigate into | Preview/download | Enter multi-select mode, toggle item | Toggle item selection |
| Multi-select | — | — | — | Toggle item selection |

- **Default mode:** Tap navigates folders / previews files. Long-press enters multi-select mode and toggles that item's selection.
- **Multi-select mode:** All items show circular checkboxes. Tapping toggles selection (no navigation). BatchActionBar appears.
- **Right dot:** Small circle icon on each item's right side. Single tap toggles that item's selection without entering multi-select mode. If item becomes selected and BatchActionBar appears, user is effectively in multi-select mode.
- **Exit multi-select:** BatchActionBar's "取消选择" clears all selectedIds and returns to default mode.

**Card layout per item:**
- Left: file type icon (reuse `getFileIcon` logic)
- Center: file name (bold), below it size or modified time (whichever is more relevant — size for files, time for folders)
- Right: small circular selection dot icon, plus `...` more button for context menu
- Long-press detection: 500ms hold via `touchstart`/`touchend` timer

**Breadcrumb:**
- Rendered above the file list, below the header
- `overflow-x: auto; white-space: nowrap` — horizontally scrollable
- Same Breadcrumb component used on desktop, just styled for mobile

---

## 4. Toolbar Adaptation

**Toolbar.vue mobile changes:**
- Buttons show icon only, hide text labels ("上传" → just upload icon)
- **Hide view mode toggle** (grid/list) — not needed on mobile
- Sort dropdown remains (dropdown is a floating overlay, works on narrow screens)

---

## 5. BatchActionBar Adaptation

- `flex-wrap: wrap` on mobile so buttons wrap to second line if needed
- When BatchActionBar is visible: **hide MobileTabBar** (mutual exclusion)
- When selection is cleared: MobileTabBar reappears
- BatchActionBar stays at `bottom: 0` (same position as TabBar would be)

---

## 6. FilesView Layout Changes

**`web/src/views/FilesView.vue`:**
- Use `useResponsive()` composable for `isMobile`
- `isMobile && !isSearching`: render MobileFileList + Breadcrumb instead of FileGrid/FileList
- `isMobile`: render MobileTabBar instead of AppSidebar
- Fix: on mobile, remove `margin-right: var(--upload-panel-width)` from `.files-main.with-panel`
- Reduce padding: `padding: 0 12px 12px` on mobile (vs 24px desktop)

---

## 7. LoginView Fix

**`web/src/views/LoginView.vue`:**
- Add `max-width: calc(100% - 32px)` to `.login-box` so it never overflows narrow viewports

---

## 8. UploadPanel Fix

**`web/src/views/FilesView.vue`:**
- On mobile, do not apply `margin-right` when upload panel is active (UploadPanel already goes full-screen on mobile via its own media query)

---

## Files Changed

| File | Change |
|------|--------|
| `web/src/styles/main.scss` | Add `@mixin mobile` breakpoint |
| `web/src/composables/useResponsive.js` | New — matchMedia-based `isMobile` ref with cleanup |
| `web/src/components/Layout/AppSidebar.vue` | Hide on mobile via mixin |
| `web/src/components/Layout/AppHeader.vue` | Simplify on mobile |
| `web/src/components/Layout/MobileTabBar.vue` | New — bottom tab bar with admin-gated "管理" tab |
| `web/src/components/Files/MobileFileList.vue` | New — card list with long-press multi-select |
| `web/src/components/Files/Toolbar.vue` | Icon-only buttons, hide view toggle on mobile |
| `web/src/components/Files/BatchActionBar.vue` | flex-wrap on mobile, mutual exclusion with TabBar |
| `web/src/views/FilesView.vue` | isMobile conditional rendering, margin fix, padding fix |
| `web/src/views/LoginView.vue` | max-width overflow fix |

Deferred: Admin page table overflow (next iteration).
