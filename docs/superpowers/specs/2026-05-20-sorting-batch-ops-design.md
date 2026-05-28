# Sorting Fix & Batch Operations Design

**Goal:** Fix broken frontend sorting and add batch delete/download operations with a floating action bar.

**Approach:** Pure frontend sorting (no backend changes). Batch operations reuse existing single-file APIs with error handling. Floating toolbar at bottom when files are selected.

---

## 1. Sorting Fix (Frontend Only)

### Current Problems

- List view size column sorts on formatted string ("1.5 MB") instead of numeric value, producing wrong results
- Grid view has no sort controls at all
- No sort state in the store — sorting is ad-hoc per component

### Solution

**Store changes (`web/src/stores/files.js`):**

- Add state: `sortBy: 'name'` and `sortOrder: 'asc'`
- Add getter `sortedFiles`: returns `files` sorted by `sortBy`/`sortOrder`
  - `name`: `file.name.localeCompare()` — alphabetical
  - `size`: `file.physical?.size ?? 0` — numeric on raw bytes
  - `updatedAt`: `file.updatedAt` — ISO string comparison
  - Folders always pinned to top; sorting applies only within same type (folder vs file)
- `fetchFiles()` resets `sortBy = 'name'`, `sortOrder = 'asc'`
- Add action `setSort(sortBy, sortOrder)`

**List view (`FileList.vue`):**

- Replace `sortable` attribute with `:sort-method` callback on size and time columns
  - Size column: sort-method compares `row.physical?.size ?? 0`
  - Time column: sort-method compares `row.updatedAt`
- On column sort change, call `filesStore.setSort()` to sync store state

**Grid view (`FileGrid.vue`):**

- Add a sort dropdown in the existing Toolbar component
  - Options: 名称 / 大小 / 修改时间, each with ascending/descending toggle
  - Bound to `filesStore.sortBy` and `filesStore.sortOrder`

**All views:**

- Replace direct `files` references with `sortedFiles` getter for display
- Search results (`searchResults`) remain sorted by the backend `searchSort` parameter — no change

---

## 2. Batch Operations

### Floating Action Bar

**New component: `BatchActionBar.vue`**

- Fixed position at viewport bottom (`position: fixed; bottom: 0; z-index: 100`)
- Visible only when `filesStore.selectedIds.length > 0`
- Content: `已选 N 项` + three buttons: 删除 / 移动到 / 下载 + 取消选择 link
- Slide-up animation on appear (CSS transition)
- Placed in `FilesView.vue`, outside the list/grid area

### Batch Delete

- Click 删除 → `ElMessageBox.confirm('确定删除选中的 N 个文件？')`
- On confirm: `Promise.allSettled(selectedIds.map(id => fileAPI.delete(id)))`
- Count fulfilled vs rejected
- All success: `ElMessage.success('已删除 N 个文件')`
- Partial failure: `ElMessage.warning('成功删除 N 个文件，M 个失败')`
- Always refresh file list once via `fetchFiles(currentFolder)` afterward
- Clear `selectedIds` regardless of outcome

### Batch Move

- Reuse existing `MoveDialog` component, pass all `selectedIds`
- Backend `PATCH /files/move` already accepts `fileIds: []` — no backend change needed
- On success: refresh file list, clear selection

### Batch Download

- Sequential `window.open(downloadUrl, '_blank')` for each selected file
- 300ms interval between downloads to prevent browser popup blocker
- Skip folders or use `downloadFolder()` for folder items
- No backend change — reuses existing single-file and folder download endpoints

---

## Files Changed

| File | Change |
|------|--------|
| `web/src/stores/files.js` | Add `sortBy`, `sortOrder` state, `sortedFiles` getter, `setSort()` action |
| `web/src/components/FileList.vue` | Fix `sort-method` on size/time columns, sync to store |
| `web/src/components/FileGrid.vue` | Use `sortedFiles` instead of `files` |
| `web/src/components/Toolbar.vue` | Add sort dropdown for grid view |
| `web/src/components/BatchActionBar.vue` | New component — floating batch action bar |
| `web/src/views/FilesView.vue` | Include `BatchActionBar`, wire batch operations |

No backend changes required.
