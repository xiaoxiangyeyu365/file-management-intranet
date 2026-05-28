# Mobile Responsiveness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adapt CloudBox UI for mobile screens (<768px) with a bottom tab bar, mobile file card list, and responsive fixes.

**Architecture:** CSS mixin for breakpoint, Vue composable for JS-level mobile detection, new MobileTabBar and MobileFileList components rendered conditionally in FilesView based on viewport width.

**Tech Stack:** Vue 3 + Pinia + Element Plus, SCSS mixins, `window.matchMedia` API

---

### Task 1: Add mobile breakpoint mixin to global styles

**Files:**
- Modify: `web/src/styles/main.scss`

- [ ] **Step 1: Add mobile mixin after the `:root` variables block**

After line 13 (closing `}` of `:root`), add:

```scss
// Mobile breakpoint mixin
@mixin mobile {
  @media (max-width: 767px) {
    @content;
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/styles/main.scss
git commit -m "feat: add mobile breakpoint mixin to global styles"
```

---

### Task 2: Create useResponsive composable

**Files:**
- Create: `web/src/composables/useResponsive.js`

- [ ] **Step 1: Create composable directory and file**

Create `web/src/composables/useResponsive.js`:

```js
import { ref, onMounted, onUnmounted } from 'vue'

export function useResponsive() {
  const isMobile = ref(false)

  let mediaQuery = null
  let handler = null

  onMounted(() => {
    mediaQuery = window.matchMedia('(max-width: 767px)')
    isMobile.value = mediaQuery.matches

    handler = (e) => {
      isMobile.value = e.matches
    }
    mediaQuery.addEventListener('change', handler)
  })

  onUnmounted(() => {
    if (mediaQuery && handler) {
      mediaQuery.removeEventListener('change', handler)
    }
  })

  return { isMobile }
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/composables/useResponsive.js
git commit -m "feat: add useResponsive composable with matchMedia and cleanup"
```

---

### Task 3: Hide AppSidebar on mobile

**Files:**
- Modify: `web/src/components/Layout/AppSidebar.vue`

- [ ] **Step 1: Add mobile hide rule**

At the end of the `<style>` block (after line 96), add:

```scss
@include mobile {
  display: none;
}
```

Note: The `<style>` tag is `scoped lang="scss"` so the mixin from `main.scss` may not be available. To use the global mixin, change `<style scoped lang="scss">` to `<style lang="scss">` and scope manually, OR add the media query inline:

```scss
@media (max-width: 767px) {
  .app-sidebar {
    display: none;
  }
}
```

Add this at the end of the `<style>` block (inside the scoped styles, after `.nav-item` rules).

- [ ] **Step 2: Commit**

```bash
git add web/src/components/Layout/AppSidebar.vue
git commit -m "feat: hide AppSidebar on mobile screens"
```

---

### Task 4: Create MobileTabBar component

**Files:**
- Create: `web/src/components/Layout/MobileTabBar.vue`

- [ ] **Step 1: Create the component**

Create `web/src/components/Layout/MobileTabBar.vue`:

```vue
<template>
  <div v-if="visible" class="mobile-tab-bar">
    <router-link
      to="/"
      class="tab-item"
      :class="{ active: route.path === '/' }"
    >
      <el-icon><Folder /></el-icon>
      <span>文件</span>
    </router-link>

    <router-link
      to="/clipboard"
      class="tab-item"
      :class="{ active: route.path === '/clipboard' }"
    >
      <el-icon><Document /></el-icon>
      <span>剪贴板</span>
    </router-link>

    <router-link
      to="/trash"
      class="tab-item"
      :class="{ active: route.path === '/trash' }"
    >
      <el-icon><Delete /></el-icon>
      <span>回收站</span>
    </router-link>

    <router-link
      v-if="authStore.isAdmin"
      to="/admin/users"
      class="tab-item"
      :class="{ active: route.path.startsWith('/admin') }"
    >
      <el-icon><Setting /></el-icon>
      <span>管理</span>
    </router-link>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useFilesStore } from '@/stores/files'
import { Folder, Document, Delete, Setting } from '@element-plus/icons-vue'

const route = useRoute()
const authStore = useAuthStore()
const filesStore = useFilesStore()

const visible = computed(() => filesStore.selectedIds.length === 0)
</script>

<style scoped lang="scss">
.mobile-tab-bar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 100;
  height: 56px;
  background: #fff;
  border-top: 1px solid #e4e7ed;
  display: flex;
  align-items: center;
  justify-content: space-around;
}

.tab-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  text-decoration: none;
  color: #909399;
  font-size: 10px;
  padding: 4px 12px;
  transition: color 0.2s;

  .el-icon {
    font-size: 22px;
  }

  &.active {
    color: #409eff;
  }
}

@media (min-width: 768px) {
  .mobile-tab-bar {
    display: none;
  }
}
</style>
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/Layout/MobileTabBar.vue
git commit -m "feat: add MobileTabBar bottom navigation with admin-gated management tab"
```

