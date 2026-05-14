<template>
  <el-dialog
    v-model="visible"
    :title="title"
    width="400px"
  >
    <div class="confirm-content">
      <el-icon :size="24" :color="iconColor" class="confirm-icon">
        <component :is="iconComponent" />
      </el-icon>
      <span>{{ message }}</span>
    </div>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button :type="type" :loading="loading" @click="handleConfirm">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed } from 'vue'
import { WarningFilled, CircleCheckFilled, CircleCloseFilled } from '@element-plus/icons-vue'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  title: {
    type: String,
    default: '确认'
  },
  message: {
    type: String,
    default: '确定要执行此操作吗？'
  },
  type: {
    type: String,
    default: 'primary'
  }
})

const emit = defineEmits(['update:modelValue', 'confirm'])

const loading = ref(false)

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const iconComponent = computed(() => {
  switch (props.type) {
    case 'danger':
      return CircleCloseFilled
    case 'success':
      return CircleCheckFilled
    default:
      return WarningFilled
  }
})

const iconColor = computed(() => {
  switch (props.type) {
    case 'danger':
      return '#f56c6c'
    case 'success':
      return '#67c23a'
    default:
      return '#e6a23c'
  }
})

function handleConfirm() {
  loading.value = true
  emit('confirm')
}
</script>

<style scoped lang="scss">
.confirm-content {
  display: flex;
  align-items: center;
  gap: 16px;

  .confirm-icon {
    flex-shrink: 0;
  }
}
</style>