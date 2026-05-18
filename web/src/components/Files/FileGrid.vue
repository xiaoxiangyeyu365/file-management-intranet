<template>
  <div class="file-grid">
    <div
      v-for="file in files"
      :key="file.id"
      class="file-card"
      :class="{ selected: selectedIds.includes(file.id) }"
      @click="handleClick(file)"
      @dblclick="handleDoubleClick(file)"
      @contextmenu.prevent="showContextMenu($event, file)"
    >
      <div class="file-preview">
        <span class="file-icon">{{ getFileIcon(file) }}</span>
      </div>
      <div class="file-name" :title="file.name">{{ file.name }}</div>
      <div class="file-meta">
        <span class="file-size">{{ file.isFolder ? '' : formatSize(file.physical?.size || 0) }}</span>
        <span class="file-time" v-if="file.updatedAt && file.updatedAt !== '0001-01-01T00:00:00Z'">{{ formatDate(file.updatedAt) }}</span>
      </div>
    </div>

    <!-- Context Menu - Single component at component level -->
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

    <div v-if="files.length === 0" class="empty-state">
      <el-empty description="暂无文件" />
    </div>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useFilesStore } from '@/stores/files'
import { formatSize, formatDate } from '@/utils/format'
import { Edit, Download, Right, Delete } from '@element-plus/icons-vue'

const props = defineProps({
  files: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['open', 'preview', 'rename', 'download', 'move', 'delete'])

const filesStore = useFilesStore()
const selectedIds = computed(() => filesStore.selectedIds)

// Context menu state
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
  if (contextMenuFile.value) {
    emit('rename', contextMenuFile.value)
  }
  hideContextMenu()
}

function handleDownload() {
  if (contextMenuFile.value) {
    emit('download', contextMenuFile.value)
  }
  hideContextMenu()
}

function handleMove() {
  if (contextMenuFile.value) {
    emit('move', contextMenuFile.value)
  }
  hideContextMenu()
}

function handleDelete() {
  if (contextMenuFile.value) {
    emit('delete', contextMenuFile.value)
  }
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

function handleClick(file) {
  const newSelected = selectedIds.value.includes(file.id)
    ? selectedIds.value.filter(id => id !== file.id)
    : [...selectedIds.value, file.id]
  filesStore.setSelected(newSelected)
}

function handleDoubleClick(file) {
  if (file.isFolder) {
    emit('open', file)
  } else {
    emit('preview', file)
  }
}
</script>

<style scoped lang="scss">
.file-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 16px;
  padding: 16px;

  .file-card {
    position: relative;
    padding: 16px;
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.2s;
    border: 2px solid transparent;

    &:hover {
      background: #f5f7fa;
    }

    &.selected {
      background: #ecf5ff;
      border-color: #409eff;
    }

    .file-preview {
      height: 80px;
      display: flex;
      align-items: center;
      justify-content: center;
      margin-bottom: 8px;

      .file-icon {
        font-size: 48px;
      }
    }

    .file-name {
      font-size: 14px;
      text-align: center;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .file-meta {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 2px;
      margin-top: 4px;
    }

    .file-size, .file-time {
      font-size: 11px;
      color: #909399;
    }

    .file-size {
      font-size: 12px;
    }
  }

  .empty-state {
    grid-column: 1 / -1;
    padding: 48px;
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
</style>