---

### Task 5: Create MobileFileList component

**Files:**
- Create: `web/src/components/Files/MobileFileList.vue`

- [ ] **Step 1: Create the component**

Create `web/src/components/Files/MobileFileList.vue`:

```vue
<template>
  <div class="mobile-file-list">
    <div
      v-for="file in files"
      :key="file.id"
      class="file-card"
      :class="{ selected: isSelected(file.id) }"
      @click="handleClick(file)"
      @touchstart.prevent="onTouchStart(file, $event)"
      @touchend="onTouchEnd"
      @touchmove="onTouchMove"
      @contextmenu.prevent="showContextMenu($event, file)"
    >
      <span class="file-icon">{{ getFileIcon(file) }}</span>

      <div class="file-info">
        <div class="file-name">{{ file.name }}</div>
        <div class="file-meta">
          <span v-if="file.isFolder">文件夹</span>
          <span v-else>{{ formatSize(file.physical?.size || 0) }}</span>
          <span v-if="!file.isFolder && file.updatedAt" class="meta-sep">·</span>
          <span v-if="file.updatedAt && file.updatedAt !== '0001-01-01T00:00:00Z'">{{ formatDate(file.updatedAt) }}</span>
        </div>
      </div>

      <div class="file-actions">
        <el-icon
          v-if="isMultiSelectMode"
          class="select-check"
          :class="{ checked: isSelected(file.id) }"
        >
          <component :is="isSelected(file.id) ? 'CircleCheckFilled' : 'CircleCheck'" />
        </el-icon>
        <el-icon
          v-else
          class="select-dot"
          :class="{ active: isSelected(file.id) }"
          @click.stop="toggleSelect(file.id)"
        >
          <CircleCheck />
        </el-icon>
        <el-icon class="more-btn" @click.stop="showContextMenu($event, file)">
          <MoreFilled />
        </el-icon>
      </div>
    </div>

    <!-- Context Menu -->
    <teleport to="body">
      <div
        v-if="contextMenuVisible"
        class="context-menu"
        :style="{ left: contextMenuPos.x + 'px', top: contextMenuPos.y + 'px' }"
        @click.stop
      >
        <div class="context-menu-item" @click="handleRename">
          <el-icon><Edit /></el-icon>
          <span>重命名</span>
        </div>
        <div class="context-menu-item" @click="handleDownload">
          <el-icon><Download /></el-icon>
          <span>下载</span>
        </div>
        <div class="context-menu-item" @click="handleMove">
          <el-icon><Right /></el-icon>
          <span>移动</span>
        </div>
        <div class="context-menu-item danger" @click="handleDelete">
          <el-icon><Delete /></el-icon>
          <span>删除</span>
        </div>
      </div>
    </teleport>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useFilesStore } from '@/stores/files'
import { formatSize, formatDate } from '@/utils/format'
import { MoreFilled, Edit, Download, Right, Delete, CircleCheck, CircleCheckFilled } from '@element-plus/icons-vue'

const props = defineProps({
  files: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['open', 'preview', 'rename', 'download', 'move', 'delete'])

const filesStore = useFilesStore()

// Multi-select state
const isMultiSelectMode = ref(false)

function isSelected(id) {
  return filesStore.selectedIds.includes(id)
}

function toggleSelect(id) {
  const ids = [...filesStore.selectedIds]
  const idx = ids.indexOf(id)
  if (idx >= 0) {
    ids.splice(idx, 1)
  } else {
    ids.push(id)
  }
  filesStore.setSelected(ids)
  if (ids.length === 0) {
    isMultiSelectMode.value = false
  }
}

// Click handling
function handleClick(file) {
  if (isMultiSelectMode.value) {
    toggleSelect(file.id)
    return
  }
  if (file.isFolder) {
    emit('open', file)
  } else {
    emit('preview', file)
  }
}

// Long-press detection
let longPressTimer = null
let longPressTriggered = false
let touchStartPos = { x: 0, y: 0 }

function onTouchStart(file, event) {
  longPressTriggered = false
  const touch = event.touches[0]
  touchStartPos = { x: touch.clientX, y: touch.clientY }
  longPressTimer = setTimeout(() => {
    longPressTriggered = true
    isMultiSelectMode.value = true
    toggleSelect(file.id)
  }, 500)
}

function onTouchEnd() {
  if (longPressTimer) {
    clearTimeout(longPressTimer)
    longPressTimer = null
  }
}

function onTouchMove(event) {
  if (!longPressTimer) return
  const touch = event.touches[0]
  const dx = Math.abs(touch.clientX - touchStartPos.x)
  const dy = Math.abs(touch.clientY - touchStartPos.y)
  if (dx > 10 || dy > 10) {
    clearTimeout(longPressTimer)
    longPressTimer = null
  }
}

// Context menu
const contextMenuVisible = ref(false)
const contextMenuFile = ref(null)
const contextMenuPos = ref({ x: 0, y: 0 })

function showContextMenu(event, file) {
  contextMenuFile.value = file
  contextMenuPos.value = { x: event.clientX, y: event.clientY }
  contextMenuVisible.value = true
}

function hideContextMenu() {
  contextMenuVisible.value = false
  contextMenuFile.value = null
}

onMounted(() => {
  document.addEventListener('click', hideContextMenu)
})

onUnmounted(() => {
  document.removeEventListener('click', hideContextMenu)
})

function handleRename() {
  if (contextMenuFile.value) emit('rename', contextMenuFile.value)
  hideContextMenu()
}

function handleDownload() {
  if (contextMenuFile.value) emit('download', contextMenuFile.value)
  hideContextMenu()
}

function handleMove() {
  if (contextMenuFile.value) emit('move', contextMenuFile.value)
  hideContextMenu()
}

function handleDelete() {
  if (contextMenuFile.value) emit('delete', contextMenuFile.value)
  hideContextMenu()
}

function getFileIcon(file) {
  if (file.isFolder) return '📁'
  const ext = file.name.split('.').pop().toLowerCase()
  if (['jpg', 'jpeg', 'png', 'gif', 'bmp', 'webp'].includes(ext)) return '🖼️'
  if (['mp4', 'avi', 'mkv', 'mov'].includes(ext)) return '🎬'
  if (['mp3', 'wav', 'flac', 'aac'].includes(ext)) return '🎵'
  if (['doc', 'docx', 'pdf', 'txt'].includes(ext)) return '📄'
  if (['xls', 'xlsx', 'csv'].includes(ext)) return '📊'
  if (['zip', 'rar', '7z', 'tar'].includes(ext)) return '📦'
  return '📄'
}
</script>

<style scoped lang="scss">
.mobile-file-list {
  padding-bottom: 56px; // Space for MobileTabBar
}

.file-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid #f0f2f5;
  transition: background 0.15s;

  &:active {
    background: #f5f7fa;
  }

  &.selected {
    background: #ecf5ff;
  }

  .file-icon {
    font-size: 32px;
    flex-shrink: 0;
  }

  .file-info {
    flex: 1;
    min-width: 0;

    .file-name {
      font-size: 15px;
      font-weight: 500;
      color: #303133;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .file-meta {
      font-size: 12px;
      color: #909399;
      margin-top: 2px;

      .meta-sep {
        margin: 0 4px;
      }
    }
  }

  .file-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;

    .select-dot {
      font-size: 20px;
      color: #c0c4cc;
      cursor: pointer;

      &.active {
        color: #409eff;
      }
    }

    .select-check {
      font-size: 22px;
      color: #c0c4cc;
      cursor: pointer;

      &.checked {
        color: #409eff;
      }
    }

    .more-btn {
      font-size: 18px;
      color: #909399;
      cursor: pointer;
      padding: 4px;
    }
  }
}

.context-menu {
  position: fixed;
  z-index: 9999;
  background: white;
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  padding: 4px 0;
  min-width: 120px;

  .context-menu-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 16px;
    cursor: pointer;
    transition: background 0.2s;

    &:hover {
      background: #f5f7fa;
    }

    &.danger {
      color: #f56c6c;
    }
  }
}

@media (min-width: 768px) {
  .mobile-file-list {
    display: none;
  }
}
</style>
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/Files/MobileFileList.vue
git commit -m "feat: add MobileFileList card component with long-press multi-select"
```

