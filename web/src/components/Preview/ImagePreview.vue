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

      <div class="image-wrapper">
        <img :src="imageUrl" :alt="file?.name" @load="handleImageLoad" />
      </div>

      <div v-if="file" class="image-info">
        <div class="file-name">{{ file.name }}</div>
        <div class="file-meta">
          <span v-if="dimensions">{{ dimensions }}</span>
          <span v-if="dimensions && file.size"> • </span>
          <span>{{ formatSize(file.size) }}</span>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { fileAPI, previewAPI } from '@/utils/api'
import { formatSize } from '@/utils/format'
import { Close } from '@element-plus/icons-vue'

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

const dimensions = ref('')

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const imageUrl = computed(() => {
  if (!props.file) return ''
  return fileAPI.downloadUrl(props.file.id) + '?t=' + Date.now()
})

watch(() => props.file, async (file) => {
  if (file && !file.is_folder) {
    try {
      const info = await previewAPI.get(file.id)
      if (info.width && info.height) {
        dimensions.value = `${info.width} × ${info.height}`
      }
    } catch {
      dimensions.value = ''
    }
  }
})

function handleClose() {
  visible.value = false
}

function handleClosed() {
  dimensions.value = ''
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