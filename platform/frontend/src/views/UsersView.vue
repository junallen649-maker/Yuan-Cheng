<template>
  <div class="polaris-card page-box">
    <div class="toolbar">
      <div class="desc">此处的账号同时可用于登录「Web 管理后台」与「Polaris Console 控制端」同步机房设备</div>
      <el-button type="primary" @click="openDialog()">新增教师/管理员账号</el-button>
    </div>

    <el-table :data="users" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" label="用户名" width="140" />
      <el-table-column prop="display_name" label="姓名" width="140" />
      <el-table-column prop="email" label="邮箱" width="190" />
      <el-table-column label="角色" width="120">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'primary'" size="small">
            {{ row.role === 'admin' ? '超级管理员' : (row.role === 'teacher' ? '教师/运维' : '只读观察') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
            {{ row.status === 1 ? '正常' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="note" label="备注" min-width="180" />
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDialog(row)">编辑</el-button>
          <el-popconfirm title="确认删除该账号？" @confirm="deleteUser(row.id)">
            <template #reference>
              <el-button link type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑账号' : '新增账号'" width="480px">
      <el-form :model="form" label-width="95px">
        <el-form-item label="用户名">
          <el-input v-model="form.username" :disabled="Boolean(form.id)" />
        </el-form-item>
        <el-form-item label="显示姓名">
          <el-input v-model="form.display_name" placeholder="例如：张老师" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="form.email" />
        </el-form-item>
        <el-form-item :label="form.id ? '重置密码' : '登录密码'">
          <el-input v-model="form.password" type="password" show-password placeholder="留空表示不修改（至少6位）" />
        </el-form-item>
        <el-form-item label="角色权限">
          <el-select v-model="form.role" style="width: 100%">
            <el-option label="教师 / 机房运维 (teacher)" value="teacher" />
            <el-option label="超级管理员 (admin)" value="admin" />
            <el-option label="只读观察 (viewer)" value="viewer" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.role !== 'admin'" label="可管理机房">
          <el-select v-model="form.allowed_groups" multiple placeholder="留空默认可访问全部" style="width: 100%">
            <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.id" label="状态">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="停用" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.note" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveUser">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/client'

const loading = ref(false)
const users = ref<any[]>([])
const groups = ref<any[]>([])
const dialogVisible = ref(false)

const form = reactive<any>({
  id: 0,
  username: '',
  display_name: '',
  email: '',
  password: '',
  role: 'teacher',
  status: 1,
  allowed_groups: [],
  note: ''
})

async function load() {
  loading.value = true
  try {
    const [uRes, gRes]: any = await Promise.all([api.get('/users'), api.get('/groups')])
    users.value = uRes.items || []
    groups.value = gRes.items || []
  } finally {
    loading.value = false
  }
}

function openDialog(row?: any) {
  if (row) {
    form.id = row.id
    form.username = row.username
    form.display_name = row.display_name || ''
    form.email = row.email || ''
    form.password = ''
    form.role = row.role || 'teacher'
    form.status = row.status ?? 1
    form.allowed_groups = row.allowed_groups || []
    form.note = row.note || ''
  } else {
    form.id = 0
    form.username = ''
    form.display_name = ''
    form.email = ''
    form.password = ''
    form.role = 'teacher'
    form.status = 1
    form.allowed_groups = []
    form.note = ''
  }
  dialogVisible.value = true
}

async function saveUser() {
  const payload: any = { ...form }
  if (form.id && !payload.password) {
    delete payload.password
  }
  if (form.id) {
    await api.put(`/users/${form.id}`, payload)
  } else {
    await api.post('/users', payload)
  }
  ElMessage.success('账号保存成功')
  dialogVisible.value = false
  load()
}

async function deleteUser(id: number) {
  await api.delete(`/users/${id}`)
  ElMessage.success('已删除账号')
  load()
}

onMounted(load)
</script>

<style scoped>
.page-box { padding: 18px 20px; }
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.desc {
  font-size: 13px;
  color: var(--polaris-muted);
}
</style>
