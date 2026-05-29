<template>
  <div class="shares-view">
    <div class="page-header">
      <h2>我的分享</h2>
    </div>

    <el-table :data="sharesStore.myShares" v-loading="sharesStore.loading" stripe style="width: 100%">
      <el-table-column label="文件" min-width="160">
        <template #default="{ row }">
          <el-icon style="vertical-align: middle; margin-right: 4px">
            <Folder v-if="false" />
            <Document />
          </el-icon>
          文件 #{{ row.fileId }}
        </template>
      </el-table-column>

      <el-table-column label="创建时间" width="170">
        <template #default="{ row }">
          {{ formatDate(row.createdAt) }}
        </template>
      </el-table-column>

      <el-table-column label="过期时间" width="170">
        <template #default="{ row }">
          <template v-if="!row.expiresAt || !row.expiresAt.Valid">永久</template>
          <template v-else>{{ formatDate(row.expiresAt.Time) }}</template>
        </template>
      </el-table-column>

      <el-table-column label="下载次数" width="120" align="center">
        <template #default="{ row }">
          {{ row.downloadCount }}<template v-if="row.maxDownloads > 0"> / {{ row.maxDownloads }}</template>
        </template>
      </el-table-column>

      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="getStatus(row).type" size="small">{{ getStatus(row).text }}</el-tag>
        </template>
      </el-table-column>

      <el-table-column label="操作" width="160" align="center">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="copyLink(row)">复制链接</el-button>
          <el-button link type="danger" size="small" @click="handleRevoke(row)">撤销</el-button>
        </template>
      </el-table-column>

      <template #empty>
        <el-empty description="暂无分享记录" :image-size="80" />
      </template>
    </el-table>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { Document } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useSharesStore } from '../stores/shares'

const sharesStore = useSharesStore()

onMounted(() => {
  sharesStore.fetchMyShares()
})

function getStatus(share) {
  if (share.revoked) return { text: '已撤销', type: 'info' }
  if (share.maxDownloads > 0 && share.downloadCount >= share.maxDownloads) return { text: '已用完', type: 'warning' }
  if (share.expiresAt && share.expiresAt.Valid && new Date(share.expiresAt.Time) <= new Date()) return { text: '已过期', type: 'info' }
  return { text: '有效', type: 'success' }
}

function formatDate(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

async function copyLink(share) {
  const url = `${window.location.origin}/s/${share.token}`
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
    ElMessage.error('复制失败')
  }
}

async function handleRevoke(share) {
  try {
    await ElMessageBox.confirm('确定要撤销此分享吗？撤销后链接将无法访问', '撤销分享', { type: 'warning' })
    await sharesStore.revokeShare(share.id)
    ElMessage.success('已撤销')
  } catch {
    // cancelled
  }
}
</script>

<style scoped>
.shares-view {
  padding: 20px;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0;
  font-size: 18px;
}
</style>
