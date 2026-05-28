# Sorting Fix & Batch Operations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix broken frontend file sorting and add batch delete/download/move operations with a floating action bar.

**Architecture:** Frontend-only sorting via Pinia store getter. Batch download via new backend ZIP streaming endpoint. Batch delete uses existing single-file API with `Promise.allSettled` error handling. Floating bottom toolbar appears when files are selected.

**Tech Stack:** Go/Gin (backend), Vue 3 + Pinia + Element Plus (frontend), `archive/zip` (Go stdlib)

---

### Task 1: Add sorting state and getter to files store

**Files:**
- Modify: `web/src/stores/files.js`

- [ ] **Step 1: Add `sortBy`, `sortOrder` state and `sortedFiles` getter**

In `web/src/stores/files.js`, add to the state object (after `path` on line 16):

```js
sortBy: 'name',
sortOrder: 'asc',
```

Add a new getter `sortedFiles` after the existing `filesOnly` getter (line 22):

```js
sortedFiles: (state) => {
  const folders = state.files.filter(f => f.isFolder)
  const filesOnly = state.files.filter(f => !f.isFolder)

  const sortFn = (a, b) => {
    let cmp = 0
    if (state.sortBy === 'name') {
      cmp = a.name.localeCompare(b.name)
    } else if (state.sortBy === 'size') {
      cmp = (a.physical?.size ?? 0) - (b.physical?.size ?? 0)
    } else if (state.sortBy === 'updatedAt') {
      cmp = (a.updatedAt || '').localeCompare(b.updatedAt || '')
    }
    return state.sortOrder === 'asc' ? cmp : -cmp
  }

  return [...folders.sort(sortFn), ...filesOnly.sort(sortFn)]
}
```

- [ ] **Step 2: Reset sort on `fetchFiles`**

In the `fetchFiles` action (line 25), add after `this.isSearching = false` (line 39):

```js
this.sortBy = 'name'
this.sortOrder = 'asc'
```

- [ ] **Step 3: Add `setSort` action**

Add after the `setSelected` action (line 172):

```js
setSort(sortBy, sortOrder) {
  this.sortBy = sortBy
  this.sortOrder = sortOrder
},
```

- [ ] **Step 4: Commit**

```bash
git add web/src/stores/files.js
git commit -m "feat: add sortBy/sortOrder state and sortedFiles getter to files store"
```

---

### Task 2: Fix FileList.vue column sorting

**Files:**
- Modify: `web/src/components/Files/FileList.vue`

- [ ] **Step 1: Replace `sortable` with `:sort-method` on size and time columns**

Replace line 21:

```html
<el-table-column label="大小" width="120" sortable>
```

with:

```html
<el-table-column label="大小" width="120" :sort-method="sortBySize" :sortable="'custom'">
```

Replace line 28:

```html
<el-table-column label="修改时间" width="180" sortable>
```

with:

```html
<el-table-column label="修改时间" width="180" :sort-method="sortByTime" :sortable="'custom'">
```

- [ ] **Step 2: Add sort-method functions and sort-change handler**

In the `<script setup>` section, after `handleSelectionChange` (line 172), add:

```js
function sortBySize(a, b) {
  return (a.physical?.size ?? 0) - (b.physical?.size ?? 0)
}

function sortByTime(a, b) {
  return (a.updatedAt || '').localeCompare(b.updatedAt || '')
}

function handleSortChange({ prop, order }) {
  if (!order) {
    filesStore.setSort('name', 'asc')
  } else if (prop === 'size' || prop === '大小') {
    filesStore.setSort('size', order === 'ascending' ? 'asc' : 'desc')
  } else if (prop === 'time' || prop === '修改时间') {
    filesStore.setSort('updatedAt', order === 'ascending' ? 'asc' : 'desc')
  } else {
    filesStore.setSort('name', order === 'ascending' ? 'asc' : 'desc')
  }
}
```

- [ ] **Step 3: Wire `@sort-change` on el-table**

On the `<el-table>` tag (line 3-9), add `@sort-change="handleSortChange"`:

