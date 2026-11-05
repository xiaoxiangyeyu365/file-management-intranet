<template>
  <el-dialog
    v-model="visible"
    :title="file?.name || '文件预览'"
    width="80%"
    top="5vh"
    destroy-on-close
    @closed="handleClosed"
  >
    <div class="preview-body" v-loading="loading">
      <!-- PDF Preview -->
      <iframe
        v-if="previewType === 'pdf'"
        :src="previewSrc"
        class="preview-frame"
      />

      <!-- Text/Code Preview -->
      <pre
        v-else-if="previewType === 'text'"
        class="preview-text"
      >{{ textContent }}</pre>

      <!-- Unsupported -->
      <div v-else class="preview-unsupported">
        <el-icon :size="48"><Document /></el-icon>
        <p>此文件类型不支持预览</p>
        <el-button type="primary" @click="handleDownload">下载文件</el-button>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { Document } from '@element-plus/icons-vue'
import { fileAPI } from '@/utils/api'

const props = defineProps({
  modelValue: Boolean,
  file: Object
})

const emit = defineEmits(['update:modelValue'])

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const loading = ref(false)
const textContent = ref('')

const textExts = ['txt', 'md', 'json', 'xml', 'yaml', 'yml', 'ini', 'cfg', 'conf', 'log',
  'csv', 'py', 'js', 'ts', 'jsx', 'tsx', 'html', 'htm', 'css', 'scss', 'sh', 'sql',
  'java', 'c', 'cpp', 'h', 'go', 'rs', 'rb', 'php', 'vue', 'toml']
const pdfExts = ['pdf']

const previewType = computed(() => {
  if (!props.file) return null
  const ext = props.file.name.split('.').pop().toLowerCase()
  if (pdfExts.includes(ext)) return 'pdf'
  if (textExts.includes(ext)) return 'text'
  return null
})

const previewSrc = computed(() => {
  if (!props.file || previewType.value !== 'pdf') return ''
  return blobUrl.value || ''
})

const blobUrl = ref('')

watch(() => props.modelValue, async (val) => {
  if (!val || !props.file) return

  if (previewType.value === 'text') {
    loading.value = true
    textContent.value = ''
    try {
      const token = localStorage.getItem('cloudbox_token')
      const resp = await fetch(fileAPI.previewUrl(props.file.id), {
        headers: { Authorization: `Bearer ${token}` }
      })
      textContent.value = await resp.text()
    } catch (err) {
      textContent.value = '加载失败: ' + err.message
    } finally {
      loading.value = false
    }
  } else if (previewType.value === 'pdf') {
    loading.value = true
    try {
      const token = localStorage.getItem('cloudbox_token')
      const resp = await fetch(fileAPI.previewUrl(props.file.id), {
        headers: { Authorization: `Bearer ${token}` }
      })
      const blob = await resp.blob()
      // Revoke previous blob URL to avoid memory leak
      if (blobUrl.value) URL.revokeObjectURL(blobUrl.value)
      blobUrl.value = URL.createObjectURL(blob)
    } catch (err) {
      textContent.value = '加载失败: ' + err.message
    } finally {
      loading.value = false
    }
  }
})

function handleClosed() {
  textContent.value = ''
  if (blobUrl.value) {
    URL.revokeObjectURL(blobUrl.value)
    blobUrl.value = ''
  }
}

function handleDownload() {
  if (props.file) {
    window.open(fileAPI.downloadUrl(props.file.id), '_blank')
  }
}
</script>

<style scoped lang="scss">
.preview-body {
  height: 70vh;
  overflow: hidden;
}

.preview-frame {
  width: 100%;
  height: 100%;
  border: none;
  border-radius: 4px;
}

.preview-text {
  height: 100%;
  overflow: auto;
  padding: 16px;
  background: #fafafa;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}

.preview-unsupported {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 16px;
  color: #909399;

  p {
    font-size: 16px;
  }
}
</style>
