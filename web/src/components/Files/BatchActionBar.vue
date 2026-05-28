<template>
  <Transition name="slide-up">
    <div v-if="filesStore.selectedIds.length > 0" class="batch-action-bar">
      <span class="batch-info">已选 {{ filesStore.selectedIds.length }} 项</span>

      <el-button type="danger" size="small" @click="emit('batch-delete')">
        <el-icon><Delete /></el-icon>
        删除
      </el-button>

      <el-button type="warning" size="small" @click="emit('batch-move')">
        <el-icon><Right /></el-icon>
        移动到
      </el-button>

      <el-button type="primary" size="small" @click="emit('batch-download')">
        <el-icon><Download /></el-icon>
        下载
      </el-button>

      <span class="batch-cancel" @click="filesStore.setSelected([])">取消选择</span>
    </div>
  </Transition>
</template>

<script setup>
import { useFilesStore } from '@/stores/files'
import { Delete, Right, Download } from '@element-plus/icons-vue'

const filesStore = useFilesStore()

const emit = defineEmits(['batch-delete', 'batch-move', 'batch-download'])
</script>

<style scoped lang="scss">
.batch-action-bar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 24px;
  background: #fff;
  border-top: 1px solid #e4e7ed;
  box-shadow: 0 -2px 12px rgba(0, 0, 0, 0.08);
}

.batch-info {
  font-weight: 600;
  color: #409eff;
  margin-right: 8px;
}

.batch-cancel {
  margin-left: auto;
  color: #909399;
  cursor: pointer;
  font-size: 13px;

  &:hover {
    color: #606266;
  }
}

.slide-up-enter-active,
.slide-up-leave-active {
  transition: transform 0.3s ease, opacity 0.3s ease;
}

.slide-up-enter-from,
.slide-up-leave-to {
  transform: translateY(100%);
  opacity: 0;
}
</style>