```html
<el-table
  :data="files"
  @row-dblclick="handleDoubleClick"
  @selection-change="handleSelectionChange"
  @sort-change="handleSortChange"
  row-key="id"
  style="width: 100%"
>
```

- [ ] **Step 4: Commit**

```bash
git add web/src/components/Files/FileList.vue
git commit -m "fix: use sort-method for numeric size and time sorting in FileList"
```

---

### Task 3: Add sort dropdown to Toolbar

**Files:**
- Modify: `web/src/components/Files/Toolbar.vue`

- [ ] **Step 1: Add sort dropdown UI**

In the template, add a sort dropdown between `toolbar-left` and `toolbar-right` divs (after line 11, before line 13):

```html
<div class="toolbar-center">
  <el-dropdown @command="handleSortChange">
    <el-button text>
      <el-icon><Sort /></el-icon>
      {{ sortLabel }}
      <el-icon class="el-icon--right"><ArrowDown /></el-icon>
    </el-button>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item command="name-asc" :class="{ active: filesStore.sortBy === 'name' && filesStore.sortOrder === 'asc' }">名称 A→Z</el-dropdown-item>
        <el-dropdown-item command="name-desc" :class="{ active: filesStore.sortBy === 'name' && filesStore.sortOrder === 'desc' }">名称 Z→A</el-dropdown-item>
        <el-dropdown-item command="size-asc" :class="{ active: filesStore.sortBy === 'size' && filesStore.sortOrder === 'asc' }">大小 小→大</el-dropdown-item>
        <el-dropdown-item command="size-desc" :class="{ active: filesStore.sortBy === 'size' && filesStore.sortOrder === 'desc' }">大小 大→小</el-dropdown-item>
        <el-dropdown-item command="updatedAt-asc" :class="{ active: filesStore.sortBy === 'updatedAt' && filesStore.sortOrder === 'asc' }">时间 旧→新</el-dropdown-item>
        <el-dropdown-item command="updatedAt-desc" :class="{ active: filesStore.sortBy === 'updatedAt' && filesStore.sortOrder === 'desc' }">时间 新→旧</el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</div>
```

- [ ] **Step 2: Add sort logic to script**

Add imports for `Sort` and `ArrowDown` icons. Replace line 45:

```js
import { Upload, FolderAdd, List, Grid } from '@element-plus/icons-vue'
```

with:

```js
import { Upload, FolderAdd, List, Grid, Sort, ArrowDown } from '@element-plus/icons-vue'
```

Add computed and handler after `setViewMode` (after line 75):

```js
import { ref, computed } from 'vue'

const sortLabels = {
  'name-asc': '名称 A→Z',
  'name-desc': '名称 Z→A',
  'size-asc': '大小 小→大',
  'size-desc': '大小 大→小',
  'updatedAt-asc': '时间 旧→新',
  'updatedAt-desc': '时间 新→旧',
}

const sortLabel = computed(() => {
  const key = `${filesStore.sortBy}-${filesStore.sortOrder}`
  return sortLabels[key] || '排序'
})

function handleSortChange(command) {
  const [sortBy, sortOrder] = command.split('-')
  filesStore.setSort(sortBy, sortOrder)
}
```

Also update the import on line 42 from `import { ref } from 'vue'` to `import { ref, computed } from 'vue'`.

- [ ] **Step 3: Add toolbar-center style**

In the `<style>` section, add after `.toolbar-right` (after line 95):

```scss
.toolbar-center {
  display: flex;
  gap: 8px;
}
```

- [ ] **Step 4: Commit**

```bash
git add web/src/components/Files/Toolbar.vue
git commit -m "feat: add sort dropdown to Toolbar with name/size/time options"
```

---

### Task 4: Use sortedFiles in views

**Files:**
- Modify: `web/src/views/FilesView.vue`

- [ ] **Step 1: Replace `filesStore.files` with `filesStore.sortedFiles` in the normal file listing**

In `FilesView.vue`, change lines 61 and 71.

Line 61 — change:
```html
:files="filesStore.files"
```
to:
```html
:files="filesStore.sortedFiles"
```

Line 71 — change:
```html
:files="filesStore.files"
```
to:
```html
:files="filesStore.sortedFiles"
```

