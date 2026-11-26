<template>
  <div class="polaris-card page-box">
    <div class="toolbar">
      <div class="filters">
        <el-radio-group v-model="filters.role" @change="loadDevices">
          <el-radio-button value="all">全部终端</el-radio-button>
          <el-radio-button value="agent">被控端 (Agent)</el-radio-button>
          <el-radio-button value="console">主控端 (Console)</el-radio-button>
        </el-radio-group>

        <el-select v-if="isSuperAdmin" v-model="filters.school_id" style="width: 145px" @change="onSchoolFilterChange">
          <el-option label="全部学校" :value="0" />
          <el-option v-for="s in schools" :key="s.id" :label="s.name" :value="s.id" />
        </el-select>

        <el-select v-model="filters.group_id" style="width: 145px" @change="loadDevices">
          <el-option label="全部分组" :value="0" />
          <el-option v-for="g in filteredGroups" :key="g.id" :label="g.name" :value="g.id" />
        </el-select>

        <el-select v-model="filters.status" style="width: 105px" @change="loadDevices">
          <el-option label="全部状态" value="all" />
          <el-option label="仅在线" value="online" />
          <el-option label="仅离线" value="offline" />
        </el-select>

        <el-input
          v-model="filters.keyword"
          placeholder="搜索连接代码 / 主机名 / IP / 账号"
          clearable
          style="width: 230px"
          @keyup.enter="loadDevices"
          @clear="loadDevices"
        />
        <el-button type="primary" @click="loadDevices">查询</el-button>
      </div>

      <div class="actions">
        <template v-if="selectedIds.length > 0">
          <el-button type="primary" plain @click="openBatchAssignID">
            批量分发 ID ({{ selectedIds.length }})
          </el-button>
          <el-button type="warning" plain @click="openBatchPassword">
            批量改密 ({{ selectedIds.length }})
          </el-button>
          <el-button @click="openBatchGroup">
            批量调拨分组 ({{ selectedIds.length }})
          </el-button>
        </template>
        <el-button @click="openCommandsDrawer">指令回执</el-button>
        <el-button :loading="loading" @click="loadDevices">刷新</el-button>
      </div>
    </div>

    <el-table
      :data="devices"
      v-loading="loading"
      stripe
      @selection-change="onSelectionChange"
    >
      <el-table-column type="selection" width="45" />
      <el-table-column label="状态" width="75">
        <template #default="{ row }">
          <el-tag :type="row.online ? 'success' : 'info'" size="small" effect="light">
            {{ row.online ? '在线' : '离线' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="角色" width="100">
        <template #default="{ row }">
          <el-tag :type="row.client_role === 'console' ? 'primary' : 'warning'" size="small">
            {{ row.client_role === 'console' ? 'Console主控' : 'Agent被控' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="id" label="连接代码 (ID)" width="155">
        <template #default="{ row }">
          <div class="code-row">
            <span class="mono-id" @click="copyText(row.id, '连接代码')">{{ formatCode(row.id) }}</span>
            <el-tag v-if="row.id_assigned" size="small" type="success" effect="plain">已分发</el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="连接密码" width="180">
        <template #default="{ row }">
          <div v-if="row.client_role !== 'console'" class="pwd-cell">
            <template v-if="effectivePwd(row)">
              <span class="mono-pwd" @click="copyText(effectivePwd(row), '连接密码')">
                {{ showPwdMap[row.id] ? effectivePwd(row) : '••••••' }}
              </span>
              <el-button link size="small" @click="showPwdMap[row.id] = !showPwdMap[row.id]">
                {{ showPwdMap[row.id] ? '隐藏' : '查看' }}
              </el-button>
              <el-button link type="primary" size="small" @click="copyText(effectivePwd(row), '连接密码')">
                复制
              </el-button>
            </template>
            <span v-else class="sub-text">待上报</span>
            <el-tag v-if="row.has_permanent_password" size="small" type="info" effect="plain" title="已启用固定密码">
              固定
            </el-tag>
          </div>
          <span v-else class="sub-text">-</span>
        </template>
      </el-table-column>
      <el-table-column label="所属学校 / 分组" min-width="155">
        <template #default="{ row }">
          <div class="bold">{{ row.school_name || '默认学校' }}</div>
          <div class="sub-text">{{ row.group_name || '未分组' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="名称 / 主机名" min-width="150">
        <template #default="{ row }">
          <div class="bold">{{ row.alias || row.hostname || '-' }}</div>
          <div v-if="row.alias && row.hostname" class="sub-text">{{ row.hostname }}</div>
        </template>
      </el-table-column>
      <el-table-column label="内网 IP / 公网 IP" min-width="170">
        <template #default="{ row }">
          <div>内网: {{ row.local_ips || '-' }}</div>
          <div class="sub-text">公网: {{ row.remote_ip || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="系统 / 用户名" min-width="145">
        <template #default="{ row }">
          <div>{{ row.bound_user || row.username || '-' }}</div>
          <div class="sub-text">{{ row.os || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="最近指令" min-width="145">
        <template #default="{ row }">
          <span class="sub-text">{{ row.latest_cmd_info || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="265" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="row.client_role !== 'console'"
            link
            type="primary"
            @click="launchConnect(row.id)"
          >
            连接
          </el-button>
          <el-button
            v-if="row.client_role !== 'console'"
            link
            type="primary"
            @click="openChangeIdDialog(row)"
          >
            改连接代码
          </el-button>
          <el-button
            v-if="row.client_role !== 'console'"
            link
            type="warning"
            @click="openChangePwdDialog(row)"
          >
            改密码
          </el-button>
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button
            v-if="row.active_conns_count > 0"
            link
            type="danger"
            @click="disconnectDevice(row.id)"
          >
            断连
          </el-button>
          <el-popconfirm title="确认删除该设备记录？" @confirm="deleteDevice(row.id)">
            <template #reference>
              <el-button link type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <!-- 编辑终端弹窗 -->
    <el-dialog v-model="editVisible" title="编辑终端信息" width="460px">
      <el-form :model="editForm" label-width="95px">
        <el-form-item label="连接代码">
          <el-input v-model="editForm.id" disabled />
        </el-form-item>
        <el-form-item label="终端别名">
          <el-input v-model="editForm.alias" placeholder="例如：101机房-01号机" />
        </el-form-item>
        <el-form-item v-if="isSuperAdmin" label="所属学校">
          <el-select v-model="editForm.school_id" style="width: 100%" @change="onEditSchoolChange">
            <el-option v-for="s in schools" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="所属机房分组">
          <el-select v-model="editForm.group_id" style="width: 100%">
            <el-option label="未分组" :value="0" />
            <el-option v-for="g in editSchoolGroups" :key="g.id" :label="g.name" :value="g.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="终端角色">
          <el-select v-model="editForm.client_role" style="width: 100%">
            <el-option label="被控端 (Agent)" value="agent" />
            <el-option label="主控端 (Console)" value="console" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="editForm.note" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 更改连接代码弹窗 -->
    <el-dialog v-model="changeIdVisible" title="远程更改连接代码 (Device ID)" width="460px">
      <el-form :model="changeIdForm" label-width="115px">
        <el-form-item label="当前连接代码">
          <el-input v-model="changeIdForm.deviceId" disabled />
        </el-form-item>
        <el-form-item label="分配方式">
          <el-radio-group v-model="changeIdForm.autoGen">
            <el-radio :value="true">按学校前缀自动发号</el-radio>
            <el-radio :value="false">手动指定连接代码</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="!changeIdForm.autoGen" label="新连接代码">
          <el-input v-model="changeIdForm.newId" placeholder="字母开头6~16位，如 yz-101-01" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="changeIdVisible = false">取消</el-button>
        <el-button type="primary" @click="submitChangeId">下发更改指令</el-button>
      </template>
    </el-dialog>

    <!-- 远程更改密码弹窗 -->
    <el-dialog v-model="changePwdVisible" :title="`远程改密 - ${changePwdForm.deviceId}`" width="460px">
      <el-form :model="changePwdForm" label-width="110px">
        <el-form-item label="操作类型">
          <el-select v-model="changePwdForm.type" style="width: 100%">
            <el-option label="修改临时密码（指定值）" value="set_temp_password" />
            <el-option label="随机刷新临时密码" value="refresh_temp_password" />
            <el-option label="修改无人值守固定密码" value="set_permanent_password" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="changePwdForm.type !== 'refresh_temp_password'" label="新密码">
          <el-input v-model="changePwdForm.password" show-password placeholder="临时密码>=4位；固定密码>=6位（留空清除）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="changePwdVisible = false">取消</el-button>
        <el-button type="primary" @click="submitChangePwd">下发改密指令</el-button>
      </template>
    </el-dialog>

    <!-- 批量分发 ID 弹窗 -->
    <el-dialog v-model="batchAssignVisible" title="批量分发学校连接代码" width="420px">
      <el-form label-width="95px">
        <el-form-item v-if="isSuperAdmin" label="归属学校">
          <el-select v-model="batchAssignSchoolId" style="width: 100%">
            <el-option label="保持各设备当前学校" :value="0" />
            <el-option v-for="s in schools" :key="s.id" :label="`${s.name} (${s.code})`" :value="s.id" />
          </el-select>
        </el-form-item>
        <div class="sub-text" style="padding-left: 16px">
          将为选中的 {{ selectedIds.length }} 台终端按学校前缀依次分配唯一连接代码并自动换绑。
        </div>
      </el-form>
      <template #footer>
        <el-button @click="batchAssignVisible = false">取消</el-button>
        <el-button type="primary" @click="submitBatchAssignID">立即分发</el-button>
      </template>
    </el-dialog>

    <!-- 批量改密弹窗 -->
    <el-dialog v-model="batchPwdVisible" title="批量下发密码" width="440px">
      <el-form :model="batchPwdForm" label-width="110px">
        <el-form-item label="改密类型">
          <el-select v-model="batchPwdForm.type" style="width: 100%">
            <el-option label="设置统一固定密码" value="set_permanent_password" />
            <el-option label="设置统一临时密码" value="set_temp_password" />
            <el-option label="随机刷新临时密码" value="refresh_temp_password" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="batchPwdForm.type !== 'refresh_temp_password'" label="新密码">
          <el-input v-model="batchPwdForm.password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchPwdVisible = false">取消</el-button>
        <el-button type="primary" @click="submitBatchPassword">确认下发</el-button>
      </template>
    </el-dialog>

    <!-- 批量调拨分组弹窗 -->
    <el-dialog v-model="batchVisible" title="批量分配机房分组" width="380px">
      <el-form label-width="85px">
        <el-form-item label="目标分组">
          <el-select v-model="batchGroupId" style="width: 100%">
            <el-option v-for="g in groups" :key="g.id" :label="`${g.school_name || ''} · ${g.name}`" :value="g.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchVisible = false">取消</el-button>
        <el-button type="primary" @click="saveBatchGroup">确认分配</el-button>
      </template>
    </el-dialog>

    <!-- 指令回执抽屉 -->
    <el-drawer v-model="cmdDrawerVisible" title="远程改密 / 改连接代码指令回执" size="56%">
      <el-table :data="commands" size="small" stripe>
        <el-table-column prop="id" label="#" width="60" />
        <el-table-column prop="device_id" label="设备ID" width="130" />
        <el-table-column prop="command_type" label="指令类型" width="145" />
        <el-table-column label="状态" width="95">
          <template #default="{ row }">
            <el-tag
              size="small"
              :type="row.status === 'succeeded' ? 'success' : (row.status === 'failed' ? 'danger' : 'warning')"
            >
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="result_msg" label="回执结果" min-width="180" />
        <el-table-column prop="created_by" label="操作人" width="95" />
        <el-table-column label="创建时间" width="155">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/client'

const loading = ref(false)
const devices = ref<any[]>([])
const schools = ref<any[]>([])
const groups = ref<any[]>([])
const selectedIds = ref<string[]>([])
const showPwdMap = reactive<Record<string, boolean>>({})
let timer: number | undefined

const user = computed(() => {
  try {
    return JSON.parse(localStorage.getItem('polaris_admin_user') || '{}')
  } catch {
    return {}
  }
})
const isSuperAdmin = computed(() => (user.value?.school_id || 0) === 0 && (user.value?.is_admin || user.value?.role === 'admin'))

const filters = reactive({
  role: 'all',
  status: 'all',
  school_id: 0,
  group_id: 0,
  keyword: ''
})

const filteredGroups = computed(() => {
  if (!filters.school_id) return groups.value
  return groups.value.filter((g) => g.school_id === filters.school_id)
})

const editVisible = ref(false)
const editForm = reactive({
  id: '',
  alias: '',
  school_id: 1,
  client_role: 'agent',
  group_id: 1,
  note: ''
})
const editSchoolGroups = computed(() => groups.value.filter((g) => g.school_id === editForm.school_id))

const changeIdVisible = ref(false)
const changeIdForm = reactive({
  deviceId: '',
  autoGen: true,
  newId: ''
})

const changePwdVisible = ref(false)
const changePwdForm = reactive({
  deviceId: '',
  type: 'set_temp_password',
  password: ''
})

const batchAssignVisible = ref(false)
const batchAssignSchoolId = ref(0)

const batchPwdVisible = ref(false)
const batchPwdForm = reactive({
  type: 'set_permanent_password',
  password: ''
})

const batchVisible = ref(false)
const batchGroupId = ref(1)

const cmdDrawerVisible = ref(false)
const commands = ref<any[]>([])

function formatCode(id: string) {
  const clean = (id || '').replace(/\s+/g, '')
  if (/^\d{9}$/.test(clean)) {
    return `${clean.slice(0, 3)} ${clean.slice(3, 6)} ${clean.slice(6, 9)}`
  }
  return id
}

function effectivePwd(row: any) {
  return row.temp_password || row.managed_perm_pass || row.unattended_pass || ''
}

function onSchoolFilterChange() {
  filters.group_id = 0
  loadDevices()
}

function onEditSchoolChange() {
  const first = editSchoolGroups.value[0]
  editForm.group_id = first ? first.id : 0
}

async function loadMeta() {
  const [sRes, gRes]: any = await Promise.all([api.get('/schools'), api.get('/groups')])
  schools.value = sRes.items || []
  groups.value = gRes.items || []
}

async function loadDevices() {
  loading.value = true
  try {
    const res: any = await api.get('/devices', { params: filters })
    devices.value = res.items || []
  } finally {
    loading.value = false
  }
}

function onSelectionChange(rows: any[]) {
  selectedIds.value = rows.map((r) => r.id)
}

function openEdit(row: any) {
  editForm.id = row.id
  editForm.alias = row.alias || ''
  editForm.school_id = row.school_id || 1
  editForm.client_role = row.client_role || 'agent'
  editForm.group_id = row.group_id || 0
  editForm.note = row.note || ''
  editVisible.value = true
}

async function saveEdit() {
  await api.put(`/devices/${editForm.id}`, editForm)
  ElMessage.success('已更新设备信息')
  editVisible.value = false
  loadDevices()
}

function openChangeIdDialog(row: any) {
  changeIdForm.deviceId = row.id
  changeIdForm.autoGen = true
  changeIdForm.newId = ''
  changeIdVisible.value = true
}

async function submitChangeId() {
  await api.post(`/devices/${changeIdForm.deviceId}/command`, {
    type: 'change_id',
    auto_gen: changeIdForm.autoGen,
    new_id: changeIdForm.newId
  })
  ElMessage.success('已下发更改连接代码指令，设备将在下一次心跳执行换绑')
  changeIdVisible.value = false
  loadDevices()
}

function openChangePwdDialog(row: any) {
  changePwdForm.deviceId = row.id
  changePwdForm.type = 'set_temp_password'
  changePwdForm.password = ''
  changePwdVisible.value = true
}

async function submitChangePwd() {
  await api.post(`/devices/${changePwdForm.deviceId}/command`, {
    type: changePwdForm.type,
    password: changePwdForm.password
  })
  ElMessage.success('已下发改密指令，设备将在下一次心跳同步生效')
  changePwdVisible.value = false
  loadDevices()
}

function openBatchAssignID() {
  batchAssignSchoolId.value = filters.school_id || 0
  batchAssignVisible.value = true
}

async function submitBatchAssignID() {
  const res: any = await api.post('/devices/assign-ids', {
    school_id: batchAssignSchoolId.value,
    device_ids: selectedIds.value
  })
  ElMessage.success(`已为 ${res.queued || 0} 台终端队列分发学校连接代码`)
  batchAssignVisible.value = false
  loadDevices()
}

function openBatchPassword() {
  batchPwdForm.type = 'set_permanent_password'
  batchPwdForm.password = ''
  batchPwdVisible.value = true
}

async function submitBatchPassword() {
  const res: any = await api.post('/devices/batch-password', {
    type: batchPwdForm.type,
    password: batchPwdForm.password,
    device_ids: selectedIds.value
  })
  ElMessage.success(`已向 ${res.queued || 0} 台终端下发改密指令`)
  batchPwdVisible.value = false
  loadDevices()
}

function openBatchGroup() {
  batchGroupId.value = groups.value[0]?.id || 1
  batchVisible.value = true
}

async function saveBatchGroup() {
  await api.post('/devices/batch-group', {
    device_ids: selectedIds.value,
    group_id: batchGroupId.value
  })
  ElMessage.success('批量分组完成')
  batchVisible.value = false
  loadDevices()
}

async function openCommandsDrawer() {
  cmdDrawerVisible.value = true
  const res: any = await api.get('/commands')
  commands.value = res.items || []
}

async function disconnectDevice(id: string) {
  await api.post(`/devices/${id}/disconnect`)
  ElMessage.success('已下发断开连接指令')
  loadDevices()
}

async function deleteDevice(id: string) {
  await api.delete(`/devices/${id}`)
  ElMessage.success('已删除设备')
  loadDevices()
}

function copyText(val: string, label: string) {
  navigator.clipboard?.writeText(val)
  ElMessage.success(`已复制${label}: ${val}`)
}

function launchConnect(id: string) {
  window.location.href = `polaris://connect/${id}`
  ElMessage.info(`正在唤起本地 Polaris 控制端连接 ${id}`)
}

function formatTime(v: string) {
  if (!v) return '-'
  return new Date(v).toLocaleString('zh-CN', { hour12: false })
}

onMounted(() => {
  loadMeta()
  loadDevices()
  timer = window.setInterval(loadDevices, 12000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
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
.filters,
.actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
.code-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.mono-id {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-weight: 700;
  color: #1e6fff;
  cursor: pointer;
}
.pwd-cell {
  display: flex;
  align-items: center;
  gap: 5px;
  flex-wrap: wrap;
}
.mono-pwd {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-weight: 600;
  color: #0f172a;
  cursor: pointer;
}
.bold { font-weight: 600; }
.sub-text {
  font-size: 12px;
  color: var(--polaris-muted);
}
</style>
