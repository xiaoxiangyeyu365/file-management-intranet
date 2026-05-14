<template>
  <div class="upload-panel" :class="{ collapsed: !expanded }">
    <div class="panel-header" @click="toggleExpand">
      <span class="panel-title">
        <el-icon><Upload /></el-icon>
        上传队列
        <span v-if="totalCount > 0" class="count">({{ totalCount }})</span>
      </span>
      <div class="panel-actions">
        <el-button
          v-if="expanded && completedCount > 0"
          text
          size="small"
          @click.stop="handleClearCompleted"
        >
          清空已完成
        </el-button>
        <el-button text size="small" @click.stop="toggleExpand">
          <el-icon>
            <ArrowRight v-if="!expanded" />
            <ArrowDown v-else />
          </el-icon>
        </el-button>
      </div>
    </div>

    <div v-if="expanded" class="panel-content">
      <div v-if="tasks.length === 0" class="empty-state">
        暂无上传任务
      </div>
      <UploadItem
        v-for="task in tasks"
        :key="task.id"
        :task="task"
        @cancel="uploadStore.cancelUpload(task.id)"
        @retry="uploadStore.retryUpload(task.id)"
      />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useUploadStore } from '@/stores/upload'
import UploadItem from './UploadItem.vue'
import { Upload, ArrowRight, ArrowDown } from '@element-plus/icons-vue'

const uploadStore = useUploadStore()

const expanded = computed(() => uploadStore.expanded)
const tasks = computed(() => uploadStore.tasks)
const totalCount = computed(() => uploadStore.totalCount)
const completedCount = computed(() => uploadStore.completedTasks.length)

function toggleExpand() {
  uploadStore.setExpanded(!expanded.value)
}

function handleClearCompleted() {
  uploadStore.clearCompleted()
}
</script>

<style scoped lang="scss">
.upload-panel {
  position: fixed;
  right: 0;
  top: var(--header-height);
  width: var(--upload-panel-width);
  height: calc(100% - var(--header-height));
  background: white;
  border-left: 1px solid #e4e7ed;
  display: flex;
  flex-direction: column;
  z-index: 100;
  box-shadow: -2px 0 8px rgba(0, 0, 0, 0.1);

  &.collapsed {
    width: auto;

    .panel-header {
      padding: 12px;
      writing-mode: vertical-rl;
      text-orientation: mixed;
      cursor: pointer;
    }

    .panel-title {
      flex-direction: column;

      .count {
        margin: 4px 0;
      }
    }

    .panel-actions {
      display: none;
    }
  }
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #fafafa;
  border-bottom: 1px solid #e4e7ed;
  cursor: pointer;

  &:hover {
    background: #f0f0f0;
  }
}

.panel-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;

  .count {
    color: #909399;
    font-weight: normal;
  }
}

.panel-actions {
  display: flex;
  gap: 4px;
}

.panel-content {
  flex: 1;
  overflow-y: auto;

  .empty-state {
    padding: 24px;
    text-align: center;
    color: #909399;
    font-size: 14px;
  }
}
</style>