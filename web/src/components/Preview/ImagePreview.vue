<template>
  <el-dialog
    v-model="visible"
    :show-close="false"
    :close-on-click-modal="true"
    :close-on-press-escape="true"
    class="image-preview-dialog"
    width="auto"
    @closed="handleClosed"
  >
    <div class="preview-container">
      <button class="close-btn" @click="handleClose">
        <el-icon><Close /></el-icon>
      </button>

      <div v-if="loading" class="loading-state">
        <el-icon class="is-loading"><Loading /></el-icon>
      </div>

      <div v-else-if="error" class="error-state">
        {{ error }}
      </div>

      <div v-else class="image-wrapper">
        <img :src="blobUrl" :alt="file?.name" @load="handleImageLoad" />
      </div>

      <div v-if="file" class="image-info">
        <div class="file-name">{{ file.name }}</div>
        <div class="file-meta">
          <span v-if="dimensions">{{ dimensions }}</span>
          <span v-if="dimensions && file.physical?.size"> • </span>
          <span>{{ formatSize(file.physical?.size || 0) }}</span>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { previewAPI } from '@/utils/api'
import { formatSize } from '@/utils/format'
import { Close, Loading } from '@element-plus/icons-vue'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  file: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['update:modelValue', 'closed'])

const blobUrl = ref('')
const dimensions = ref('')
const loading = ref(false)
const error = ref('')

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

watch(() => props.file, async (file) => {
  if (file && !file.isFolder) {
    // Fetch image with auth header
    loading.value = true
    error.value = ''
    try {
      const token = localStorage.getItem('cloudbox_token')
      const url = `/api/files/${file.id}/download?t=${Date.now()}`
      const response = await fetch(url, {
        headers: { Authorization: `Bearer ${token}` }
      })
      if (!response.ok) {
        throw new Error('加载图片失败')
      }
      const blob = await response.blob()
      blobUrl.value = URL.createObjectURL(blob)
    } catch (e) {
      error.value = e.message || '加载图片失败'
      blobUrl.value = ''
    } finally {
      loading.value = false
    }

    // Fetch metadata for dimensions
    try {
      const info = await previewAPI.get(file.id)
      const data = info?.data || info
      if (data.width && data.height) {
        dimensions.value = `${data.width} × ${data.height}`
      } else {
        dimensions.value = ''
      }
    } catch {
      dimensions.value = ''
    }
  }
}, { immediate: true })

watch(visible, (val) => {
  if (!val) {
    // Clean up blob URL when dialog closes
    if (blobUrl.value) {
      URL.revokeObjectURL(blobUrl.value)
      blobUrl.value = ''
    }
  }
})

onBeforeUnmount(() => {
  if (blobUrl.value) {
    URL.revokeObjectURL(blobUrl.value)
  }
})

function handleClose() {
  visible.value = false
}

function handleClosed() {
  if (blobUrl.value) {
    URL.revokeObjectURL(blobUrl.value)
    blobUrl.value = ''
  }
  dimensions.value = ''
  error.value = ''
  emit('closed')
}

function handleImageLoad(event) {
  // Image loaded successfully
}
</script>

<style scoped lang="scss">
.image-preview-dialog {
  :deep(.el-dialog) {
    background: transparent;
    box-shadow: none;
    max-width: 90vw;
  }

  :deep(.el-dialog__body) {
    padding: 0;
  }
}

.preview-container {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.loading-state,
.error-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 200px;
  min-height: 200px;
  color: white;
}

.error-state {
  color: #ff6b6b;
}

.close-btn {
  position: absolute;
  top: -40px;
  right: 0;
  width: 32px;
  height: 32px;
  border: none;
  background: rgba(255, 255, 255, 0.8);
  border-radius: 50%;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;

  &:hover {
    background: rgba(255, 255, 255, 1);
  }

  .el-icon {
    font-size: 20px;
    color: #333;
  }
}

.image-wrapper {
  max-width: 90vw;
  max-height: 80vh;
  overflow: hidden;

  img {
    max-width: 100%;
    max-height: 80vh;
    object-fit: contain;
    border-radius: 4px;
  }
}

.image-info {
  margin-top: 16px;
  text-align: center;

  .file-name {
    font-size: 16px;
    font-weight: 500;
    color: white;
    text-shadow: 0 1px 4px rgba(0, 0, 0, 0.5);
  }

  .file-meta {
    font-size: 14px;
    color: rgba(255, 255, 255, 0.8);
    text-shadow: 0 1px 4px rgba(0, 0, 0, 0.5);
    margin-top: 4px;
  }
}
</style>