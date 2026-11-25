<template>
  <div class="login-page">
    <div class="login-card">
      <div class="brand-header">
        <div class="brand-icon">
          <el-icon :size="26" color="#fff"><Monitor /></el-icon>
        </div>
        <div>
          <h1>Polaris 校园远控平台</h1>
          <p>机房设备统一监控 · 无人值守运维后台</p>
        </div>
      </div>

      <el-form ref="formRef" :model="form" :rules="rules" size="large" @submit.prevent="onSubmit">
        <el-form-item prop="username">
          <el-input v-model="form.username" placeholder="管理员 / 教师账号" prefix-icon="User" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="请输入密码"
            prefix-icon="Lock"
            show-password
            @keyup.enter="onSubmit"
          />
        </el-form-item>
        <el-button type="primary" class="submit-btn" :loading="loading" @click="onSubmit">
          登录管理后台
        </el-button>
      </el-form>

      <div class="login-tip">
        默认管理员：<code>admin</code> / <code>polaris2026</code>（首次登录后建议修改）
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import api from '@/api/client'

const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({
  username: '',
  password: ''
})

const rules = {
  username: [{ required: true, message: '请输入账号', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

async function onSubmit() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    loading.value = true
    try {
      const res: any = await api.post('/login', form)
      localStorage.setItem('polaris_admin_token', res.token)
      localStorage.setItem('polaris_admin_user', JSON.stringify(res.user))
      ElMessage.success('登录成功')
      router.push('/overview')
    } finally {
      loading.value = false
    }
  })
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: radial-gradient(circle at 20% 20%, #e0ecff 0%, #f4f7fb 50%, #eef2ff 100%);
  padding: 20px;
}

.login-card {
  width: 420px;
  max-width: 100%;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  padding: 36px 32px 28px;
  box-shadow: 0 20px 50px rgba(30, 111, 255, 0.08);
}

.brand-header {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 28px;
}

.brand-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  background: linear-gradient(135deg, #3b82f6, #1e6fff);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 6px 16px rgba(30, 111, 255, 0.3);
}

.brand-header h1 {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}

.brand-header p {
  margin: 4px 0 0;
  font-size: 12px;
  color: #64748b;
}

.submit-btn {
  width: 100%;
  height: 44px;
  font-size: 15px;
  font-weight: 600;
  border-radius: 10px;
  margin-top: 6px;
}

.login-tip {
  margin-top: 18px;
  text-align: center;
  font-size: 12px;
  color: #64748b;
}

.login-tip code {
  background: #f1f5f9;
  padding: 2px 6px;
  border-radius: 4px;
  color: #1e293b;
}
</style>
