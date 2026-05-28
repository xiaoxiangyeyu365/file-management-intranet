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