- [ ] **Step 2: Commit**

```bash
git add web/src/views/FilesView.vue
git commit -m "feat: use sortedFiles getter for file display in both grid and list views"
```

---

### Task 5: Create BatchActionBar component

**Files:**
- Create: `web/src/components/Files/BatchActionBar.vue`

- [ ] **Step 1: Write the BatchActionBar component**

Create `web/src/components/Files/BatchActionBar.vue`:

```vue
<template>
  <Transition name="slide-up">
    <div v-if="filesStore.selectedIds.length > 0" class="batch-action-bar">
      <span class="batch-info">已选 {{ filesStore.selectedIds.length }} 项</span>

      <el-button type="danger" size="small" @click="emit('batch-delete')">
        <el-icon><Delete /></el-icon>
        删除
      </el-button>

      <el-button type="warning" size="small" @click="emit('batch-move')">
        <el-icon><Right /></el-icon>
        移动到
      </el-button>

      <el-button type="primary" size="small" @click="emit('batch-download')">
        <el-icon><Download /></el-icon>
        下载
      </el-button>

      <span class="batch-cancel" @click="filesStore.setSelected([])">取消选择</span>
    </div>
  </Transition>
</template>

<script setup>
import { useFilesStore } from '@/stores/files'
import { Delete, Right, Download } from '@element-plus/icons-vue'

const filesStore = useFilesStore()

const emit = defineEmits(['batch-delete', 'batch-move', 'batch-download'])
</script>

<style scoped lang="scss">
.batch-action-bar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 24px;
  background: #fff;
  border-top: 1px solid #e4e7ed;
  box-shadow: 0 -2px 12px rgba(0, 0, 0, 0.08);
}

.batch-info {
  font-weight: 600;
  color: #409eff;
  margin-right: 8px;
}

.batch-cancel {
  margin-left: auto;
  color: #909399;
  cursor: pointer;
  font-size: 13px;

  &:hover {
    color: #606266;
  }
}

.slide-up-enter-active,
.slide-up-leave-active {
  transition: transform 0.3s ease, opacity 0.3s ease;
}

.slide-up-enter-from,
.slide-up-leave-to {
  transform: translateY(100%);
  opacity: 0;
}
</style>
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/Files/BatchActionBar.vue
git commit -m "feat: add BatchActionBar floating component with delete/move/download"
```

---

### Task 6: Wire BatchActionBar and batch operations in FilesView

**Files:**
- Modify: `web/src/views/FilesView.vue`

- [ ] **Step 1: Import BatchActionBar**

Add import after the `ConfirmDialog` import (line 143):

```js
import BatchActionBar from '@/components/Files/BatchActionBar.vue'
```

- [ ] **Step 2: Add BatchActionBar to template**

In the template, add after the drag overlay closing `</div>` (line 123), before `</div>` closing the root:

```html
<BatchActionBar
  @batch-delete="handleBatchDelete"
  @batch-move="handleBatchMove"
  @batch-download="handleBatchDownload"
/>
```

- [ ] **Step 3: Add ElMessageBox import**

Replace line 144:

```js
import { ElMessage } from 'element-plus'
```

with:

```js
import { ElMessage, ElMessageBox } from 'element-plus'
```

- [ ] **Step 4: Add batch operation handlers**

Add after the `handleConfirmDelete` function (after line 288):

```js
async function handleBatchDelete() {
  const count = filesStore.selectedIds.length
  try {
    await ElMessageBox.confirm(
      `确定删除选中的 ${count} 个文件？此操作不可恢复`,
      '批量删除',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }

  const results = await Promise.allSettled(
    filesStore.selectedIds.map(id => filesStore.deleteFile(id))
  )
  const succeeded = results.filter(r => r.status === 'fulfilled').length
  const failed = results.filter(r => r.status === 'rejected').length

  if (failed === 0) {
    ElMessage.success(`已删除 ${succeeded} 个文件`)
  } else {
    const failedIds = results
      .map((r, i) => r.status === 'rejected' ? filesStore.selectedIds[i] : null)
      .filter(Boolean)
    console.error('批量删除失败，文件ID:', failedIds)
    ElMessage.warning(`成功删除 ${succeeded} 个文件，${failed} 个失败`)
  }

  await filesStore.fetchFiles(filesStore.currentFolder)
  filesStore.setSelected([])
}

function handleBatchMove() {
  const selected = filesStore.sortedFiles.filter(f => filesStore.selectedIds.includes(f.id))
  selectedFiles.value = selected
  showMove.value = true
}

function handleBatchDownload() {
  filesStore.batchDownload()
}
```

