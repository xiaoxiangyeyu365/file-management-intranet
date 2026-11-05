<template>
  <div class="file-list">
    <el-table
      ref="tableRef"
      :data="files"
      :default-sort="defaultSort"
      @row-dblclick="handleDoubleClick"
      @selection-change="handleSelectionChange"
      @sort-change="handleSortChange"
      row-key="id"
      style="width: 100%"
    >
      <el-table-column type="selection" width="50" />

      <el-table-column label="名称" prop="name" min-width="300" :sortable="sortable ? 'custom' : false">
        <template #default="{ row }">
          <div class="file-name" @click="handleClick(row, $event)">
            <span class="file-icon">{{ getFileIcon(row) }}</span>
            <span class="file-text">{{ row.name }}</span>
            <el-icon
              v-if="row.physical?.summary"
              class="ai-mark"
              title="查看 AI 摘要"
              @click.stop="emit('ai', row)"
            >
              <MagicStick />
            </el-icon>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="大小" prop="size" width="120" :sortable="sortable ? 'custom' : false">
        <template #default="{ row }">
          <span v-if="row.isFolder">-</span>
          <span v-else>{{ formatSize(row.physical?.size || 0) }}</span>
        </template>
      </el-table-column>

      <el-table-column label="修改时间" prop="updatedAt" width="180" :sortable="sortable ? 'custom' : false">
        <template #default="{ row }">
          <span v-if="row.updatedAt && row.updatedAt !== '0001-01-01T00:00:00Z'">{{ formatDate(row.updatedAt) }}</span>
          <span v-else>-</span>
        </template>
      </el-table-column>

      <el-table-column label="操作" width="110" fixed="right">
        <template #default="{ row }">
          <div class="row-actions">
            <el-button text size="small" title="AI 摘要与索引" @click.stop="emit('ai', row)">
              <el-icon><MagicStick /></el-icon>
            </el-button>
            <el-button text size="small" @click.stop="showContextMenu($event, row)">
              <el-icon><MoreFilled /></el-icon>
            </el-button>
          </div>
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
        <div class="context-menu-item" @click="handleShare">
          <el-icon><Share /></el-icon>
          <span>分享</span>
        </div>
        <div class="context-menu-item" @click="handleAI">
          <el-icon><MagicStick /></el-icon>
          <span>AI 摘要</span>
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
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useFilesStore } from '@/stores/files'
import { formatSize, formatDate } from '@/utils/format'
import { MoreFilled, Edit, Download, Right, Delete, Share, MagicStick } from '@element-plus/icons-vue'

const props = defineProps({
  files: {
    type: Array,
    default: () => []
  },
  sortable: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['open', 'preview', 'rename', 'download', 'move', 'delete', 'share', 'ai'])

const filesStore = useFilesStore()

const tableRef = ref(null)

const defaultSort = computed(() => ({
  prop: filesStore.sortBy,
  order: filesStore.sortOrder === 'asc' ? 'ascending' : 'descending'
}))

// The store is the single source of truth for sorting. Mirror store changes
// into Element Plus so the header arrows never contradict the toolbar label
// (e.g. after picking "大小 大→小" from the dropdown).
watch(
  () => [filesStore.sortBy, filesStore.sortOrder],
  ([sortBy, sortOrder]) => {
    tableRef.value?.sort(sortBy, sortOrder === 'asc' ? 'ascending' : 'descending')
  }
)

// Context menu state
const contextMenuVisible = ref(false)
const contextMenuFile = ref(null)
const contextMenuPos = ref({ x: 0, y: 0 })

function showContextMenu(event, file) {
  contextMenuFile.value = file
  // Clamp position to viewport bounds
  const menuWidth = 160
  const menuHeight = 250
  const x = Math.max(0, Math.min(event.clientX, window.innerWidth - menuWidth))
  const y = Math.max(0, Math.min(event.clientY, window.innerHeight - menuHeight))
  contextMenuPos.value = { x, y }
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

function handleShare() {
  if (contextMenuFile.value) {
    emit('share', contextMenuFile.value)
  }
  hideContextMenu()
}

function handleAI() {
  if (contextMenuFile.value) {
    emit('ai', contextMenuFile.value)
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

function handleClick(file, event) {
  if (event && event.shiftKey) {
    // Shift+Click: range select
    filesStore.selectRange(props.files, file.id)
    return
  }
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

function handleSortChange({ prop, order }) {
  if (!order) {
    filesStore.setSort('name', 'asc')
    return
  }
  filesStore.setSort(prop || 'name', order === 'ascending' ? 'asc' : 'desc')
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

      .ai-mark {
        flex-shrink: 0;
        color: #b37feb;
        font-size: 14px;
        cursor: pointer;
      }
    }
  }

  .row-actions {
    display: flex;
    align-items: center;
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