<template>
  <div class="admin-audit-view">
    <AppHeader />
    <div class="files-layout">
      <AppSidebar />
      <main class="files-main">
        <div class="page-header">
          <h2>审计日志</h2>
        </div>

        <div class="filter-bar">
          <el-select v-model="filters.action" placeholder="操作类型" clearable style="width: 180px" @change="fetchLogs">
            <el-option-group label="认证">
              <el-option label="登录" value="user.login" />
              <el-option label="登录失败" value="user.login_failed" />
              <el-option label="修改密码" value="user.change_password" />
            </el-option-group>
            <el-option-group label="文件">
              <el-option label="上传" value="file.upload" />
              <el-option label="删除" value="file.delete" />
              <el-option label="恢复" value="file.restore" />
              <el-option label="永久删除" value="file.permanent_delete" />
              <el-option label="重命名" value="file.rename" />
              <el-option label="移动" value="file.move" />
              <el-option label="下载" value="file.download" />
            </el-option-group>
            <el-option-group label="文件夹">
              <el-option label="创建文件夹" value="folder.create" />
            </el-option-group>
            <el-option-group label="回收站">
              <el-option label="清空回收站" value="trash.empty" />
            </el-option-group>
            <el-option-group label="分享">
              <el-option label="创建分享" value="share.create" />
              <el-option label="撤销分享" value="share.revoke" />
              <el-option label="分享下载" value="share.download" />
            </el-option-group>
            <el-option-group label="剪切板">
              <el-option label="创建记录" value="clipboard.create" />
              <el-option label="删除记录" value="clipboard.delete" />
            </el-option-group>
            <el-option-group label="管理">
              <el-option label="创建用户" value="admin.create_user" />
              <el-option label="删除用户" value="admin.delete_user" />
              <el-option label="重置密码" value="admin.reset_password" />
              <el-option label="修改状态" value="admin.update_status" />
              <el-option label="设置配额" value="admin.set_quota" />
            </el-option-group>
          </el-select>

          <el-select v-model="filters.targetType" placeholder="目标类型" clearable style="width: 140px" @change="fetchLogs">
            <el-option label="文件" value="file" />
            <el-option label="文件夹" value="folder" />
            <el-option label="用户" value="user" />
            <el-option label="分享" value="share" />
            <el-option label="剪切板" value="clipboard" />
            <el-option label="回收站" value="trash" />
          </el-select>

          <el-input v-model="filters.keyword" placeholder="搜索目标名称" clearable style="width: 200px" @clear="fetchLogs" @keyup.enter="fetchLogs" />

          <el-date-picker
            v-model="filters.dateRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            value-format="YYYY-MM-DDTHH:mm:ssZ"
            style="width: 380px"
            @change="fetchLogs"
          />

          <el-button type="primary" @click="fetchLogs">查询</el-button>
        </div>

        <el-table :data="logs" v-loading="loading" stripe>
          <el-table-column prop="createdAt" label="时间" width="180">
            <template #default="{ row }">{{ formatDate(row.createdAt) }}</template>
          </el-table-column>
          <el-table-column prop="username" label="操作者" width="120" />
          <el-table-column prop="action" label="操作" width="150">
            <template #default="{ row }">
              <el-tag :type="actionTagType(row.action)" size="small">{{ actionLabel(row.action) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="targetType" label="目标类型" width="100">
            <template #default="{ row }">{{ targetTypeLabel(row.targetType) }}</template>
          </el-table-column>
          <el-table-column prop="targetName" label="目标" min-width="150" />
          <el-table-column prop="ipAddress" label="IP" width="140" />
          <el-table-column prop="detail" label="详情" min-width="200">
            <template #default="{ row }">
              <span v-if="row.detail" class="detail-text">{{ row.detail }}</span>
              <span v-else class="detail-empty">-</span>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination-wrapper">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :total="total"
            :page-sizes="[20, 50, 100]"
            layout="total, sizes, prev, pager, next"
            @size-change="fetchLogs"
            @current-change="fetchLogs"
          />
        </div>
      </main>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { adminAPI } from '@/utils/api'
import AppHeader from '@/components/Layout/AppHeader.vue'
import AppSidebar from '@/components/Layout/AppSidebar.vue'

const logs = ref([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)

const filters = reactive({
  action: '',
  targetType: '',
  keyword: '',
  dateRange: null
})

function formatDate(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleString('zh-CN')
}

function actionLabel(action) {
  const map = {
    'user.login': '登录', 'user.login_failed': '登录失败', 'user.change_password': '修改密码',
    'file.upload': '上传', 'file.delete': '删除', 'file.restore': '恢复',
    'file.permanent_delete': '永久删除', 'file.rename': '重命名', 'file.move': '移动',
    'file.download': '下载', 'folder.create': '创建文件夹', 'trash.empty': '清空回收站',
    'share.create': '创建分享', 'share.revoke': '撤销分享', 'share.download': '分享下载',
    'clipboard.create': '创建剪切板', 'clipboard.delete': '删除剪切板',
    'admin.create_user': '创建用户', 'admin.delete_user': '删除用户',
    'admin.reset_password': '重置密码', 'admin.update_status': '修改状态', 'admin.set_quota': '设置配额'
  }
  return map[action] || action
}

function actionTagType(action) {
  if (action.startsWith('admin.')) return 'danger'
  if (action.includes('delete') || action.includes('failed')) return 'warning'
  if (action.includes('create') || action.includes('upload') || action === 'user.login') return 'success'
  return 'info'
}

function targetTypeLabel(type) {
  const map = { file: '文件', folder: '文件夹', user: '用户', share: '分享', clipboard: '剪切板', trash: '回收站' }
  return map[type] || type
}

async function fetchLogs() {
  loading.value = true
  try {
    const params = { page: page.value, pageSize: pageSize.value }
    if (filters.action) params.action = filters.action
    if (filters.targetType) params.targetType = filters.targetType
    if (filters.keyword) params.keyword = filters.keyword
    if (filters.dateRange && filters.dateRange.length === 2) {
      params.startDate = filters.dateRange[0]
      params.endDate = filters.dateRange[1]
    }
    const res = await adminAPI.listAuditLogs(params)
    const data = res.data
    logs.value = data.logs || []
    total.value = data.total || 0
  } catch (err) {
    logs.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

onMounted(() => { fetchLogs() })
</script>

<style scoped lang="scss">
.admin-audit-view {
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.files-layout {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.files-main {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;

  h2 {
    margin: 0;
    font-size: 20px;
    color: #303133;
  }
}

.filter-bar {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 20px;
  align-items: center;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}

.detail-text {
  font-size: 12px;
  color: #909399;
}

.detail-empty {
  color: #c0c4cc;
}
</style>
