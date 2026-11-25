<template>
  <div class="polaris-card page-box">
    <div class="toolbar">
      <div class="filters">
        <el-radio-group v-model="filters.role" @change="loadDevices">
          <el-radio-button value="all">全部终端</el-radio-button>
          <el-radio-button value="agent">被控端 (Agent)</el-radio-button>
          <el-radio-button value="console">主控端 (Console)</el-radio-button>
        </el-radio-group>

        <el-select v-model="filters.status" style="width: 110px" @change="loadDevices">
          <el-option label="全部状态" value="all" />
          <el-option label="仅在线" value="online" />
          <el-option label="仅离线" value="offline" />
        </el-select>

        <el-select v-model="filters.group_id" style="width: 150px" @change="loadDevices">
          <el-option label="全部分组" :value="0" />
          <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
        </el-select>

        <el-input
          v-model="filters.keyword"
          placeholder="搜索连接代码 / 主机名 / IP / 账号"
          clearable
          style="width: 250px"
          @keyup.enter="loadDevices"
          @clear="loadDevices"
        />
        <el-button type="primary" @click="loadDevices">查询</el-button>
      </div>

      <div class="actions">
        <el-button v-if="selectedIds.length > 0" @click="openBatchGroup">
          批量分组 ({{ selectedIds.length }})
        </el-button>
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
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.online ? 'success' : 'info'" size="small" effect="light">
            {{ row.online ? '在线' : '离线' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="角色" width="105">
        <template #default="{ row }">
          <el-tag :type="row.client_role === 'console' ? 'primary' : 'warning'" size="small">
            {{ row.client_role === 'console' ? 'Console主控' : 'Agent被控' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="id" label="连接代码 (ID)" width="145">
        <template #default="{ row }">
          <div class="code-row">
            <span class="mono-id" @click="copyText(row.id, '连接代码')">{{ formatCode(row.id) }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="连接密码" width="175">
        <template #default="{ row }">
          <div v-if="row.client_role !== 'console'" class="pwd-cell">
            <template v-if="effectivePwd(row)">
              <span class="mono-pwd" @click="copyText(effectivePwd(row), '连接密码')">
                {{ showPwdMap[row.id] ? effectivePwd(row) : '••••••' }}
              </span>
              <el-button
                link
                size="small"
                @click="showPwdMap[row.id] = !showPwdMap[row.id]"
              >
                {{ showPwdMap[row.id] ? '隐藏' : '查看' }}
              </el-button>
              <el-button
                link
                type="primary"
                size="small"
                @click="copyText(effectivePwd(row), '连接密码')"
              >
                复制
              </el-button>
            </template>
            <span v-else class="sub-text">
              {{ row.has_permanent_password ? '已设固定密码' : '待上报' }}
            </span>
          </div>
          <span v-else class="sub-text">-</span>
        </template>
      </el-table-column>
      <el-table-column label="名称 / 主机名" min-width="155">
        <template #default="{ row }">
          <div class="bold">{{ row.alias || row.hostname || '-' }}</div>
          <div v-if="row.alias && row.hostname" class="sub-text">{{ row.hostname }}</div>
        </template>
      </el-table-column>
      <el-table-column prop="group_name" label="所属机房/分组" width="130" />
      <el-table-column label="登录用户 / 绑定教师" min-width="130">
        <template #default="{ row }">
          <div>{{ row.bound_user || row.username || '-' }}</div>
          <div v-if="row.bound_user && row.username" class="sub-text">OS: {{ row.username }}</div>
        </template>
      </el-table-column>
      <el-table-column label="内网 IP / 公网 IP" min-width="175">
        <template #default="{ row }">
          <div>内网: {{ row.local_ips || '-' }}</div>
          <div class="sub-text">公网: {{ row.remote_ip || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="系统 / 版本" min-width="155">
        <template #default="{ row }">
          <div>{{ row.os || '-' }}</div>
          <div class="sub-text">v{{ row.version || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="活动会话" width="85" align="center">
        <template #default="{ row }">
          <el-tag v-if="row.active_conns_count > 0" type="danger" size="small">
            {{ row.active_conns_count }} 路
          </el-tag>
          <span v-else class="sub-text">0</span>
        </template>
      </el-table-column>
      <el-table-column label="最后心跳" width="160">
        <template #default="{ row }">{{ formatTime(row.last_seen_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="210" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="row.client_role !== 'console'"
            link
            type="primary"
            @click="launchConnect(row.id)"
          >
            远控连接
          </el-button>
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button
            v-if="row.active_conns_count > 0"
            link
            type="warning"
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

    <!-- 编辑弹窗 -->
    <el-dialog v-model="editVisible" title="编辑终端信息" width="460px">
      <el-form :model="editForm" label-width="95px">
        <el-form-item label="设备 ID">
          <el-input v-model="editForm.id" disabled />
        </el-form-item>
        <el-form-item label="终端别名">
          <el-input v-model="editForm.alias" placeholder="例如：101机房-01号机" />
        </el-form-item>
        <el-form-item label="终端角色">
          <el-select v-model="editForm.client_role" style="width: 100%">
            <el-option label="被控端 (Agent)" value="agent" />
            <el-option label="主控端 (Console)" value="console" />
          </el-select>
        </el-form-item>
        <el-form-item label="所属分组">
          <el-select v-model="editForm.group_id" style="width: 100%">
            <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
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

    <!-- 批量分组弹窗 -->
    <el-dialog v-model="batchVisible" title="批量分配机房分组" width="380px">
      <el-form label-width="85px">
        <el-form-item label="目标分组">
          <el-select v-model="batchGroupId" style="width: 100%">
            <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchVisible = false">取消</el-button>
        <el-button type="primary" @click="saveBatchGroup">确认分配</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/client'

const loading = ref(false)
const devices = ref<any[]>([])
const groups = ref<any[]>([])
const selectedIds = ref<string[]>([])
const showPwdMap = reactive<Record<string, boolean>>({})
let timer: number | undefined

const filters = reactive({
  role: 'all',
  status: 'all',
  group_id: 0,
  keyword: ''
})

const editVisible = ref(false)
const editForm = reactive({
  id: '',
  alias: '',
  client_role: 'agent',
  group_id: 1,
  note: ''
})

const batchVisible = ref(false)
const batchGroupId = ref(1)

function formatCode(id: string) {
  const clean = (id || '').replace(/\s+/g, '')
  if (clean.length === 9) {
    return `${clean.slice(0, 3)} ${clean.slice(3, 6)} ${clean.slice(6, 9)}`
  }
  return id
}

function effectivePwd(row: any) {
  return row.temp_password || row.unattended_pass || ''
}

async function loadGroups() {
  const res: any = await api.get('/groups')
  groups.value = res.items || []
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
  editForm.client_role = row.client_role || 'agent'
  editForm.group_id = row.group_id || 1
  editForm.note = row.note || ''
  editVisible.value = true
}

async function saveEdit() {
  await api.put(`/devices/${editForm.id}`, editForm)
  ElMessage.success('已更新设备信息')
  editVisible.value = false
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

async function disconnectDevice(id: string) {
  await api.post(`/devices/${id}/disconnect`)
  ElMessage.success('已下发断开连接指令，将在下一次心跳生效')
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
  loadGroups()
  loadDevices()
  timer = window.setInterval(loadDevices, 12000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.page-box {
  padding: 18px 20px;
}

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

.mono-id {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-weight: 700;
  color: #1e6fff;
  cursor: pointer;
}

.pwd-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.mono-pwd {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-weight: 600;
  color: #0f172a;
  cursor: pointer;
}

.bold {
  font-weight: 600;
}

.sub-text {
  font-size: 12px;
  color: var(--polaris-muted);
}
</style>
