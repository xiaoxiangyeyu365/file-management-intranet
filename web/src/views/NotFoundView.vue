<template>
  <div class="not-found">
    <div class="not-found-content">
      <h1>404</h1>
      <p>页面不存在</p>
      <el-button type="primary" @click="router.push('/')">返回首页</el-button>
    </div>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
const router = useRouter()
</script>

<style scoped lang="scss">
.not-found {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  background: #f5f7fa;
}

.not-found-content {
  text-align: center;

  h1 {
    font-size: 72px;
    color: #409eff;
    margin: 0;
    font-weight: 700;
  }

  p {
    font-size: 18px;
    color: #606266;
    margin: 16px 0 32px;
  }
}
</style>
