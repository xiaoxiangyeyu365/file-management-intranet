<template>
  <div class="file-grid">
    <div
      v-for="file in files"
      :key="file.id"
      class="file-card"
      :class="{ selected: selectedIds.includes(file.id) }"
      @click="handleClick(file)"
      @dblclick="handleDoubleClick(file)"
    >
      <div class="file-preview">
        <span class="file-icon">{{ getFileIcon(file) }}</span>
      </div>
      <div class="file-name" :title="file.name">{{ file.name }}</div>
      <div class="file-size">{{ file.is_folder ? '' : formatSize(file.size) }}</div>
    </div>

    <div v-if="files.length === 0" class="empty-state">
      <el-empty description="暂无文件" />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useFilesStore } from '@/stores/files'
import { formatSize } from '@/utils/format'

const props = defineProps({
  files: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['open', 'preview', 'rename', 'download', 'move', 'delete'])

const filesStore = useFilesStore()
const selectedIds = computed(() => filesStore.selectedIds)

function getFileIcon(file) {
  if (file.is_folder) return '📁'
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
  if (file.is_folder) {
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

    .file-size {
      font-size: 12px;
      color: #909399;
      text-align: center;
      margin-top: 4px;
    }
  }

  .empty-state {
    grid-column: 1 / -1;
    padding: 48px;
  }
}
</style>