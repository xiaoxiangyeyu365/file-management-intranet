<template>
  <div class="force-password-container">
    <div class="force-password-box">
      <div class="force-password-header">
        <h1>修改密码</h1>
        <p>首次登录需要修改默认密码</p>
      </div>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        @submit.prevent="handleSubmit"
      >
        <el-form-item prop="oldPassword">
          <el-input
            v-model="form.oldPassword"
            type="password"
            placeholder="当前密码"
            prefix-icon="Lock"
            size="large"
            show-password
          />
        </el-form-item>

        <el-form-item prop="newPassword">
          <el-input
            v-model="form.newPassword"
            type="password"
            placeholder="新密码（至少6位）"
            prefix-icon="Lock"
            size="large"
            show-password
          />
        </el-form-item>

        <el-form-item prop="confirmPassword">
          <el-input
            v-model="form.confirmPassword"
            type="password"
            placeholder="确认新密码"
            prefix-icon="Lock"
            size="large"
            show-password
            @keyup.enter="handleSubmit"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            style="width: 100%"
            @click="handleSubmit"
          >
            修改密码
          </el-button>
        </el-form-item>
      </el-form>

      <el-alert
        v-if="error"
        type="error"
        :title="error"
        show-icon
        :closable="false"
        style="margin-top: 16px"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref(null)
const loading = ref(false)
const error = ref('')

const form = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const validateConfirm = (rule, value, callback) => {
  if (value !== form.newPassword) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const rules = {
  oldPassword: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    { validator: validateConfirm, trigger: 'blur' }
  ]
}

async function handleSubmit() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
  } catch {
    return
  }

  loading.value = true
  error.value = ''

  try {
    await authStore.changePassword(form.oldPassword, form.newPassword)
    ElMessage.success('密码修改成功')
    router.push('/')
  } catch (err) {
    const msg = err.response?.data?.message || '修改失败'
    if (msg.includes('incorrect') || msg.includes('错误')) {
      error.value = '当前密码不正确'
    } else if (msg.includes('same')) {
      error.value = '新密码不能与当前密码相同'
    } else {
      error.value = msg
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped lang="scss">
.force-password-container {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.force-password-box {
  width: 400px;
  max-width: calc(100% - 32px);
  padding: 40px;
  background: white;
  border-radius: 12px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
}

.force-password-header {
  text-align: center;
  margin-bottom: 32px;

  h1 {
    font-size: 24px;
    font-weight: 600;
    color: #333;
    margin-bottom: 8px;
  }

  p {
    color: #e6a23c;
    font-size: 14px;
  }
}
</style>
