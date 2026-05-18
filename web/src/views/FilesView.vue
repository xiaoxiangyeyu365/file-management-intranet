<template>
  <div class="files-view" @dragover.prevent @drop.prevent="handleDrop">
    <AppHeader />

    <div class="files-layout">
      <AppSidebar />

      <main class="files-main" :class="{ 'with-panel': uploadStore.totalCount > 0 }">
        <div class="main-header">
          <Breadcrumb :path="filesStore.path" @navigate="handleNavigate" />

          <div class="header-actions">
            <Toolbar @create-folder="showCreateFolder = true" />

            <SearchBar v-if="!filesStore.isSearching" />
            <el-button v-else text @click="clearSearch">
              <el-icon><Close /></el-icon>
              清除搜索
            </el-button>
          </div>
        </div>

        <div class="files-content">
          <div v-if="filesStore.loading" class="loading-state">
            <el-icon class="is-loading"><Loading /></el-icon>
            加载中...
          </div>

          <template v-else-if="filesStore.isSearching">
            <div class="search-results">
              <div class="results-header">
                搜索结果: "{{ filesStore.searchKeyword }}"
                <span class="results-count">({{ filesStore.searchResults.length }} 个文件)</span>
              </div>
              <FileGrid
                v-if="filesStore.viewMode === 'grid'"
                :files="filesStore.searchResults"
                @open="handleOpenFolder"
                @preview="handlePreviewFile"
                @download="handleDownloadFile"
                @rename="handleRename"
                @move="handleMove"
                @delete="handleDelete"
              />
              <FileList
                v-else
                :files="filesStore.searchResults"
                @open="handleOpenFolder"
                @preview="handlePreviewFile"
                @download="handleDownloadFile"
                @rename="handleRename"
                @move="handleMove"
                @delete="handleDelete"
              />
            </div>
          </template>

          <template v-else>
            <FileGrid
              v-if="filesStore.viewMode === 'grid'"
              :files="filesStore.files"
              @open="handleOpenFolder"
              @preview="handlePreviewFile"
              @download="handleDownloadFile"
              @rename="handleRename"
              @move="handleMove"
              @delete="handleDelete"
            />
            <FileList
              v-else
              :files="filesStore.files"
              @open="handleOpenFolder"
              @preview="handlePreviewFile"
              @download="handleDownloadFile"
              @rename="handleRename"
              @move="handleMove"
              @delete="handleDelete"
            />
          </template>
        </div>
      </main>

      <UploadPanel v-if="uploadStore.totalCount > 0" />
    </div>

    <!-- Dialogs -->
    <CreateFolderDialog
      v-model="showCreateFolder"
      @created="handleFolderCreated"
    />

    <RenameDialog
      v-model="showRename"
      :file="selectedFile"
      @renamed="handleFileRenamed"
    />

    <MoveDialog
      v-model="showMove"
      :files="selectedFiles"
      @moved="handleFilesMoved"
    />

    <ConfirmDialog
      v-model="showDeleteConfirm"
      title="删除文件"
      :message="`确定要删除 ${selectedFiles.length > 1 ? selectedFiles.length + ' 个文件' : selectedFile?.name} 吗？删除后将移至回收站。`"
      type="danger"
      @confirm="handleConfirmDelete"
    />

    <ImagePreview
      v-model="showImagePreview"
      :file="previewFile"
    />

    <!-- Drag overlay -->
    <div v-if="isDragging" class="drag-overlay">
      <div class="drag-content">
        <div class="icon">📤</div>
        <div>拖放文件到此处上传</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useFilesStore } from '@/stores/files'
import { useUploadStore } from '@/stores/upload'
import AppHeader from '@/components/Layout/AppHeader.vue'
import AppSidebar from '@/components/Layout/AppSidebar.vue'
import Breadcrumb from '@/components/Files/Breadcrumb.vue'
import Toolbar from '@/components/Files/Toolbar.vue'
import SearchBar from '@/components/Files/SearchBar.vue'
import FileList from '@/components/Files/FileList.vue'
import FileGrid from '@/components/Files/FileGrid.vue'
import UploadPanel from '@/components/Upload/UploadPanel.vue'
import ImagePreview from '@/components/Preview/ImagePreview.vue'
import CreateFolderDialog from '@/components/Dialogs/CreateFolderDialog.vue'
import RenameDialog from '@/components/Dialogs/RenameDialog.vue'
import MoveDialog from '@/components/Dialogs/MoveDialog.vue'
import ConfirmDialog from '@/components/Dialogs/ConfirmDialog.vue'
import { ElMessage } from 'element-plus'
import { Close, Loading } from '@element-plus/icons-vue'

