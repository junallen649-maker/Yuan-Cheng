<template>
  <div class="polaris-card page-box">
    <div class="toolbar">
      <div class="desc">
        管理多所学校的 ID 编码规则、入校注册码及分级权限（<b>提示：双击学校名称可直接改名</b>）
      </div>
      <el-button v-if="isSuperAdmin" type="primary" @click="openDialog()">新增学校</el-button>
    </div>

    <el-table :data="schools" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="65" />
      <el-table-column label="学校名称（双击可改名）" min-width="190">
        <template #default="{ row }">
          <div v-if="editingSchoolId === row.id" class="inline-edit-wrap">
            <el-input
              ref="inlineInputRef"
              v-model="editingSchoolName"
              size="small"
              @keyup.enter="saveInlineRename(row)"
              @keyup.esc="cancelInlineRename"
              @blur="saveInlineRename(row)"
            />
          </div>
          <div v-else class="dbl-rename-cell" title="双击修改学校名称" @dblclick="startInlineRename(row)">
            <span class="bold">{{ row.name }}</span>
            <el-icon class="edit-hint"><EditPen /></el-icon>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="ID 前缀编码" width="120">
        <template #default="{ row }">
          <el-tag size="small" type="primary" effect="plain">{{ row.code }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="自动发号规则" width="175">
        <template #default="{ row }">
          <div>示例: <code>{{ row.code }}-{{ String((row.id_seq || 0) + 1).padStart(4, '0') }}</code></div>
          <div class="sub-text">
            入网自动领号:
            <el-tag size="small" :type="row.auto_assign_id ? 'success' : 'info'">
              {{ row.auto_assign_id ? '开启' : '关闭' }}
            </el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="入校分发码" width="150">
        <template #default="{ row }">
          <code class="copy-chip" @click="copyText(row.enroll_token, '入校分发码')">{{ row.enroll_token }}</code>
        </template>
      </el-table-column>
      <el-table-column label="终端 / 分组" width="140">
        <template #default="{ row }">
          <div>{{ row.online_count }} / {{ row.device_count }} 在线</div>
          <div class="sub-text">{{ row.group_count }} 个机房分组</div>
        </template>
      </el-table-column>
      <el-table-column prop="contact" label="负责人" width="120" />
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="copyDeployCmd(row)">复制装机命令</el-button>
          <el-button link type="primary" @click="openDialog(row)">配置</el-button>
          <el-popconfirm
            v-if="isSuperAdmin && row.id > 1"
            title="删除后该校设备将移回默认学校，确认删除？"
            @confirm="deleteSchool(row.id)"
          >
            <template #reference>
              <el-button link type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑学校配置' : '新增学校'" width="520px">
      <el-form :model="form" label-width="130px">
        <el-form-item label="学校名称">
          <el-input v-model="form.name" placeholder="例如：第一实验中学" />
        </el-form-item>
        <el-form-item label="学校编码 (前缀)">
          <el-input v-model="form.code" placeholder="字母开头 2~8 位，如 yz、syxx" />
        </el-form-item>
        <el-form-item label="入校分发码">
          <el-input v-model="form.enroll_token" placeholder="留空自动生成 code-enroll" />
        </el-form-item>
        <el-form-item label="首次入网自动发号">
          <el-switch v-model="form.auto_assign_id" active-text="自动分配学校ID" inactive-text="手动分发" />
        </el-form-item>
        <el-form-item label="负责人 / 联系方式">
          <el-input v-model="form.contact" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.note" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveSchool">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/client'

const loading = ref(false)
const schools = ref<any[]>([])
const dialogVisible = ref(false)

const editingSchoolId = ref<number | null>(null)
const editingSchoolName = ref('')
const inlineInputRef = ref<any>()

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
  name: '',
  code: '',
  enroll_token: '',
  auto_assign_id: true,
  contact: '',
  note: ''
})

async function load() {
  loading.value = true
  try {
    const res: any = await api.get('/schools')
    schools.value = res.items || []
  } finally {
    loading.value = false
  }
}

function startInlineRename(row: any) {
  editingSchoolId.value = row.id
  editingSchoolName.value = row.name
  nextTick(() => {
    inlineInputRef.value?.focus?.()
  })
}

function cancelInlineRename() {
  editingSchoolId.value = null
  editingSchoolName.value = ''
}

async function saveInlineRename(row: any) {
  const newName = editingSchoolName.value.trim()
  if (editingSchoolId.value !== row.id) return
  editingSchoolId.value = null
  if (!newName || newName === row.name) return
  await api.put(`/schools/${row.id}`, { name: newName })
  row.name = newName
  ElMessage.success(`学校名称已更新为：${newName}`)
}

function openDialog(row?: any) {
  if (row) {
    form.id = row.id
    form.name = row.name
    form.code = row.code
    form.enroll_token = row.enroll_token
    form.auto_assign_id = Boolean(row.auto_assign_id)
    form.contact = row.contact || ''
    form.note = row.note || ''
  } else {
    form.id = 0
    form.name = ''
    form.code = ''
    form.enroll_token = ''
    form.auto_assign_id = true
    form.contact = ''
    form.note = ''
  }
  dialogVisible.value = true
}

async function saveSchool() {
  if (!form.name.trim() || !form.code.trim()) {
    ElMessage.warning('请填写学校名称与编码')
    return
  }
  if (form.id) {
    await api.put(`/schools/${form.id}`, form)
  } else {
    await api.post('/schools', form)
  }
  ElMessage.success('学校配置已保存')
  dialogVisible.value = false
  load()
}

async function deleteSchool(id: number) {
  await api.delete(`/schools/${id}`)
  ElMessage.success('已删除学校')
  load()
}

function copyText(val: string, label: string) {
  navigator.clipboard?.writeText(val)
  ElMessage.success(`已复制${label}: ${val}`)
}

function copyDeployCmd(row: any) {
  const cmd = `Polaris-Agent.exe --silent-install --agent --school=${row.code}`
  navigator.clipboard?.writeText(cmd)
  ElMessage.success(`已复制「${row.name}」安装命令: ${cmd}`)
}

onMounted(load)
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
.sub-text {
  font-size: 12px;
  color: var(--polaris-muted);
  margin-top: 2px;
}
.copy-chip {
  background: #f1f5f9;
  padding: 2px 6px;
  border-radius: 4px;
  cursor: pointer;
}
</style>
