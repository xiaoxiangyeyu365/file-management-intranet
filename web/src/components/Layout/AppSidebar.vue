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
    </nav>
  </aside>
</template>

<script setup>
import { useRoute } from 'vue-router'
import { Folder, Delete } from '@element-plus/icons-vue'

const route = useRoute()
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
</style>