- [ ] **Step 5: Commit**

```bash
git add web/src/views/FilesView.vue
git commit -m "feat: wire BatchActionBar with batch delete/move/download handlers"
```

---

### Task 7: Add batchDownload action to files store

**Files:**
- Modify: `web/src/stores/files.js`

- [ ] **Step 1: Add batchDownload action**

Add after the `downloadFolder` action (after line 163):

```js
async batchDownload() {
  const token = localStorage.getItem('cloudbox_token')
  if (!token) {
    console.error('No token found, please login first')
    return
  }

  const ids = this.selectedIds.join(',')
  const url = `/api/files/download?ids=${ids}`
  try {
    const response = await fetch(url, {
      headers: { Authorization: `Bearer ${token}` }
    })

    const contentType = response.headers.get('content-type')
    if (contentType && contentType.includes('application/json')) {
      const data = await response.json()
      if (data.code !== 0) {
        console.error('Batch download failed:', data.message)
        return
      }
    }

    if (response.ok) {
      const blob = await response.blob()
      const disposition = response.headers.get('Content-Disposition')
      let filename = 'downloads.zip'
      if (disposition) {
        const parts = disposition.split("'")
        if (parts.length >= 3) {
          filename = decodeURIComponent(parts[2])
        } else {
          const match = disposition.match(/filename="?([^;"']+)/)
          if (match) filename = match[1]
        }
      }
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = filename
      a.click()
      URL.revokeObjectURL(a.href)
    }
  } catch (err) {
    console.error('Batch download error:', err)
  }
},
```

- [ ] **Step 2: Commit**

```bash
git add web/src/stores/files.js
git commit -m "feat: add batchDownload action with fetch+blob ZIP download"
```

---

### Task 8: Add backend BatchDownload service method

**Files:**
- Modify: `internal/service/file.go`

- [ ] **Step 1: Add `StreamBatchZip` method**

Add after `StreamFolderZip` (after line 501):

```go
func (s *FileService) StreamBatchZip(ctx context.Context, userID int64, fileIDs []int64, writer io.Writer) error {
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	for _, id := range fileIDs {
		file, err := s.fileRepo.FindByIDAndOwner(ctx, id, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return fmt.Errorf("failed to find file %d: %w", id, err)
		}

		if file.DeletedAt.Valid {
			continue
		}

		if file.IsFolder {
			if err := s.addFolderToZip(ctx, zipWriter, file, file.Name); err != nil {
				log.Printf("warning: failed to add folder %s to batch zip: %v", file.Name, err)
				continue
			}
		} else if file.ContentRef != 0 {
			pf, err := s.physicalRepo.FindByID(ctx, file.ContentRef)
			if err != nil {
				log.Printf("warning: failed to find physical file %d: %v", file.ContentRef, err)
				continue
			}
			absPath := s.storage.ToAbsPath(pf.StoragePath)
			if err := s.addFileToZip(zipWriter, absPath, file.Name); err != nil {
				log.Printf("warning: failed to add file %s to batch zip: %v", file.Name, err)
				continue
			}
		}
	}

	return nil
}
```

Note: This reuses the existing `addFolderToZip` and `addFileToZip` helper methods. For batch download, each selected item is at the ZIP root using its own name — this matches the spec's path structure where top-level items retain their names, and folders recursively include their contents.

- [ ] **Step 2: Commit**

```bash
git add internal/service/file.go
git commit -m "feat: add StreamBatchZip service method for multi-file ZIP download"
```

---

### Task 9: Add backend BatchDownload handler and route