---

### Task 6: Adapt Toolbar for mobile

**Files:**
- Modify: `web/src/components/Files/Toolbar.vue`

- [ ] **Step 1: Add mobile styles to hide text and view toggle**

At the end of the `<style>` section, add:

```scss
@media (max-width: 767px) {
  .toolbar-left {
    .el-button span:not(.el-icon) {
      display: none;
    }
  }

  .toolbar-right {
    display: none;
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/Files/Toolbar.vue
git commit -m "feat: hide button text and view toggle on mobile Toolbar"
```

---

### Task 7: Adapt BatchActionBar for mobile

**Files:**
- Modify: `web/src/components/Files/BatchActionBar.vue`

- [ ] **Step 1: Add mobile styles for flex-wrap and viewport-only visibility**

At the end of the `<style>` section, add:

```scss
@media (max-width: 767px) {
  .batch-action-bar {
    flex-wrap: wrap;
    gap: 8px;
    padding: 8px 12px;

    .batch-cancel {
      flex-basis: 100%;
      text-align: center;
      margin-left: 0;
      margin-top: 4px;
    }
  }
}

@media (min-width: 768px) {
  .batch-action-bar {
    // Desktop-only, MobileTabBar handles mobile
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/Files/BatchActionBar.vue
git commit -m "feat: add mobile flex-wrap styles to BatchActionBar"
```

