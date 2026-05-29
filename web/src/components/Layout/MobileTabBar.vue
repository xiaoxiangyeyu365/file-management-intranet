<template>
  <div v-if="visible" class="mobile-tab-bar">
    <router-link
      to="/"
      class="tab-item"
      :class="{ active: route.path === '/' }"
    >
      <el-icon><Folder /></el-icon>
      <span>文件</span>
    </router-link>

    <router-link
      to="/clipboard"
      class="tab-item"
      :class="{ active: route.path === '/clipboard' }"
    >
      <el-icon><Document /></el-icon>
      <span>剪贴板</span>
    </router-link>

    <router-link
      to="/shares"
      class="tab-item"
      :class="{ active: route.path === '/shares' }"
    >
      <el-icon><Share /></el-icon>
      <span>分享</span>
    </router-link>

    <router-link
      to="/trash"
      class="tab-item"
      :class="{ active: route.path === '/trash' }"
    >
      <el-icon><Delete /></el-icon>
      <span>回收站</span>
    </router-link>

    <router-link
      v-if="authStore.isAdmin"
      to="/admin/users"
      class="tab-item"
      :class="{ active: route.path.startsWith('/admin') }"
    >
      <el-icon><Setting /></el-icon>
      <span>管理</span>
    </router-link>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useFilesStore } from '@/stores/files'
import { Folder, Document, Delete, Setting, Share } from '@element-plus/icons-vue'

const route = useRoute()
const authStore = useAuthStore()
const filesStore = useFilesStore()

const visible = computed(() => filesStore.selectedIds.length === 0)
</script>

<style scoped lang="scss">
.mobile-tab-bar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 100;
  height: 56px;
  background: #fff;
  border-top: 1px solid #e4e7ed;
  display: flex;
  align-items: center;
  justify-content: space-around;
}

.tab-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  text-decoration: none;
  color: #909399;
  font-size: 10px;
  padding: 4px 12px;
  transition: color 0.2s;

  .el-icon {
    font-size: 22px;
  }

  &.active {
    color: #409eff;
  }
}

@media (min-width: 768px) {
  .mobile-tab-bar {
    display: none;
  }
}
</style>