**Files:**
- Modify: `internal/handler/file.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add `BatchDownload` handler**

Add after `DownloadFolder` (after line 386) in `internal/handler/file.go`:

```go
func (h *FileHandler) BatchDownload(c *gin.Context) {
	userID := GetUserID(c)

	idsStr := c.Query("ids")
	if idsStr == "" {
		response.Error(c, 400, "ids parameter is required")
		return
	}

	parts := strings.Split(idsStr, ",")
	var fileIDs []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			response.Error(c, 400, "invalid file id: "+p)
			return
		}
		fileIDs = append(fileIDs, id)
	}

	if len(fileIDs) == 0 {
		response.Error(c, 400, "no valid file ids provided")
		return
	}

	c.Header("Content-Disposition", "attachment; filename*=UTF-8''downloads.zip")
	c.Header("Content-Type", "application/zip")

	if err := h.fileService.StreamBatchZip(c.Request.Context(), userID, fileIDs, c.Writer); err != nil {
		log.Printf("error streaming batch zip: %v", err)
	}
}
```

Also ensure `strings` is imported at the top of `internal/handler/file.go` (it's already imported — verify it exists).

- [ ] **Step 2: Register the route in main.go**

In `cmd/server/main.go`, add the batch download route **before** the `/:id` routes. After line 98 (`protected.GET("/files/search", fileHandler.SearchFiles)`), add:

```go
			protected.GET("/files/download", fileHandler.BatchDownload)  // Must be before /:id routes
```

- [ ] **Step 3: Verify the Go code compiles**

Run: `cd E:/fileManagementIntranet && go build ./cmd/server`
Expected: success (no errors)

- [ ] **Step 4: Commit**

```bash
git add internal/handler/file.go cmd/server/main.go
git commit -m "feat: add BatchDownload handler and route for multi-file ZIP download"
```

---

### Task 10: Build frontend and test end-to-end

**Files:**
- Modify: `static/` (rebuilt from source)

- [ ] **Step 1: Rebuild frontend**

Run: `cd E:/fileManagementIntranet && make build-frontend`
Expected: build succeeds

- [ ] **Step 2: Verify Go build with new static files**

Run: `cd E:/fileManagementIntranet && go build ./cmd/server`
Expected: build succeeds

- [ ] **Step 3: Commit rebuilt static files**

```bash
git add -f static/
git commit -m "chore: rebuild frontend static files"
```

- [ ] **Step 4: Start server and manually test**

Run: `cd E:/fileManagementIntranet && go run ./cmd/server`

Test checklist:
1. Open browser, login
2. **Sort by name/size/time** via Toolbar dropdown — verify files reorder correctly
3. **List view column sort** — click size/time column headers, verify numeric sort works
4. **Select multiple files** — verify bottom action bar appears
5. **Batch delete** — select 2+ files, click delete, confirm, verify success message and list refresh
6. **Batch move** — select 2+ files, click move, verify MoveDialog opens with all selected files
7. **Batch download** — select 2+ files, click download, verify ZIP file downloads with correct contents
8. **Cancel selection** — click "取消选择", verify action bar disappears

---

## Self-Review

**Spec coverage:**
- Sorting: sortBy/sortOrder state ✓, sortedFiles getter ✓, FileList sort-method ✓, Toolbar dropdown ✓, views use sortedFiles ✓
- Batch action bar: BatchActionBar component ✓, fixed bottom position ✓, slide-up animation ✓
- Batch delete: Promise.allSettled ✓, error counting ✓, failed filenames logged ✓, "不可恢复" in confirm ✓, list refresh ✓
- Batch move: reuses MoveDialog ✓, passes selectedFiles ✓
- Batch download: backend ZIP endpoint ✓, GET /files/download?ids= ✓, owner_id + deleted_at validation ✓, streaming ZIP ✓, fetch+blob on frontend ✓

**Placeholder scan:** No TBD/TODO found. All code steps have complete implementations.

**Type consistency:** `sortedFiles` getter referenced consistently in FilesView.vue. `filesStore.setSort(sortBy, sortOrder)` signature matches usage in FileList and Toolbar. `fileIDs []int64` matches `BatchUpdateParent` pattern. `StreamBatchZip` reuses existing `addFolderToZip`/`addFileToZip`.