---

### Task 8: Adapt AppHeader for mobile

**Files:**
- Modify: `web/src/components/Layout/AppHeader.vue`

- [ ] **Step 1: Add mobile styles to simplify header**

At the end of the `<style>` section, add:

```scss
@media (max-width: 767px) {
  .app-header {
    padding: 0 12px;

    .header-left .logo .logo-text {
      font-size: 16px;
    }

    .header-right .user-dropdown span:not(.el-icon) {
      display: none;
    }
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/Layout/AppHeader.vue
git commit -m "feat: simplify AppHeader on mobile — smaller title, icon-only user dropdown"
```

---

### Task 9: Wire MobileTabBar and MobileFileList in FilesView

**Files:**
- Modify: `web/src/views/FilesView.vue`

- [ ] **Step 1: Import new components and composable**

Add these imports in the `<script setup>` section:

After `import BatchActionBar from '@/components/Files/BatchActionBar.vue'` add:
```js
import MobileTabBar from '@/components/Layout/MobileTabBar.vue'
import MobileFileList from '@/components/Files/MobileFileList.vue'
import { useResponsive } from '@/composables/useResponsive'
```

Add composable usage after `const uploadStore = useUploadStore()`:
```js
const { isMobile } = useResponsive()
```

- [ ] **Step 2: Add MobileTabBar to template**

After the `<BatchActionBar ... />` element, add:
```html
<MobileTabBar />
```

- [ ] **Step 3: Add mobile file list to template**

In the `<template v-else>` block (normal file listing, around lines 58-79), add a MobileFileList option. Replace the existing block:

