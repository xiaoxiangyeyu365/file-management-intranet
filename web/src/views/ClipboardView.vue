<template>
  <div class="clipboard-view">
    <AppHeader />

    <div class="files-layout">
      <AppSidebar />

      <main class="clipboard-main">
        <div class="main-header">
          <h2>云剪切板</h2>
          <el-button type="danger" plain @click="handleClear">
            清空
          </el-button>
        </div>

        <div class="clipboard-content">
          <!-- Input Section -->
          <div class="input-section">
            <el-input
              v-model="inputContent"
              type="textarea"
              :rows="4"
              :maxlength="10240"
              show-word-limit
              placeholder="输入文本内容，点击保存后可在其他设备查看"
              @keydown.ctrl.enter="handleSave"
            />
            <div class="input-actions">
              <el-button type="primary" @click="handleSave" :loading="saving">
                保存
              </el-button>
            </div>
          </div>

          <!-- Device Name Section -->
          <div class="device-section">
            <span class="label">设备名称：</span>
            <el-input
              v-model="deviceName"
              placeholder="设置设备名称，方便识别来源"
              style="width: 200px"
            />
            <el-button @click="handleSaveDeviceName">保存</el-button>
          </div>

          <!-- Records List -->
          <div class="records-section" ref="recordsSectionRef">
            <div v-if="clipboardStore.loading && clipboardStore.records.length === 0" class="loading">
              <el-icon class="is-loading"><Loading /></el-icon>
              加载中...
            </div>

            <div v-else-if="clipboardStore.records.length === 0" class="empty">
              <el-empty description="暂无剪切板记录" />
            </div>

            <div v-else class="records-list">
              <div
                v-for="record in clipboardStore.records"
                :key="record.id"
                class="record-item"
                @click="handleCopy(record)"
              >
                <div class="record-content">
                  <span class="pin-icon" v-if="record.pinned">📌</span>
                  <span class="new-icon" v-if="clipboardStore.isNew(record)">✨</span>
                  <span class="content-text">{{ record.content }}</span>
                </div>
                <div class="record-meta">
                  来自: {{ record.deviceName }} · {{ formatTime(record.createdAt) }}
                </div>
                <div class="record-actions" @click.stop>
                  <el-button
                    size="small"
                    text
                    @click="handleTogglePin(record)"
                  >
                    {{ record.pinned ? '取消置顶' : '置顶' }}
                  </el-button>
                  <el-button
                    size="small"
                    text
                    type="danger"
                    @click="handleDelete(record)"
                  >
                    删除
                  </el-button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useClipboardStore } from '@/stores/clipboard'
import { useAuthStore } from '@/stores/auth'
import AppHeader from '@/components/Layout/AppHeader.vue'
import AppSidebar from '@/components/Layout/AppSidebar.vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'

const clipboardStore = useClipboardStore()
const authStore = useAuthStore()

const inputContent = ref('')
const saving = ref(false)
const deviceName = ref('')
const recordsSectionRef = ref(null)
let scrollTop = 0

const DEVICE_NAME_KEY = 'cloudbox_device_name'

function getDeviceNameKey() {
  return DEVICE_NAME_KEY
}

onMounted(async () => {
  // Load device name from localStorage
  deviceName.value = localStorage.getItem(DEVICE_NAME_KEY) || ''
  await clipboardStore.fetchRecords()
  clipboardStore.startPolling()

  // Save scroll position before polling updates
  if (recordsSectionRef.value) {
    recordsSectionRef.value.addEventListener('scroll', () => {
      scrollTop = recordsSectionRef.value.scrollTop
    })
  }
})

// Restore scroll position after records update
watch(() => clipboardStore.records.length, () => {
  nextTick(() => {
    if (recordsSectionRef.value && scrollTop > 0) {
      recordsSectionRef.value.scrollTop = scrollTop
    }
  })
})

onUnmounted(() => {
  clipboardStore.stopPolling()
})

async function handleSave() {
  if (!inputContent.value.trim()) {
    ElMessage.error('请输入内容')
    return
  }

  if (!deviceName.value) {
    ElMessage.error('请先设置设备名称')
    return
  }

  saving.value = true
  try {
    const result = await clipboardStore.createRecord(inputContent.value)
    if (result) {
      inputContent.value = ''
    }
  } finally {
    saving.value = false
  }
}

function handleSaveDeviceName() {
  if (deviceName.value) {
    localStorage.setItem(DEVICE_NAME_KEY, deviceName.value)
    ElMessage.success('设备名称已保存: ' + deviceName.value)
  }
}

async function handleCopy(record) {
  try {
    // Try modern API first
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(record.content)
      ElMessage.success('已复制到剪贴板')
    } else {
      // Fallback for older browsers or HTTP context
      const textarea = document.createElement('textarea')
      textarea.value = record.content
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
      ElMessage.success('已复制到剪贴板')
    }
  } catch (err) {
    console.error('Copy failed:', err)
    ElMessage.error('复制失败')
  }
}

async function handleTogglePin(record) {
  await clipboardStore.togglePin(record)
}

async function handleDelete(record) {
  try {
    await ElMessageBox.confirm('确定要删除这条记录吗？', '确认删除', {
      type: 'warning'
    })
    await clipboardStore.deleteRecord(record)
  } catch {
    // User cancelled
  }
}

async function handleClear() {
  try {
    await ElMessageBox.confirm(
      '确定要清空所有非置顶记录吗？置顶记录将保留。',
      '确认清空',
      { type: 'warning' }
    )
    await clipboardStore.clearAll(true)
  } catch {
    // User cancelled
  }
}

function formatTime(dateStr) {
  const date = new Date(dateStr)
  const now = new Date()
  const diff = now - date

  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}天前`

  return date.toLocaleDateString('zh-CN')
}
</script>

<style scoped lang="scss">
.clipboard-view {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.files-layout {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.clipboard-main {
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

  h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 600;
  }
}

.clipboard-content {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.input-section {
  background: white;
  padding: 16px;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);

  .input-actions {
    margin-top: 12px;
    display: flex;
    justify-content: flex-end;
  }
}

.device-section {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: #f5f7fa;
  border-radius: 8px;

  .label {
    color: #606266;
    font-size: 14px;
  }
}

.records-section {
  flex: 1;
  overflow-y: auto;
}

.loading, .empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px;
  color: #909399;
}

.records-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.record-item {
  background: white;
  padding: 16px;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  cursor: pointer;
  transition: box-shadow 0.2s;

  &:hover {
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  }

  .record-content {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;

    .pin-icon, .new-icon {
      font-size: 14px;
    }

    .content-text {
      font-size: 14px;
      color: #333;
      word-break: break-all;
      max-height: 100px;
      overflow: hidden;
    }
  }

  .record-meta {
    font-size: 12px;
    color: #909399;
    margin-bottom: 8px;
  }

  .record-actions {
    display: flex;
    gap: 8px;
  }
}
</style>