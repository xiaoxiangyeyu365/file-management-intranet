<template>
  <div class="upload-item" :class="statusClass">
    <div class="upload-info">
      <span class="file-icon">📄</span>
      <span class="file-name" :title="task.name">{{ task.name }}</span>
    </div>

    <div v-if="task.status === 'hashing'" class="upload-status">
      <span class="status-text">计算哈希...</span>
    </div>

    <div v-else-if="task.status === 'uploading'" class="upload-progress">
      <el-progress :percentage="task.progress" :stroke-width="6" />
      <div class="progress-info">
        <span>{{ formatSize(task.uploadedBytes) }} / {{ formatSize(task.size) }}</span>
        <span>{{ formatSpeed(task.speed) }}</span>
      </div>
    </div>

    <div v-else-if="task.status === 'completed'" class="upload-status completed">
      <el-icon><Check /></el-icon>
      <span>已完成</span>
    </div>

    <div v-else-if="task.status === 'error'" class="upload-status error">
      <span class="error-text">{{ task.error || '上传失败' }}</span>
    </div>

    <div class="upload-actions">
      <el-button
        v-if="task.status === 'uploading'"
        text
        size="small"
        @click="emit('cancel')"
      >
        取消
      </el-button>
      <el-button
        v-if="task.status === 'error'"
        text
        size="small"
        @click="emit('retry')"
      >
        重试
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { formatSize } from '@/utils/format'
import { Check } from '@element-plus/icons-vue'

const props = defineProps({
  task: {
    type: Object,
    required: true
  }
})

const emit = defineEmits(['cancel', 'retry'])

const statusClass = computed(() => ({
  hashing: props.task.status === 'hashing',
  uploading: props.task.status === 'uploading',
  completed: props.task.status === 'completed',
  error: props.task.status === 'error'
}))

function formatSpeed(bytesPerSecond) {
  if (!bytesPerSecond) return ''
  return formatSize(bytesPerSecond) + '/s'
}
</script>

<style scoped lang="scss">
.upload-item {
  padding: 12px;
  border-bottom: 1px solid #f0f0f0;

  &:last-child {
    border-bottom: none;
  }

  .upload-info {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;

    .file-icon {
      font-size: 20px;
    }

    .file-name {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 14px;
    }
  }

  .upload-status {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 12px;
    color: #909399;

    &.completed {
      color: #67c23a;
    }

    &.error {
      color: #f56c6c;
    }
  }

  .upload-progress {
    .progress-info {
      display: flex;
      justify-content: space-between;
      font-size: 11px;
      color: #909399;
      margin-top: 4px;
    }
  }

  .upload-actions {
    margin-top: 8px;
    text-align: right;
  }
}
</style>