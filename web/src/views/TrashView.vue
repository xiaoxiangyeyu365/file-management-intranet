<template>
  <div class="trash-view">
    <AppHeader />

    <div class="files-layout">
      <AppSidebar />

      <main class="files-main">
        <div class="main-header">
          <div class="header-title">
            <h2>回收站</h2>
            <span class="file-count">{{ trashFiles.length }} 个文件</span>
          </div>

          <div class="header-actions">
            <el-button
              v-if="selectedFiles.length > 0"
              type="primary"
              @click="handleRestore"
            >
              恢复
            </el-button>
            <el-button
              v-if="selectedFiles.length > 0"
              type="danger"
              @click="handlePermanentDelete"
            >
              永久删除
            </el-button>
            <el-button
              v-if="trashFiles.length > 0"
              type="danger"
              plain
              @click="showEmptyConfirm = true"
            >
              清空回收站
            </el-button>
          </div>
        </div>

        <div class="files-content">
          <el-table
            v-if="trashFiles.length > 0"
            :data="trashFiles"
            @selection-change="handleSelectionChange"
            row-key="id"
            style="width: 100%"
          >
            <el-table-column type="selection" width="50" />

            <el-table-column label="名称" min-width="300">
              <template #default="{ row }">
                <div class="file-name">
                  <span class="file-icon">{{ row.isFolder ? '📁' : '📄' }}</span>
                  <span>{{ row.name }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column label="删除时间" width="180">
              <template #default="{ row }">
                {{ row.deletedAt?.Valid ? formatDate(row.deletedAt.Time) : '-' }}
              </template>
            </el-table-column>

            <el-table-column label="操作" width="200" fixed="right">
              <template #default="{ row }">
                <el-button text size="small" @click="handleRestoreSingle(row)">
                  恢复
                </el-button>
                <el-button text size="small" type="danger" @click="handleDeleteSingle(row)">
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>

          <el-empty v-else description="回收站为空" />
        </div>
      </main>
    </div>

    <ConfirmDialog
      v-model="showEmptyConfirm"
      title="清空回收站"
      message="确定要清空回收站吗？此操作不可恢复。"
      type="danger"
      @confirm="handleEmptyTrash"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { trashAPI } from '@/utils/api'
import { formatDate } from '@/utils/format'
import AppHeader from '@/components/Layout/AppHeader.vue'
import AppSidebar from '@/components/Layout/AppSidebar.vue'
import ConfirmDialog from '@/components/Dialogs/ConfirmDialog.vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const route = useRoute()
const trashFiles = ref([])
const selectedFiles = ref([])
const showEmptyConfirm = ref(false)
const loading = ref(false)

onMounted(() => {
  fetchTrash()
})

// Reload data when route changes (e.g., when navigating to /trash)
import { onBeforeRouteUpdate } from 'vue-router'
onBeforeRouteUpdate(() => {
  fetchTrash()
})

async function fetchTrash() {
  loading.value = true
  try {
    const response = await trashAPI.list()
    trashFiles.value = response?.data?.files || []
  } catch (err) {
    ElMessage.error('加载回收站失败')
  } finally {
    loading.value = false
  }
}

function handleSelectionChange(selection) {
  selectedFiles.value = selection
}

async function handleRestore() {
  try {
    for (const file of selectedFiles.value) {
      await trashAPI.restore(file.id)
    }
    ElMessage.success('恢复成功')
    await fetchTrash()
    selectedFiles.value = []
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '恢复失败')
  }
}

async function handleRestoreSingle(file) {
  try {
    await trashAPI.restore(file.id)
    ElMessage.success('恢复成功')
    await fetchTrash()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '恢复失败')
  }
}

async function handlePermanentDelete() {
  try {
    await ElMessageBox.confirm(
      `确定要永久删除 ${selectedFiles.value.length} 个文件吗？此操作不可恢复。`,
      '确认删除',
      { type: 'warning' }
    )

    for (const file of selectedFiles.value) {
      await trashAPI.delete(file.id)
    }
    ElMessage.success('删除成功')
    await fetchTrash()
    selectedFiles.value = []
  } catch (err) {
    if (err !== 'cancel') {
      ElMessage.error(err.response?.data?.message || '删除失败')
    }
  }
}

async function handleDeleteSingle(file) {
  try {
    await ElMessageBox.confirm(
      `确定要永久删除 "${file.name}" 吗？此操作不可恢复。`,
      '确认删除',
      { type: 'warning' }
    )

    await trashAPI.delete(file.id)
    ElMessage.success('删除成功')
    await fetchTrash()
  } catch (err) {
    if (err !== 'cancel') {
      ElMessage.error(err.response?.data?.message || '删除失败')
    }
  }
}

async function handleEmptyTrash() {
  try {
    await trashAPI.empty()
    ElMessage.success('清空成功')
    await fetchTrash()
    showEmptyConfirm.value = false
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '清空失败')
  }
}
</script>

<style scoped lang="scss">
.trash-view {
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
  padding: 24px;
  overflow: hidden;
}

.main-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;

  .header-title {
    display: flex;
    align-items: center;
    gap: 12px;

    h2 {
      margin: 0;
      font-size: 20px;
      font-weight: 600;
    }

    .file-count {
      color: #909399;
      font-size: 14px;
    }
  }
}

.files-content {
  flex: 1;
  overflow-y: auto;

  .file-name {
    display: flex;
    align-items: center;
    gap: 8px;

    .file-icon {
      font-size: 20px;
    }
  }
}
</style>