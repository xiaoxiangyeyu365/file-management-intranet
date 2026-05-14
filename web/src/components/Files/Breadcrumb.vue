<template>
  <div class="breadcrumb">
    <el-breadcrumb separator="/">
      <el-breadcrumb-item
        v-for="(item, index) in path"
        :key="item.id"
        @click="handleNavigate(index)"
      >
        <span v-if="index === 0" class="root-icon">
          <el-icon><House /></el-icon>
        </span>
        <span :class="{ clickable: index < path.length - 1 }">
          {{ item.name }}
        </span>
      </el-breadcrumb-item>
    </el-breadcrumb>
  </div>
</template>

<script setup>
import { House } from '@element-plus/icons-vue'

const props = defineProps({
  path: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['navigate'])

function handleNavigate(index) {
  if (index < props.path.length - 1) {
    emit('navigate', props.path[index])
  }
}
</script>

<style scoped lang="scss">
.breadcrumb {
  padding: 12px 0;
}

.root-icon {
  display: flex;
  align-items: center;
}

.clickable {
  cursor: pointer;

  &:hover {
    color: #409eff;
  }
}
</style>