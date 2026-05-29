<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:visible', $event)"
    :title="file?.isFolder ? '分享文件夹' : '分享文件'"
    :fullscreen="isMobile"
    width="480px"
    @open="resetState"
  >
    <template v-if="!createdShare">
      <el-form label-position="top">
        <el-form-item label="过期时间">
          <el-select v-model="form.expiresIn" style="width: 100%">
            <el-option :value="3600" label="1 小时" />
            <el-option :value="86400" label="1 天" />
            <el-option :value="604800" label="7 天" />
            <el-option :value="0" label="永久" />
          </el-select>
        </el-form-item>
        <el-form-item label="访问密码">
          <el-input v-model="form.password" type="password" placeholder="留空则无需密码" show-password />
        </el-form-item>
        <el-form-item label="下载次数限制">
          <el-input-number v-model="form.maxDownloads" :min="0" :step="1" style="width: 100%" />
          <div class="form-hint">0 表示不限制</div>
        </el-form-item>
      </el-form>
    </template>

    <template v-else>
      <el-result icon="success" title="分享链接已创建" style="padding: 10px 0" />
      <el-input :model-value="shareUrl" readonly>
        <template #append>
          <el-button @click="copyLink(shareUrl)">复制</el-button>
        </template>
      </el-input>
    </template>

    <template #footer>
      <template v-if="!createdShare">
        <el-button @click="$emit('update:visible', false)">取消</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreate">创建分享链接</el-button>
      </template>
      <template v-else>
        <el-button type="primary" @click="$emit('update:visible', false)">完成</el-button>
      </template>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useSharesStore } from '../../stores/shares'

const props = defineProps({
  visible: Boolean,
  file: Object,
})
const emit = defineEmits(['update:visible'])

const sharesStore = useSharesStore()

const isMobile = ref(false)
const creating = ref(false)
const createdShare = ref(null)

const form = ref({
  expiresIn: 86400,
  password: '',
  maxDownloads: 0,
})

const shareUrl = computed(() => {
  if (!createdShare.value) return ''
  return `${window.location.origin}/s/${createdShare.value.token}`
})

function checkMobile() {
  isMobile.value = window.innerWidth < 768
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})

function resetState() {
  form.value = { expiresIn: 86400, password: '', maxDownloads: 0 }
  createdShare.value = null
  creating.value = false
}

async function handleCreate() {
  if (!props.file) return
  creating.value = true
  try {
    const result = await sharesStore.createShare(props.file.id, {
      password: form.value.password,
      expiresIn: form.value.expiresIn,
      maxDownloads: form.value.maxDownloads,
    })
    createdShare.value = result
  } catch (err) {
    ElMessage.error('创建分享失败')
  } finally {
    creating.value = false
  }
}

async function copyLink(url) {
  try {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(url)
    } else {
      const textArea = document.createElement('textarea')
      textArea.value = url
      document.body.appendChild(textArea)
      textArea.select()
      document.execCommand('copy')
      document.body.removeChild(textArea)
    }
    ElMessage.success('链接已复制')
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}
</script>

<style scoped>
.form-hint {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
</style>
