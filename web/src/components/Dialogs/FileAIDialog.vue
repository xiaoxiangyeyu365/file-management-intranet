<template>
  <el-dialog
    v-model="visible"
    title="AI 摘要与索引"
    width="min(560px, 92vw)"
  >
    <div v-if="loading" class="loading-state">
      <el-icon class="is-loading"><Loading /></el-icon>
      加载中...
    </div>

    <template v-else>
      <div class="file-head">
        <span class="file-icon">{{ isFolder ? '📁' : '📄' }}</span>
        <div class="file-head-text">
          <div class="file-name" :title="fileInfo?.name">{{ fileInfo?.name }}</div>
          <div class="file-meta">
            <span v-if="!isFolder">{{ formatSize(fileInfo?.physical?.size || 0) }}</span>
            <span v-if="fileInfo?.physical?.mimeType"> · {{ fileInfo.physical.mimeType }}</span>
          </div>
        </div>
      </div>

      <div class="status-row">
        <el-tag :type="summary ? 'success' : 'info'" size="small">
          {{ summary ? '已有 AI 摘要' : '暂无 AI 摘要' }}
        </el-tag>
        <el-tag :type="chunkCount > 0 ? 'success' : 'warning'" size="small">
          {{ chunkCount > 0 ? `已索引 · ${chunkCount} 个片段` : '未索引' }}
        </el-tag>
        <span v-if="generatedAt" class="generated-at">生成于 {{ formatDate(generatedAt) }}</span>
      </div>

      <div class="section">
        <div class="section-title">摘要</div>
        <div v-if="summary" class="summary-text">{{ summary }}</div>
        <div v-else class="empty-hint">暂无摘要，可点击下方“重新生成摘要”。</div>
      </div>

      <div class="section">
        <div class="section-title">标签</div>
        <div v-if="tags.length" class="tag-list">
          <el-tag v-for="tag in tags" :key="tag" size="small" type="info">{{ tag }}</el-tag>
        </div>
        <div v-else class="empty-hint">暂无标签</div>
      </div>

      <el-alert
        v-if="error"
        :title="error"
        type="error"
        :closable="false"
        show-icon
        class="error-alert"
      />

      <div v-if="isFolder" class="empty-hint">文件夹不支持 AI 摘要与索引。</div>
      <div v-else-if="!canIndex" class="hint">
        该文件类型暂不支持建立索引（目前支持文本 / PDF / DOCX）。
      </div>
      <div v-else class="hint">
        索引支持文本 / PDF / DOCX，重建在后台执行；完成后点“刷新”查看状态。
      </div>
    </template>

    <template #footer>
      <el-button :loading="loading" :disabled="isFolder" @click="load(true)">刷新</el-button>
      <el-button
        :loading="reindexing"
        :disabled="!canReindex || loading"
        @click="handleReindex"
      >
        重建索引
      </el-button>
      <el-button
        type="primary"
        :loading="regenerating"
        :disabled="isFolder || !hasPhysical || loading"
        @click="handleRegenerate"
      >
        重新生成摘要
      </el-button>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { fileAPI } from '@/utils/api'
import { formatSize, formatDate } from '@/utils/format'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  file: { type: Object, default: null }
})

const emit = defineEmits(['update:modelValue', 'updated'])

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const fileInfo = ref(null)
const loading = ref(false)
const summary = ref('')
const tags = ref([])
const chunkCount = ref(0)
const generatedAt = ref('')
const hasPhysical = ref(false)
const error = ref('')
const regenerating = ref(false)
const reindexing = ref(false)
let pollTimer = null

const isFolder = computed(() => !!props.file?.isFolder)

const INDEXABLE_EXTS = [
  'txt', 'md', 'markdown', 'pdf', 'docx',
  'json', 'csv', 'log', 'xml', 'yaml', 'yml', 'ini', 'cfg', 'conf',
  'sh', 'py', 'js', 'html', 'htm', 'css'
]

// Mirror the backend's supported RAG input types so we don't submit an
// index job that is guaranteed to fail for e.g. images or archives.
const canIndex = computed(() => {
  if (isFolder.value) return false
  const mime = fileInfo.value?.physical?.mimeType || ''
  if (
    mime.startsWith('text/') ||
    mime === 'application/pdf' ||
    mime === 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
  ) {
    return true
  }
  const name = fileInfo.value?.name || props.file?.name || ''
  const ext = name.includes('.') ? name.split('.').pop().toLowerCase() : ''
  return INDEXABLE_EXTS.includes(ext)
})

