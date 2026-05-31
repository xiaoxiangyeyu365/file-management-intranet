<template>
  <div class="admin-users-view">
    <AppHeader />

    <div class="files-layout">
      <AppSidebar />

      <main class="files-main">
        <div class="page-header">
          <h2>用户管理</h2>
          <el-button type="primary" @click="showCreateDialog = true">
            创建用户
          </el-button>
        </div>

        <div class="filter-tabs">
      <el-radio-group v-model="statusFilter" @change="fetchUsers">
        <el-radio-button label="">全部</el-radio-button>
        <el-radio-button label="pending">
          待审批
          <el-badge v-if="adminStore.pendingCount > 0" :value="adminStore.pendingCount" class="tab-badge" />
        </el-radio-button>
        <el-radio-button label="approved">正常</el-radio-button>
        <el-radio-button label="disabled">已禁用</el-radio-button>
      </el-radio-group>
    </div>

    <el-table :data="adminStore.users" v-loading="adminStore.loading" stripe>
      <el-table-column prop="username" label="用户名" width="180" />
      <el-table-column prop="role" label="角色" width="120">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'info'" size="small">
            {{ row.role === 'admin' ? '管理员' : '普通用户' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="120">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.status)" size="small">
            {{ statusLabel(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="passwordChanged" label="密码状态" width="120">
        <template #default="{ row }">
          <el-tag :type="row.passwordChanged ? 'success' : 'warning'" size="small">
            {{ row.passwordChanged ? '已修改' : '默认密码' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="存储" width="180">
        <template #default="{ row }">
          <span>{{ formatBytes(row.usedBytes) }}</span>
          <span v-if="row.diskQuota"> / {{ formatBytes(row.diskQuota) }}</span>
          <span v-else> / 全局默认</span>
        </template>
      </el-table-column>
      <el-table-column prop="createdAt" label="创建时间" width="180">
        <template #default="{ row }">
          {{ formatDate(row.createdAt) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" min-width="240" fixed="right">
        <template #default="{ row }">
          <el-popover trigger="click" width="280" placement="left">
            <template #reference>
              <el-button size="small" link>配额</el-button>
            </template>
            <div>
              <el-radio-group v-model="quotaMode" style="display: flex; flex-direction: column; gap: 8px;">
                <el-radio value="global">使用全局默认</el-radio>
                <el-radio value="unlimited">无限制</el-radio>
                <el-radio value="custom">自定义</el-radio>
              </el-radio-group>
              <el-input
                v-if="quotaMode === 'custom'"
                v-model="customQuotaGB"
                type="number"
                min="0.1"
                step="0.1"
                placeholder="GB"
                style="margin-top: 8px;"
              >
                <template #append>GB</template>
              </el-input>
              <el-button type="primary" size="small" style="margin-top: 8px; width: 100%;" @click="handleQuotaChange(row)">
                保存
              </el-button>
            </div>
          </el-popover>
          <el-button
            v-if="row.status === 'pending'"
            type="success"
            size="small"
            @click="handleApprove(row)"
          >
            审批
          </el-button>
          <el-button
            v-if="row.status === 'approved'"
            type="warning"
            size="small"
            @click="handleDisable(row)"
          >
            禁用
          </el-button>
          <el-button
            v-if="row.status === 'disabled'"
            type="success"
            size="small"
            @click="handleEnable(row)"
          >
            启用
          </el-button>
          <el-button
            size="small"
            @click="showResetDialog(row)"
          >
            重置密码
          </el-button>
          <el-button
            v-if="row.id !== currentUserId"
            type="danger"
            size="small"
            @click="handleDelete(row)"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Create User Dialog -->
    <el-dialog v-model="showCreateDialog" title="创建用户" width="400">
      <el-form ref="createFormRef" :model="createForm" :rules="createRules">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="createForm.username" placeholder="3-50个字符" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="createForm.password" type="password" placeholder="至少6位" show-password />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-select v-model="createForm.role" style="width: 100%">
            <el-option label="普通用户" value="user" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" :loading="createLoading" @click="handleCreate">创建</el-button>
      </template>
    </el-dialog>

    <!-- Reset Password Dialog -->
    <el-dialog v-model="showResetDialogVisible" title="重置密码" width="400">
      <el-form ref="resetFormRef" :model="resetForm" :rules="resetRules">
        <el-form-item label="新密码" prop="newPassword">
          <el-input v-model="resetForm.newPassword" type="password" placeholder="至少6位" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showResetDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="resetLoading" @click="handleResetPassword">确认</el-button>
      </template>
    </el-dialog>
      </main>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin'
import { useAuthStore } from '@/stores/auth'
import { ElMessage, ElMessageBox } from 'element-plus'
import axios from 'axios'
import AppHeader from '@/components/Layout/AppHeader.vue'
import AppSidebar from '@/components/Layout/AppSidebar.vue'

const adminStore = useAdminStore()
const authStore = useAuthStore()

const statusFilter = ref('')
const currentUserId = computed(() => authStore.user?.id)

// Create dialog
const showCreateDialog = ref(false)
const createLoading = ref(false)
const createFormRef = ref(null)
const createForm = reactive({
  username: '',
  password: '',
  role: 'user'
})
const createRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 50, message: '长度为3-50个字符', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_]+$/, message: '只能包含字母、数字和下划线', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' }
  ]
}

// Reset dialog
const showResetDialogVisible = ref(false)
const resetLoading = ref(false)
const resetFormRef = ref(null)
const resetTargetUser = ref(null)
const resetForm = reactive({ newPassword: '' })
const resetRules = {
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' }
  ]
}

function fetchUsers() {
  adminStore.fetchUsers(statusFilter.value)
}

function statusTagType(status) {
  switch (status) {
    case 'approved': return 'success'
    case 'pending': return 'warning'
    case 'disabled': return 'danger'
    default: return 'info'
  }
}

function statusLabel(status) {
  switch (status) {
    case 'approved': return '正常'
    case 'pending': return '待审批'
    case 'disabled': return '已禁用'
    default: return status
  }
}

function formatDate(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleString('zh-CN')
}

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

const quotaMode = ref('global')
const customQuotaGB = ref(10)

async function handleQuotaChange(user) {
  let quota = null
  if (quotaMode.value === 'unlimited') quota = 0
  else if (quotaMode.value === 'custom') quota = Math.round(customQuotaGB.value * 1024 * 1024 * 1024)

  try {
    await axios.put(`/admin/users/${user.id}/quota`, { disk_quota: quota })
    ElMessage.success('配额已更新')
    fetchUsers()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '更新失败')
  }
}

async function handleApprove(user) {
  await adminStore.updateUser(user.id, { status: 'approved' })
  ElMessage.success('已审批通过')
}

async function handleDisable(user) {
  await adminStore.updateUser(user.id, { status: 'disabled' })
  ElMessage.success('已禁用')
}

async function handleEnable(user) {
  await adminStore.updateUser(user.id, { status: 'approved' })
  ElMessage.success('已启用')
}

function showResetDialog(user) {
  resetTargetUser.value = user
  resetForm.newPassword = ''
  showResetDialogVisible.value = true
}

async function handleResetPassword() {
  if (!resetFormRef.value) return
  try {
    await resetFormRef.value.validate()
  } catch { return }

  resetLoading.value = true
  try {
    await adminStore.resetPassword(resetTargetUser.value.id, resetForm.newPassword)
    showResetDialogVisible.value = false
    ElMessage.success('密码已重置，用户下次登录需修改密码')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '重置失败')
  } finally {
    resetLoading.value = false
  }
}

async function handleCreate() {
  if (!createFormRef.value) return
  try {
    await createFormRef.value.validate()
  } catch { return }

  createLoading.value = true
  try {
    await adminStore.createUser(createForm.username, createForm.password, createForm.role)
    showCreateDialog.value = false
    createForm.username = ''
    createForm.password = ''
    createForm.role = 'user'
    ElMessage.success('用户创建成功')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '创建失败')
  } finally {
    createLoading.value = false
  }
}

async function handleDelete(user) {
  try {
    await ElMessageBox.confirm(
      `删除用户 "${user.username}" 将同时永久删除其所有文件，且不可恢复。是否继续？`,
      '确认删除用户',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' }
    )

    const result = await adminStore.deleteUser(user.id)
    const data = result?.data || result
    ElMessage.success(`已删除用户，共删除 ${data?.deletedFiles || 0} 个文件，${data?.deletedFolders || 0} 个文件夹`)
  } catch (err) {
    if (err !== 'cancel') {
      ElMessage.error(err.response?.data?.message || '删除失败')
    }
  }
}

onMounted(() => {
  fetchUsers()
})
</script>

<style scoped lang="scss">
.admin-users-view {
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

.filter-tabs {
  margin-bottom: 20px;
}

.tab-badge {
  margin-left: 8px;
}
</style>
