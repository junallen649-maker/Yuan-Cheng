<template>
  <div class="polaris-card page-box">
    <div class="toolbar">
      <div class="left-tools">
        <el-select v-if="isSuperAdmin" v-model="filterSchoolId" style="width: 170px" @change="load">
          <el-option label="全部学校" :value="0" />
          <el-option v-for="s in schools" :key="s.id" :label="s.name" :value="s.id" />
        </el-select>
        <div class="desc">
          按学校与机房分类管理策略；<b>双击“所属学校”或“分组名称”可直接改名</b>
        </div>
      </div>
      <el-button type="primary" @click="openDialog()">新建机房分组</el-button>
    </div>

    <el-table :data="groups" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="65" />
      <el-table-column label="所属学校（双击可改校名）" min-width="165">
        <template #default="{ row }">
          <div v-if="editingSchoolRowId === row.id">
            <el-input
              ref="schoolInputRef"
              v-model="editingSchoolName"
              size="small"
              @keyup.enter="saveInlineSchoolRename(row)"
              @keyup.esc="editingSchoolRowId = null"
              @blur="saveInlineSchoolRename(row)"
            />
          </div>
          <div v-else class="dbl-rename-cell" title="双击修改学校名称" @dblclick="startInlineSchoolRename(row)">
            <el-tag size="small" type="primary" effect="plain">{{ row.school_name || '默认学校' }}</el-tag>
            <el-icon class="edit-hint"><EditPen /></el-icon>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="分组名称（双击可改名）" min-width="180">
        <template #default="{ row }">
          <div v-if="editingGroupRowId === row.id">
            <el-input
              ref="groupInputRef"
              v-model="editingGroupName"
              size="small"
              @keyup.enter="saveInlineGroupRename(row)"
              @keyup.esc="editingGroupRowId = null"
              @blur="saveInlineGroupRename(row)"
            />
          </div>
          <div v-else class="dbl-rename-cell" title="双击修改分组名称" @dblclick="startInlineGroupRename(row)">
            <span class="bold">{{ row.name }}</span>
            <el-icon class="edit-hint"><EditPen /></el-icon>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="说明" min-width="170" />
      <el-table-column prop="device_count" label="设备数" width="80" align="center" />
      <el-table-column label="验证模式" width="160">
        <template #default="{ row }">
          <el-tag size="small" effect="plain">
            {{ row.strategy?.['approve-mode'] || '默认' }} / {{ row.strategy?.['verification-method'] || '默认' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="权限策略" min-width="190">
        <template #default="{ row }">
          <el-tag v-if="row.strategy?.['enable-file-transfer'] === 'N'" size="small" type="info" style="margin-right: 4px">禁传文件</el-tag>
          <el-tag v-if="row.strategy?.['enable-clipboard'] === 'N'" size="small" type="info" style="margin-right: 4px">禁剪贴板</el-tag>
          <el-tag v-if="row.strategy?.['disable-settings'] === 'Y'" size="small" type="warning">锁定设置</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDialog(row)">配置策略</el-button>
          <el-button link type="warning" @click="openGroupPasswordDialog(row)">整组改密</el-button>
          <el-popconfirm
            v-if="row.id > 2"
            title="删除后该分组设备将移回本校默认机房，确认删除？"
            @confirm="deleteGroup(row.id)"
          >
            <template #reference>
              <el-button link type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="form.id ? '配置分组与策略' : '新建机房分组'" width="540px">
      <el-form :model="form" label-width="135px">
        <el-form-item v-if="isSuperAdmin" label="所属学校">
          <el-select v-model="form.school_id" style="width: 100%">
            <el-option v-for="s in schools" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="分组名称">
          <el-input v-model="form.name" placeholder="例如：一号教学楼-101机房" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="form.description" />
        </el-form-item>
        <el-form-item label="运维密码备注">
          <el-input v-model="form.unattended_pass" placeholder="可选：记录该机房统一固定密码供教师查阅" />
        </el-form-item>
        <el-divider>下发至终端策略 (Strategy)</el-divider>
        <el-form-item label="接入验证模式">
          <el-select v-model="form.strategy['approve-mode']" style="width: 100%">
            <el-option label="仅密码验证（无人值守免点击确认）" value="password" />
            <el-option label="密码或点击确认均可" value="both" />
            <el-option label="仅点击确认" value="click" />
          </el-select>
        </el-form-item>
        <el-form-item label="密码类型">
          <el-select v-model="form.strategy['verification-method']" style="width: 100%">
            <el-option label="同时支持固定与临时密码" value="use-both-passwords" />
            <el-option label="仅使用固定密码" value="use-permanent-password" />
            <el-option label="仅使用临时密码" value="use-temporary-password" />
          </el-select>
        </el-form-item>
        <el-form-item label="允许文件传输">
          <el-switch v-model="form.strategy['enable-file-transfer']" active-value="Y" inactive-value="N" />
        </el-form-item>
        <el-form-item label="允许剪贴板同步">
          <el-switch v-model="form.strategy['enable-clipboard']" active-value="Y" inactive-value="N" />
        </el-form-item>
        <el-form-item label="允许远程终端">
          <el-switch v-model="form.strategy['enable-terminal']" active-value="Y" inactive-value="N" />
        </el-form-item>
        <el-form-item label="锁定终端本地设置">
          <el-switch v-model="form.strategy['disable-settings']" active-value="Y" inactive-value="N" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveGroup">保存并下发</el-button>
      </template>
    </el-dialog>

    <!-- 整组批量改密弹窗 -->
    <el-dialog v-model="pwdDialogVisible" :title="`整组下发密码 - ${pwdForm.groupName}`" width="440px">
      <el-form :model="pwdForm" label-width="110px">
        <el-form-item label="改密类型">
          <el-select v-model="pwdForm.type" style="width: 100%">
            <el-option label="设置统一固定密码（推荐）" value="set_permanent_password" />
            <el-option label="设置统一临时密码" value="set_temp_password" />
            <el-option label="全组随机刷新临时密码" value="refresh_temp_password" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="pwdForm.type !== 'refresh_temp_password'" label="新密码">
          <el-input v-model="pwdForm.password" show-password placeholder="固定密码>=6位，临时密码>=4位" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="pwdDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitGroupPassword">下发到全组终端</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/client'

const loading = ref(false)
const groups = ref<any[]>([])
const schools = ref<any[]>([])
const filterSchoolId = ref(0)
const dialogVisible = ref(false)

const editingGroupRowId = ref<number | null>(null)
const editingGroupName = ref('')
const groupInputRef = ref<any>()

const editingSchoolRowId = ref<number | null>(null)
const editingSchoolName = ref('')
const schoolInputRef = ref<any>()

const pwdDialogVisible = ref(false)
const pwdForm = reactive({
  groupId: 0,
  groupName: '',
  type: 'set_permanent_password',
  password: ''
})

const user = computed(() => {
  try {
    return JSON.parse(localStorage.getItem('polaris_admin_user') || '{}')
  } catch {
    return {}
  }
})
const isSuperAdmin = computed(() => (user.value?.school_id || 0) === 0 && (user.value?.is_admin || user.value?.role === 'admin'))

const form = reactive<any>({
  id: 0,
  school_id: 1,
  name: '',
  description: '',
  unattended_pass: '',
  strategy: {
    'approve-mode': 'password',
    'verification-method': 'use-both-passwords',
    'enable-file-transfer': 'Y',
    'enable-clipboard': 'Y',
    'enable-terminal': 'Y',
    'disable-settings': 'N'
  }
})

async function loadSchools() {
  const res: any = await api.get('/schools')
  schools.value = res.items || []
}

async function load() {
  loading.value = true
  try {
    const res: any = await api.get('/groups', { params: { school_id: filterSchoolId.value } })
    groups.value = res.items || []
  } finally {
    loading.value = false
  }
}

function startInlineGroupRename(row: any) {
  editingGroupRowId.value = row.id
  editingGroupName.value = row.name
  nextTick(() => {
    groupInputRef.value?.focus?.()
  })
}

async function saveInlineGroupRename(row: any) {
  if (editingGroupRowId.value !== row.id) return
  const newName = editingGroupName.value.trim()
  editingGroupRowId.value = null
  if (!newName || newName === row.name) return
  await api.put(`/groups/${row.id}`, { name: newName })
  row.name = newName
  ElMessage.success(`分组名称已改为：${newName}`)
}

function startInlineSchoolRename(row: any) {
  editingSchoolRowId.value = row.id
  editingSchoolName.value = row.school_name || '默认学校'
  nextTick(() => {
    schoolInputRef.value?.focus?.()
  })
}

async function saveInlineSchoolRename(row: any) {
  if (editingSchoolRowId.value !== row.id) return
  const newName = editingSchoolName.value.trim()
  editingSchoolRowId.value = null
  if (!newName || newName === row.school_name) return
  await api.put(`/schools/${row.school_id}`, { name: newName })
  ElMessage.success(`学校名称已改为：${newName}`)
  await Promise.all([loadSchools(), load()])
}

function openDialog(row?: any) {
  if (row) {
    form.id = row.id
    form.school_id = row.school_id || 1
    form.name = row.name
    form.description = row.description || ''
    form.unattended_pass = row.unattended_pass || ''
    form.strategy = {
      'approve-mode': 'password',
      'verification-method': 'use-both-passwords',
      'enable-file-transfer': 'Y',
      'enable-clipboard': 'Y',
      'enable-terminal': 'Y',
      'disable-settings': 'N',
      ...(row.strategy || {})
    }
  } else {
    form.id = 0
    form.school_id = filterSchoolId.value || (schools.value[0]?.id || 1)
    form.name = ''
    form.description = ''
    form.unattended_pass = ''
    form.strategy = {
      'approve-mode': 'password',
      'verification-method': 'use-both-passwords',
      'enable-file-transfer': 'Y',
      'enable-clipboard': 'Y',
      'enable-terminal': 'Y',
      'disable-settings': 'N'
    }
  }
  dialogVisible.value = true
}

async function saveGroup() {
  if (!form.name.trim()) {
    ElMessage.warning('请填写分组名称')
    return
  }
  if (form.id) {
    await api.put(`/groups/${form.id}`, form)
  } else {
    await api.post('/groups', form)
  }
  ElMessage.success('策略已保存，将随设备心跳自动下发')
  dialogVisible.value = false
  load()
}

function openGroupPasswordDialog(row: any) {
  pwdForm.groupId = row.id
  pwdForm.groupName = row.name
  pwdForm.type = 'set_permanent_password'
  pwdForm.password = ''
  pwdDialogVisible.value = true
}

async function submitGroupPassword() {
  const res: any = await api.post('/devices/batch-password', {
    type: pwdForm.type,
    password: pwdForm.password,
    group_id: pwdForm.groupId
  })
  ElMessage.success(`已向该分组 ${res.queued || 0} 台终端下发改密指令`)
  pwdDialogVisible.value = false
  load()
}

async function deleteGroup(id: number) {
  await api.delete(`/groups/${id}`)
  ElMessage.success('已删除分组')
  load()
}

onMounted(() => {
  loadSchools()
  load()
})
</script>

<style scoped>
.page-box { padding: 18px 20px; }
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}
.left-tools {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.desc {
  font-size: 13px;
  color: var(--polaris-muted);
}
.dbl-rename-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 6px;
  transition: background-color 0.15s;
}
.dbl-rename-cell:hover {
  background: #f1f5f9;
}
.edit-hint {
  color: #94a3b8;
  font-size: 13px;
}
.bold { font-weight: 600; }
</style>
