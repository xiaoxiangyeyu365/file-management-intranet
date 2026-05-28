<template>
  <div class="file-list">
    <el-table
      :data="files"
      @row-dblclick="handleDoubleClick"
      @selection-change="handleSelectionChange"
      @sort-change="handleSortChange"
      row-key="id"
      style="width: 100%"
    >
      <el-table-column type="selection" width="50" />

      <el-table-column label="名称" min-width="300">
        <template #default="{ row }">
          <div class="file-name" @click="handleClick(row)">
            <span class="file-icon">{{ getFileIcon(row) }}</span>
            <span class="file-text">{{ row.name }}</span>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="大小" width="120" :sort-method="sortBySize" :sortable="'custom'">
        <template #default="{ row }">
          <span v-if="row.isFolder">-</span>
          <span v-else>{{ formatSize(row.physical?.size || 0) }}</span>
        </template>
      </el-table-column>

      <el-table-column label="修改时间" width="180" :sort-method="sortByTime" :sortable="'custom'">
        <template #default="{ row }">
          <span v-if="row.updatedAt && row.updatedAt !== '0001-01-01T00:00:00Z'">{{ formatDate(row.updatedAt) }}</span>
          <span v-else>-</span>
        </template>
      </el-table-column>

      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button text size="small" @click.stop="showContextMenu($event, row)">
            <el-icon><MoreFilled /></el-icon>
          </el-button>
        </template>
      </el-table-column>
    </el-table>

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
import { ref, onMounted, onUnmounted } from 'vue'
import { useFilesStore } from '@/stores/files'
import { formatSize, formatDate } from '@/utils/format'
import { MoreFilled, Edit, Download, Right, Delete } from '@element-plus/icons-vue'

const props = defineProps({
  files: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['open', 'preview', 'rename', 'download', 'move', 'delete'])

const filesStore = useFilesStore()

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
  if (file.isFolder) {
    emit('open', file)
  } else {
    emit('preview', file)
  }
}

function handleDoubleClick(row) {
  if (row.isFolder) {
    emit('open', row)
  } else {
    emit('preview', row)
  }
}

function handleSelectionChange(selection) {
  filesStore.setSelected(selection.map(f => f.id))
}

function sortBySize(a, b) {
  return (a.physical?.size ?? 0) - (b.physical?.size ?? 0)
}

function sortByTime(a, b) {
  return (a.updatedAt || '').localeCompare(b.updatedAt || '')
}

function handleSortChange({ prop, order }) {
  if (!order) {
    filesStore.setSort('name', 'asc')
  } else {
    const sortBy = prop === '大小' || prop === 'size' ? 'size' : prop === '修改时间' || prop === 'updatedAt' ? 'updatedAt' : 'name'
    filesStore.setSort(sortBy, order === 'ascending' ? 'asc' : 'desc')
  }
}
</script>

<style scoped lang="scss">
.file-list {
  :deep(.el-table) {
    .file-name {
      display: flex;
      align-items: center;
      gap: 8px;
      cursor: pointer;

      &:hover {
        color: #409eff;
      }

      .file-icon {
        font-size: 20px;
      }

      .file-text {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
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
</style>