const filesStore = useFilesStore()
const uploadStore = useUploadStore()

// Dialog states
const showCreateFolder = ref(false)
const showRename = ref(false)
const showMove = ref(false)
const showDeleteConfirm = ref(false)
const showImagePreview = ref(false)

// Selection states
const selectedFile = ref(null)
const selectedFiles = ref([])
const previewFile = ref(null)

// Drag states
const isDragging = ref(false)
let dragCounter = 0

onMounted(() => {
  filesStore.fetchFiles()
  document.addEventListener('dragenter', handleDragEnter)
  document.addEventListener('dragleave', handleDragLeave)
  document.addEventListener('drop', handleDocumentDrop)
})

onUnmounted(() => {
  document.removeEventListener('dragenter', handleDragEnter)
  document.removeEventListener('dragleave', handleDragLeave)
  document.removeEventListener('drop', handleDocumentDrop)
})

function handleDragEnter(e) {
  e.preventDefault()
  dragCounter++
  if (e.dataTransfer?.types.includes('Files')) {
    isDragging.value = true
  }
}

function handleDragLeave(e) {
  e.preventDefault()
  dragCounter--
  if (dragCounter === 0) {
    isDragging.value = false
  }
}

function handleDocumentDrop(e) {
  e.preventDefault()
  dragCounter = 0
  isDragging.value = false
}

function handleDrop(e) {
  e.preventDefault()
  isDragging.value = false
  dragCounter = 0

  const files = Array.from(e.dataTransfer?.files || [])
  if (files.length > 0) {
    files.forEach(file => {
      file.parentId = filesStore.currentFolder
      uploadStore.addTask(file)
    })
  }
}

// Navigation
function handleNavigate(folder) {
  filesStore.navigateToFolder(folder)
}

function handleOpenFolder(folder) {
  filesStore.navigateToFolder(folder)
}

// Download
function handleDownloadFile(file) {
  if (file.isFolder) {
    filesStore.downloadFolder(file.id)
  } else {
    filesStore.downloadFile(file.id)
  }
}

// Preview
function handlePreviewFile(file) {
  const ext = file.name.split('.').pop().toLowerCase()
  const imageExts = ['jpg', 'jpeg', 'png', 'gif', 'bmp', 'webp']

  if (imageExts.includes(ext)) {
    previewFile.value = file
    showImagePreview.value = true
  } else {
    filesStore.downloadFile(file.id)
  }
}

// File actions
function handleRename(file) {
  selectedFile.value = file
  showRename.value = true
}

function handleMove(file) {
  selectedFiles.value = [file]
  showMove.value = true
}

function handleDelete(file) {
  selectedFile.value = file
  selectedFiles.value = [file]
  showDeleteConfirm.value = true
}

// Dialog handlers
function handleFolderCreated() {
  // Folder created, list auto-refreshed
}

function handleFileRenamed() {
  // File renamed, list auto-refreshed
}

function handleFilesMoved() {
  // Files moved, list auto-refreshed
}

async function handleConfirmDelete() {
  try {
    for (const file of selectedFiles.value) {
      await filesStore.deleteFile(file.id)
    }
    ElMessage.success('删除成功')
    showDeleteConfirm.value = false
    selectedFiles.value = []
    selectedFile.value = null
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '删除失败')
  }
}

function clearSearch() {
  filesStore.searchResults = []
  filesStore.isSearching = false
  filesStore.searchKeyword = ''
}
</script>

<style scoped lang="scss">
.files-view {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.files-layout {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.files-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 0 24px 24px;
  overflow: hidden;

  &.with-panel {
    margin-right: var(--upload-panel-width);
  }
}

.main-header {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.header-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.files-content {
  flex: 1;
  overflow-y: auto;
}

.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 48px;
  color: #909399;
}

.search-results {
  .results-header {
    padding: 12px 0;
    font-size: 14px;
    color: #606266;

    .results-count {
      color: #909399;
      margin-left: 8px;
    }
  }
}

.drag-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(64, 158, 255, 0.1);
  border: 3px dashed #409eff;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  pointer-events: none;

  .drag-content {
    text-align: center;
    color: #409eff;

    .icon {
      font-size: 48px;
      margin-bottom: 16px;
    }
  }
}
</style>
