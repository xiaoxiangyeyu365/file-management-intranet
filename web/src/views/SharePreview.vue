<template>
  <div class="share-preview">
    <el-card class="share-card" shadow="hover">
      <template v-if="loading">
        <el-skeleton :rows="4" animated />
      </template>

      <template v-else-if="error">
        <el-result :icon="errorIcon" :title="errorMessage" />
      </template>

      <template v-else-if="shareInfo">
        <div class="file-info">
          <el-icon :size="48" class="file-icon">
            <Folder v-if="shareInfo.isFolder" />
            <Document v-else />
          </el-icon>
          <h2 class="file-name">{{ shareInfo.fileName }}</h2>
          <p class="file-meta">
            <span v-if="shareInfo.fileSize">{{ formatFileSize(shareInfo.fileSize) }}</span>
            <span v-if="shareInfo.fileSize && shareInfo.createdAt"> · </span>
            <span v-if="shareInfo.createdAt">{{ formatDate(shareInfo.createdAt) }}</span>
          </p>
        </div>

        <!-- Password input -->
        <div v-if="shareInfo.hasPassword && !credential" class="password-section">
          <el-input
            v-model="password"
            type="password"
            placeholder="请输入访问密码"
            show-password
            @keyup.enter="handleVerify"
          >
            <template #prefix>
              <el-icon><Lock /></el-icon>
            </template>
          </el-input>
          <el-button type="primary" :loading="verifying" @click="handleVerify" style="margin-top: 12px; width: 100%">
            验证
          </el-button>
          <p v-if="passwordError" class="error-text">密码错误，请重试</p>
        </div>

        <!-- Download button -->
        <div v-if="credential" class="download-section">
          <el-button type="primary" size="large" @click="handleDownload" style="width: 100%">
            <el-icon><Download /></el-icon>
            下载文件
          </el-button>
        </div>
      </template>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { Folder, Document, Lock, Download } from '@element-plus/icons-vue'
import { useSharesStore } from '../stores/shares'

const route = useRoute()
const sharesStore = useSharesStore()

const loading = ref(true)
const error = ref(false)
const errorMessage = ref('')
const errorIcon = ref('error')
const shareInfo = ref(null)
const password = ref('')
const credential = ref('')
const verifying = ref(false)
const passwordError = ref(false)

onMounted(async () => {
  try {
    const data = await sharesStore.getShareInfo(route.params.token)
    shareInfo.value = data
    if (!data.hasPassword) {
      const cred = await sharesStore.verifyShare(route.params.token, '')
      credential.value = cred.credential
    }
  } catch (err) {
    error.value = true
    const status = err?.response?.status
    const msg = err?.response?.data?.message || ''
    if (status === 404 || msg.includes('not found')) {
      errorMessage.value = '该分享不存在'
      errorIcon.value = 'warning'
    } else if (msg.includes('expired')) {
      errorMessage.value = '该分享已过期'
      errorIcon.value = 'warning'
    } else if (msg.includes('revoked')) {
      errorMessage.value = '该分享已被撤销'
      errorIcon.value = 'error'
    } else if (msg.includes('limit') || msg.includes('reached')) {
      errorMessage.value = '下载次数已达上限'
      errorIcon.value = 'warning'
    } else {
      errorMessage.value = '该分享不存在或已失效'
      errorIcon.value = 'error'
    }
  } finally {
    loading.value = false
  }
})

async function handleVerify() {
  verifying.value = true
  passwordError.value = false
  try {
    const cred = await sharesStore.verifyShare(route.params.token, password.value)
    credential.value = cred.credential
  } catch (err) {
    const msg = err?.response?.data?.message || ''
    if (msg.includes('wrong password') || msg.includes('password')) {
      passwordError.value = true
    }
  } finally {
    verifying.value = false
  }
}

function handleDownload() {
  const url = sharesStore.getDownloadUrl(route.params.token, credential.value)
  window.open(url, '_blank')
}

function formatFileSize(bytes) {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatDate(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' })
}
</script>

<style scoped>
.share-preview {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
  padding: 20px;
}

.share-card {
  width: 100%;
  max-width: 480px;
}

.file-info {
  text-align: center;
  padding: 20px 0;
}

.file-icon {
  color: #409eff;
  margin-bottom: 16px;
}

.file-name {
  margin: 0 0 8px;
  font-size: 20px;
  word-break: break-all;
}

.file-meta {
  color: #909399;
  font-size: 14px;
  margin: 0;
}

.password-section {
  margin-top: 20px;
}

.error-text {
  color: #f56c6c;
  font-size: 13px;
  margin-top: 8px;
  text-align: center;
}

.download-section {
  margin-top: 24px;
}
</style>