const canReindex = computed(() => hasPhysical.value && canIndex.value)

function apiError(res, fallback) {
  if (res && res.code !== undefined && res.code !== 0) {
    return res.message || fallback
  }
  return ''
}

function clearPoll() {
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
}

async function load(silent = false) {
  if (!props.file) return
  clearPoll()
  if (!silent) loading.value = true
  error.value = ''
  try {
    const res = await fileAPI.get(props.file.id)
    const fresh = res?.data || res
    const errMsg = apiError(res, '加载文件信息失败')
    if (errMsg) throw new Error(errMsg)

    fileInfo.value = fresh
    hasPhysical.value = !!fresh?.physical
    summary.value = fresh?.physical?.summary || ''
    chunkCount.value = fresh?.physical?.chunkCount || 0
    generatedAt.value = fresh?.physical?.summaryGeneratedAt || ''

    // The AI endpoint returns plain JSON on success and the standard
    // {code,message} envelope on errors, so treat code 404 as "no summary".
    const aiRes = await fileAPI.getAISummary(props.file.id)
    if (aiRes && aiRes.code === undefined) {
      summary.value = aiRes.summary || summary.value
      tags.value = aiRes.tags || []
      generatedAt.value = aiRes.generated_at || generatedAt.value
    } else if (aiRes && aiRes.code !== 0 && aiRes.code !== 404) {
      error.value = aiRes.message || '加载 AI 摘要失败'
    } else {
      tags.value = []
    }
  } catch (e) {
    error.value = e.response?.data?.message || e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function handleRegenerate() {
  if (!props.file) return
  regenerating.value = true
  error.value = ''
  try {
    const res = await fileAPI.regenerateSummary(props.file.id)
    const errMsg = apiError(res, '生成摘要失败')
    if (errMsg) throw new Error(errMsg)
    summary.value = res.summary || summary.value
    tags.value = res.tags || []
    generatedAt.value = new Date().toISOString()
    ElMessage.success('AI 摘要已更新')
    emit('updated')
  } catch (e) {
    error.value = e.response?.data?.message || e.message || '生成摘要失败'
  } finally {
    regenerating.value = false
  }
}

// Indexing runs asynchronously on the server; poll a few times so the
// status updates without the user having to reopen the dialog.
function pollReindex(attempt = 0) {
  if (attempt >= 4) return
  pollTimer = setTimeout(async () => {
    if (!visible.value) return
    await load(true)
    pollReindex(attempt + 1)
  }, 3000)
}

async function handleReindex() {
  if (!props.file) return
  reindexing.value = true
  error.value = ''
  try {
    const res = await fileAPI.reindex(props.file.id)
    const errMsg = apiError(res, '重建索引失败')
    if (errMsg) throw new Error(errMsg)
    ElMessage.success('已提交后台重建索引，稍后自动刷新状态')
    emit('updated')
    pollReindex()
  } catch (e) {
    error.value = e.response?.data?.message || e.message || '重建索引失败'
  } finally {
    reindexing.value = false
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) load()
    else clearPoll()
  }
)

onUnmounted(clearPoll)
</script>

<style scoped lang="scss">
.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px;
  color: #909399;
}

.file-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}

.file-icon {
  font-size: 34px;
}

.file-head-text {
  min-width: 0;
}

.file-name {
  font-size: 15px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-meta {
  margin-top: 2px;
  font-size: 12px;
  color: #909399;
}

.status-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}

.generated-at {
  font-size: 12px;
  color: #909399;
}

.section {
  margin-bottom: 16px;
}

.section-title {
  margin-bottom: 6px;
  font-size: 13px;
  font-weight: 600;
  color: #606266;
}

.summary-text {
  max-height: 220px;
  overflow-y: auto;
  padding: 10px 12px;
  border-radius: 6px;
  background: #f5f7fa;
  font-size: 13px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.empty-hint {
  font-size: 13px;
  color: #909399;
}

.hint {
  margin-top: 12px;
  font-size: 12px;
  color: #a8abb2;
}

.error-alert {
  margin-top: 8px;
}
</style>
