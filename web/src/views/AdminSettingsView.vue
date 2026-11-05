<template>
  <div class="settings-view">
    <AppHeader />
    <div class="settings-layout">
      <AppSidebar />
      <main class="settings-main">
        <div class="page-header">
          <h2>系统设置</h2>
        </div>

        <div v-loading="loading" class="settings-content">
          <el-form :model="form" label-width="160px" class="settings-form">
            <!-- Disk Quota -->
            <el-divider content-position="left">磁盘配额</el-divider>
            <el-form-item label="默认配额 (GB)">
              <el-input-number v-model="form.defaultQuotaGB" :min="0" :max="100000" />
              <span class="form-hint">新用户的默认磁盘配额。0 表示无限制。</span>
            </el-form-item>

            <!-- Registration -->
            <el-divider content-position="left">注册设置</el-divider>
            <el-form-item label="允许公开注册">
              <el-switch v-model="form.registration" />
              <span class="form-hint">关闭后新用户无法自行注册。</span>
            </el-form-item>
            <el-form-item label="注册需审批">
              <el-switch v-model="form.approvalRequired" />
              <span class="form-hint">开启后新注册用户需管理员审批才能登录。</span>
            </el-form-item>

            <!-- Share -->
            <el-divider content-position="left">文件分享</el-divider>
            <el-form-item label="分享 Token 长度">
              <el-input-number v-model="form.tokenLength" :min="6" :max="64" />
              <span class="form-hint">分享链接中 Token 的字符数（6-64），越长越安全。</span>
            </el-form-item>

            <!-- Audit -->
            <el-divider content-position="left">审计日志</el-divider>
            <el-form-item label="日志保留天数">
              <el-input-number v-model="form.retentionDays" :min="0" :max="3650" />
              <span class="form-hint">超过此天数的审计日志将被自动清理。0 表示不清理。</span>
            </el-form-item>

            <!-- AI Status (read-only) -->
            <el-divider content-position="left">AI 配置状态（只读）</el-divider>
            <el-form-item label="AI 摘要">
              <el-tag :type="settings.ai?.enabled ? 'success' : 'info'">
                {{ settings.ai?.enabled ? '已启用' : '未启用' }}
              </el-tag>
              <span v-if="settings.ai?.model" class="form-hint">模型: {{ settings.ai.model }}</span>
            </el-form-item>
            <el-form-item label="RAG 知识库">
              <el-tag :type="settings.rag?.enabled ? 'success' : 'info'">
                {{ settings.rag?.enabled ? '已启用' : '未启用' }}
              </el-tag>
            </el-form-item>
            <el-form-item label="知识库索引">
              <div class="index-stats">
                <span v-if="indexStats">
                  物理文件 {{ indexStats.totalPhysical }} · 已索引 {{ indexStats.indexedFiles }}
                  · 待索引 {{ indexStats.unindexedSupported }} · 片段 {{ indexStats.totalChunks }}
                  · 有摘要 {{ indexStats.summarizedFiles }}
                </span>
                <span v-else class="form-hint">暂无统计</span>
              </div>
              <div class="index-actions">
                <el-button size="small" @click="fetchIndexStats">刷新统计</el-button>
                <el-button
                  size="small"
                  type="primary"
                  :loading="reindexing"
                  :disabled="!settings.rag?.enabled || !indexStats?.unindexedSupported"
                  @click="handleReindexAll"
                >
                  为未索引文件建立索引
                </el-button>
              </div>
              <span class="form-hint">索引在后台执行；大文件可能需要几分钟，完成后刷新统计查看。</span>
            </el-form-item>

            <!-- TLS Status -->
            <el-form-item label="HTTPS (TLS)">
              <el-tag :type="settings.tls?.enabled ? 'success' : 'warning'">
                {{ settings.tls?.enabled ? '已启用' : '未启用' }}
              </el-tag>
              <span v-if="settings.tls?.enabled" class="form-hint">端口: {{ settings.tls.port }}</span>
            </el-form-item>

            <!-- Submit -->
            <el-form-item>
              <el-button type="primary" @click="handleSave" :loading="saving">保存设置</el-button>
            </el-form-item>
          </el-form>
        </div>
      </main>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { adminAPI } from '@/utils/api'
import { ElMessage, ElMessageBox } from 'element-plus'
import AppHeader from '@/components/Layout/AppHeader.vue'
import AppSidebar from '@/components/Layout/AppSidebar.vue'

const loading = ref(false)
const saving = ref(false)
const settings = ref({})
const indexStats = ref(null)
const reindexing = ref(false)

const form = reactive({
  defaultQuotaGB: 10,
  registration: true,
  approvalRequired: true,
  tokenLength: 8,
  retentionDays: 90
})

onMounted(async () => {
  await fetchSettings()
  fetchIndexStats()
})

async function fetchSettings() {
  loading.value = true
  try {
    const res = await adminAPI.getSettings()
    const data = res?.data || res
    settings.value = data
    form.defaultQuotaGB = Math.round((data.disk?.defaultQuota || 0) / (1024 * 1024 * 1024))
    form.registration = data.auth?.registration ?? true
    form.approvalRequired = data.auth?.approvalRequired ?? true
    form.tokenLength = data.share?.tokenLength ?? 8
    form.retentionDays = data.audit?.retentionDays ?? 90
  } catch (err) {
    ElMessage.error('加载设置失败')
  } finally {
    loading.value = false
  }
}

async function fetchIndexStats() {
  try {
    const res = await adminAPI.getIndexStats()
    const data = res?.data || res
    if (data && data.code !== undefined && data.code !== 0) return
    indexStats.value = data
  } catch (err) {
    // RAG may be disabled; the section simply stays without stats.
  }
}

async function handleReindexAll() {
  try {
    await ElMessageBox.confirm(
      '为所有未索引的文本 / PDF / DOCX 文件建立索引？该操作会调用 embedding 接口，可能产生费用。',
      '建立索引',
      { confirmButtonText: '开始', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }

  reindexing.value = true
  try {
    const res = await adminAPI.reindexAll()
    const data = res?.data || res
    if (data && data.code !== undefined && data.code !== 0) {
      throw new Error(data.message || '提交失败')
    }
    ElMessage.success(`已提交 ${data.queued ?? 0} 个文件，后台处理中`)
    setTimeout(fetchIndexStats, 5000)
  } catch (err) {
    ElMessage.error(err.response?.data?.message || err.message || '提交失败')
  } finally {
    reindexing.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    await adminAPI.updateSettings({
      defaultQuota: form.defaultQuotaGB * 1024 * 1024 * 1024,
      registration: form.registration,
      approvalRequired: form.approvalRequired,
      tokenLength: form.tokenLength,
      retentionDays: form.retentionDays
    })
    ElMessage.success('设置已保存')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped lang="scss">
.settings-view {
  min-height: 100vh;
  background: #f5f7fa;
}

.settings-layout {
  display: flex;
  min-height: calc(100vh - 56px);
}

.settings-main {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
}

.page-header {
  margin-bottom: 24px;

  h2 {
    margin: 0;
    font-size: 20px;
    color: #303133;
  }
}

.settings-content {
  max-width: 700px;
}

.settings-form {
  background: #fff;
  padding: 24px;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
}

.form-hint {
  margin-left: 12px;
  color: #909399;
  font-size: 13px;
}

.index-stats {
  width: 100%;
  margin-bottom: 8px;
  color: #606266;
  font-size: 13px;
  line-height: 1.6;
}

.index-actions {
  display: flex;
  gap: 8px;
}

:deep(.el-divider__text) {
  font-weight: 600;
  font-size: 15px;
}
</style>
