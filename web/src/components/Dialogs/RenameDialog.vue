<template>
  <el-dialog
    v-model="visible"
    title="重命名"
    width="400px"
    @closed="handleClosed"
  >
    <el-form ref="formRef" :model="form" :rules="rules">
      <el-form-item label="新名称" prop="name">
        <el-input
          v-model="form.name"
          :placeholder="file?.name"
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
  file: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['update:modelValue', 'renamed'])

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
    { required: true, message: '请输入新名称', trigger: 'blur' },
    { pattern: /^[^\\/:*?"<>|]+$/, message: '名称不能包含特殊字符', trigger: 'blur' }
  ]
}

watch(visible, (val) => {
  if (val && props.file) {
    const ext = props.file.name.includes('.')
      ? '.' + props.file.name.split('.').pop()
      : ''
    const baseName = ext ? props.file.name.slice(0, -ext.length) : props.file.name
    form.name = baseName
    formRef.value?.clearValidate()
  }
})

async function handleSubmit() {
  if (!formRef.value || !props.file) return

  try {
    await formRef.value.validate()
  } catch {
    return
  }

  loading.value = true
  try {
    let newName = form.name
    if (!props.file.is_folder && props.file.name.includes('.')) {
      const ext = '.' + props.file.name.split('.').pop()
      if (!form.name.endsWith(ext)) {
        newName = form.name + ext
      }
    }

    await filesStore.renameFile(props.file.id, newName)
    ElMessage.success('重命名成功')
    emit('renamed', props.file.id, newName)
    visible.value = false
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '重命名失败')
  } finally {
    loading.value = false
  }
}

function handleClosed() {
  formRef.value?.resetFields()
}
</script>