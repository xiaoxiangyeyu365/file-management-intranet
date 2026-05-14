<template>
  <el-dialog
    v-model="visible"
    title="新建文件夹"
    width="400px"
    @closed="handleClosed"
  >
    <el-form ref="formRef" :model="form" :rules="rules">
      <el-form-item label="文件夹名称" prop="name">
        <el-input
          v-model="form.name"
          placeholder="请输入文件夹名称"
          autofocus
          @keyup.enter="handleSubmit"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="loading" @click="handleSubmit">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, watch, computed } from 'vue'
import { useFilesStore } from '@/stores/files'
import { ElMessage } from 'element-plus'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  parentId: {
    type: Number,
    default: 0
  }
})

const emit = defineEmits(['update:modelValue', 'created'])

const filesStore = useFilesStore()

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const formRef = ref(null)
const loading = ref(false)

const form = reactive({
  name: ''
})

const rules = {
  name: [
    { required: true, message: '请输入文件夹名称', trigger: 'blur' },
    { pattern: /^[^\\/:*?"<>|]+$/, message: '文件夹名称不能包含特殊字符', trigger: 'blur' }
  ]
}

watch(visible, (val) => {
  if (val) {
    form.name = ''
    formRef.value?.resetFields()
  }
})

async function handleSubmit() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
  } catch {
    return
  }

  loading.value = true
  try {
    const folder = await filesStore.createFolder(props.parentId || filesStore.currentFolder, form.name)
    ElMessage.success('文件夹创建成功')
    emit('created', folder)
    visible.value = false
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '创建失败')
  } finally {
    loading.value = false
  }
}

function handleClosed() {
  formRef.value?.resetFields()
}
</script>