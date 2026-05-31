<template>
  <aside class="app-sidebar">
    <nav class="sidebar-nav">
      <router-link
        to="/"
        class="nav-item"
        :class="{ active: route.path === '/' }"
      >
        <el-icon><Folder /></el-icon>
        <span>全部文件</span>
      </router-link>

      <router-link
        to="/trash"
        class="nav-item"
        :class="{ active: route.path === '/trash' }"
      >
        <el-icon><Delete /></el-icon>
        <span>回收站</span>
      </router-link>

      <router-link
        to="/clipboard"
        class="nav-item"
        :class="{ active: route.path === '/clipboard' }"
      >
        <el-icon><Document /></el-icon>
        <span>云剪切板</span>
      </router-link>

      <router-link
        to="/shares"
        class="nav-item"
        :class="{ active: route.path === '/shares' }"
      >
        <el-icon><Share /></el-icon>
        <span>我的分享</span>
      </router-link>

      <router-link
        v-if="authStore.isAdmin"
        to="/admin/users"
        class="nav-item"
        :class="{ active: route.path.startsWith('/admin') }"
      >
        <el-icon><User /></el-icon>
        <span>用户管理</span>
      </router-link>
    </nav>

    <div v-if="usedBytes > 0" class="storage-usage">
      <div class="storage-text">
        <span>{{ formatBytes(usedBytes) }} 已使用</span>
        <span v-if="effectiveQuota"> / {{ formatBytes(effectiveQuota) }}</span>
      </div>
      <el-progress v-if="effectiveQuota" :percentage="usagePercent()" :status="usageStatus()" :stroke-width="6" />
    </div>
  </aside>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { storageAPI } from '@/utils/api'
import { Folder, Delete, Document, User, Share } from '@element-plus/icons-vue'

const route = useRoute()
const authStore = useAuthStore()

const usedBytes = ref(0)
const effectiveQuota = ref(0)

onMounted(async () => {
  try {
    const res = await storageAPI.getUsage()
    usedBytes.value = res.data.usedBytes
    effectiveQuota.value = res.data.effectiveQuota
  } catch (e) { /* ignore */ }
})

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function usagePercent() {
  if (!effectiveQuota.value) return 0
  return Math.min(100, Math.round((usedBytes.value / effectiveQuota.value) * 100))
}

function usageStatus() {
  const pct = usagePercent()
  if (pct > 95) return 'danger'
  if (pct > 80) return 'warning'
  return ''
}
</script>

<style scoped lang="scss">
.app-sidebar {
  width: var(--sidebar-width);
  background: #fafafa;
  border-right: 1px solid #e4e7ed;
  padding: 16px 0;
}

.sidebar-nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 0 12px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  text-decoration: none;
  color: #606266;
  border-radius: 8px;
  transition: all 0.2s;

  &:hover {
    background: #f0f2f5;
    color: #409eff;
  }

  &.active {
    background: #ecf5ff;
    color: #409eff;
  }

  .el-icon {
    font-size: 20px;
  }

  span {
    font-size: 14px;
  }
}

.storage-usage {
  padding: 16px 12px;
  border-top: 1px solid #e4e7ed;
  margin-top: auto;
}

.storage-text {
  font-size: 12px;
  color: #909399;
  margin-bottom: 8px;
}

@media (max-width: 767px) {
  .app-sidebar {
    display: none;
  }
}
</style>