```html
<template v-else>
  <MobileFileList
    v-if="isMobile"
    :files="filesStore.sortedFiles"
    @open="handleOpenFolder"
    @preview="handlePreviewFile"
    @download="handleDownloadFile"
    @rename="handleRename"
    @move="handleMove"
    @delete="handleDelete"
  />
  <template v-else>
    <FileGrid
      v-if="filesStore.viewMode === 'grid'"
      :files="filesStore.sortedFiles"
      @open="handleOpenFolder"
      @preview="handlePreviewFile"
      @download="handleDownloadFile"
      @rename="handleRename"
      @move="handleMove"
      @delete="handleDelete"
    />
    <FileList
      v-else
      :files="filesStore.sortedFiles"
      @open="handleOpenFolder"
      @preview="handlePreviewFile"
      @download="handleDownloadFile"
      @rename="handleRename"
      @move="handleMove"
      @delete="handleDelete"
    />
  </template>
</template>
```

- [ ] **Step 4: Add mobile styles**

At the end of the `<style>` section, add:

```scss
@media (max-width: 767px) {
  .files-main {
    padding: 0 12px 12px;

    &.with-panel {
      margin-right: 0;
    }
  }

  .header-actions {
    flex-wrap: wrap;
    gap: 4px;
  }
}
```

- [ ] **Step 5: Commit**

```bash
git add web/src/views/FilesView.vue
git commit -m "feat: wire MobileTabBar and MobileFileList with isMobile conditional rendering"
```

---

### Task 10: Fix LoginView overflow on narrow screens

**Files:**
- Modify: `web/src/views/LoginView.vue`

- [ ] **Step 1: Add max-width to .login-box**

In the `.login-box` style (line 123-128), add `max-width: calc(100% - 32px)`:

```scss
.login-box {
  width: 360px;
  max-width: calc(100% - 32px);
  padding: 40px;
  background: white;
  border-radius: 12px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/views/LoginView.vue
git commit -m "fix: prevent LoginView overflow on narrow viewports"
```

---

### Task 11: Build frontend and test

**Files:**
- Modify: `static/` (rebuilt from source)

- [ ] **Step 1: Rebuild frontend**

Run: `cd E:/fileManagementIntranet && make build-frontend`
Expected: build succeeds

- [ ] **Step 2: Verify Go build**

Run: `cd E:/fileManagementIntranet && go build ./cmd/server`
Expected: build succeeds

- [ ] **Step 3: Commit rebuilt static files**

```bash
git add -f static/
git commit -m "chore: rebuild frontend static files"
```

- [ ] **Step 4: Manual testing checklist**

Start server: `cd E:/fileManagementIntranet && go run ./cmd/server`

Test on desktop (browser width > 768px):
- Sidebar visible, FileGrid/FileList rendered, MobileTabBar hidden, MobileFileList hidden

Test on mobile (use Chrome DevTools device emulation, e.g. iPhone 12):
- Sidebar hidden, MobileTabBar visible at bottom with 4 tabs (3 if non-admin)
- MobileFileList renders card-style file items
- Tap folder → navigate into it
- Tap file → preview/download
- Long-press item → enter multi-select, checkboxes appear
- Tap right circle dot → toggle selection without entering multi-select
- Select items → BatchActionBar appears at bottom, MobileTabBar hides
- Cancel selection → BatchActionBar hides, MobileTabBar reappears
- Toolbar shows icon-only buttons, view toggle hidden
- Login page fits narrow screens without horizontal scroll

---

## Self-Review

**Spec coverage:**
- Breakpoint mixin ✓ (Task 1)
- useResponsive composable with cleanup ✓ (Task 2)
- AppSidebar hidden on mobile ✓ (Task 3)
- MobileTabBar with admin-gated "管理" ✓ (Task 4)
- MobileFileList with long-press state machine ✓ (Task 5)
- Toolbar icon-only + hidden view toggle ✓ (Task 6)
- BatchActionBar flex-wrap + mutual exclusion with TabBar ✓ (Task 7)
- AppHeader simplified on mobile ✓ (Task 8)
- FilesView conditional rendering + margin fix + padding ✓ (Task 9)
- LoginView max-width ✓ (Task 10)
- Breadcrumb rendered in FilesView (already present above file list) ✓

**Placeholder scan:** No TBD/TODO found. All code steps have complete implementations.

**Type consistency:** `isMobile` ref from `useResponsive()` used consistently in FilesView. `isSelected(id)` in MobileFileList uses `filesStore.selectedIds`. `authStore.isAdmin` used in MobileTabBar matching AppSidebar pattern. `filesStore.sortedFiles` used in MobileFileList binding.
