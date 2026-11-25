<template>
  <el-row :gutter="16">
    <el-col :xs="24" :lg="14">
      <div class="polaris-card page-box">
        <div class="section-title">平台基础与全局被控策略</div>
        <el-form :model="form" label-width="150px">
          <el-form-item label="学校 / 平台名称">
            <el-input v-model="form.school_name" />
          </el-form-item>
          <el-form-item label="心跳离线判定 (秒)">
            <el-input v-model="form.heartbeat_timeout_sec" placeholder="默认 45 秒" />
          </el-form-item>
          <el-form-item label="默认验证方式">
            <el-select v-model="form.global_strategy['approve-mode']" style="width: 100%">
              <el-option label="仅密码验证（无人值守）" value="password" />
              <el-option label="密码或点击确认" value="both" />
              <el-option label="仅点击确认" value="click" />
            </el-select>
          </el-form-item>
          <el-form-item label="默认密码类型">
            <el-select v-model="form.global_strategy['verification-method']" style="width: 100%">
              <el-option label="仅使用固定密码" value="use-permanent-password" />
              <el-option label="同时支持固定与临时密码" value="use-both-passwords" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="saving" @click="saveSettings">保存系统设置</el-button>
          </el-form-item>
        </el-form>
      </div>
    </el-col>

    <el-col :xs="24" :lg="10">
      <div class="polaris-card page-box">
        <div class="section-title">修改当前登录密码</div>
        <el-form :model="pwdForm" label-width="100px">
          <el-form-item label="当前密码">
            <el-input v-model="pwdForm.old_password" type="password" show-password />
          </el-form-item>
          <el-form-item label="新密码">
            <el-input v-model="pwdForm.new_password" type="password" show-password />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="changePassword">更新密码</el-button>
          </el-form-item>
        </el-form>
      </div>
    </el-col>
  </el-row>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/client'

const saving = ref(false)
const form = reactive<any>({
  school_name: '',
  heartbeat_timeout_sec: '45',
  global_strategy: {
    'approve-mode': 'password',
    'verification-method': 'use-both-passwords'
  }
})

const pwdForm = reactive({
  old_password: '',
  new_password: ''
})

async function load() {
  const res: any = await api.get('/settings')
  form.school_name = res.school_name || ''
  form.heartbeat_timeout_sec = res.heartbeat_timeout_sec || '45'
  form.global_strategy = res.global_strategy || {
    'approve-mode': 'password',
    'verification-method': 'use-both-passwords'
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await api.put('/settings', form)
    ElMessage.success('系统设置已保存')
  } finally {
    saving.value = false
  }
}

async function changePassword() {
  await api.post('/change-password', pwdForm)
  ElMessage.success('密码修改成功')
  pwdForm.old_password = ''
  pwdForm.new_password = ''
}

onMounted(load)
</script>

<style scoped>
.page-box { padding: 20px 22px; margin-bottom: 16px; }
.section-title { font-size: 15px; font-weight: 600; margin-bottom: 18px; }
</style>
