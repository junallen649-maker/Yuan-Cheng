<template>
  <div class="polaris-card page-box">
    <div class="toolbar">
      <div class="desc">可创建全平台超管或归属具体学校的管理员/教师账号，实现按学校隔离管控</div>
      <el-button type="primary" @click="openDialog()">新增教师/管理员账号</el-button>
    </div>

    <el-table :data="users" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="65" />
      <el-table-column prop="username" label="用户名" width="135" />
      <el-table-column prop="display_name" label="姓名" width="130" />
      <el-table-column prop="school_name" label="所属学校范围" width="150">
        <template #default="{ row }">
          <el-tag size="small" :type="row.school_id === 0 ? 'danger' : 'primary'" effect="plain">
            {{ row.school_name || '全平台' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="角色" width="115">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'primary'" size="small">
            {{ row.role === 'admin' ? '管理员' : (row.role === 'teacher' ? '教师/运维' : '只读观察') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="85">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
            {{ row.status === 1 ? '正常' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="note" label="备注" min-width="160" />
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
        <el-form-item v-if="isSuperAdmin" label="所属学校">
          <el-select v-model="form.school_id" style="width: 100%">
            <el-option label="全平台超管（管理全部学校）" :value="0" />
            <el-option v-for="s in schools" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="角色权限">
          <el-select v-model="form.role" style="width: 100%">
            <el-option label="教师 / 机房运维 (teacher)" value="teacher" />
            <el-option label="管理员 (admin)" value="admin" />
            <el-option label="只读观察 (viewer)" value="viewer" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.role !== 'admin'" label="可管理机房">
          <el-select v-model="form.allowed_groups" multiple placeholder="留空默认可访问本校全部机房" style="width: 100%">
            <el-option v-for="g in schoolFilteredGroups" :key="g.id" :label="g.name" :value="g.id" />
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
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/client'

const loading = ref(false)
const users = ref<any[]>([])
const schools = ref<any[]>([])
const groups = ref<any[]>([])
const dialogVisible = ref(false)

const currentUser = computed(() => {
  try {
    return JSON.parse(localStorage.getItem('polaris_admin_user') || '{}')
  } catch {
    return {}
  }
})
const isSuperAdmin = computed(() => (currentUser.value?.school_id || 0) === 0 && (currentUser.value?.is_admin || currentUser.value?.role === 'admin'))

const form = reactive<any>({
  id: 0,
  username: '',
  display_name: '',
  email: '',
  password: '',
  role: 'teacher',
  school_id: 0,
  status: 1,
  allowed_groups: [],
  note: ''
})

const schoolFilteredGroups = computed(() => {
  if (!form.school_id) return groups.value
  return groups.value.filter((g) => g.school_id === form.school_id)
})

async function load() {
  loading.value = true
  try {
    const [uRes, sRes, gRes]: any = await Promise.all([api.get('/users'), api.get('/schools'), api.get('/groups')])
    users.value = uRes.items || []
    schools.value = sRes.items || []
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
    form.school_id = row.school_id ?? 0
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
    form.school_id = isSuperAdmin.value ? (schools.value[0]?.id || 1) : (currentUser.value?.school_id || 1)
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
