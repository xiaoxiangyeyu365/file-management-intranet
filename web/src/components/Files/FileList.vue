<template>
  <div class="file-list">
    <el-table
      :data="files"
      @row-dblclick="handleDoubleClick"
      @selection-change="handleSelectionChange"
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

      <el-table-column prop="size" label="大小" width="120" sortable>
        <template #default="{ row }">
          <span v-if="row.is_folder">-</span>
          <span v-else>{{ formatSize(row.size) }}</span>
        </template>
      </el-table-column>

      <el-table-column prop="updated_at" label="修改时间" width="180" sortable>
        <template #default="{ row }">
          {{ formatDate(row.updated_at) }}
        </template>
      </el-table-column>

      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-dropdown trigger="click" @command="handleCommand($event, row)">
            <el-button text size="small">
              <el-icon><MoreFilled /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="rename">
                  <el-icon><Edit /></el-icon>
                  重命名
                </el-dropdown-item>
                <el-dropdown-item command="download">
                  <el-icon><Download /></el-icon>
                  下载
                </el-dropdown-item>
                <el-dropdown-item command="move">
                  <el-icon><Right /></el-icon>
                  移动
                </el-dropdown-item>
                <el-dropdown-item command="delete" divided>
                  <el-icon><Delete /></el-icon>
                  删除
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
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
  if (file.is_folder) {
    emit('open', file)
  } else {
    emit('preview', file)
  }
}

function handleDoubleClick(row) {
  if (row.is_folder) {
    emit('open', row)
  } else {
    emit('preview', row)
  }
}

function handleSelectionChange(selection) {
  filesStore.setSelected(selection.map(f => f.id))
}

function handleCommand(command, file) {
  switch (command) {
    case 'rename':
      emit('rename', file)
      break
    case 'download':
      emit('download', file)
      break
    case 'move':
      emit('move', file)
      break
    case 'delete':
      emit('delete', file)
      break
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
</style>