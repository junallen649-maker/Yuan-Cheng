<template>
  <div class="polaris-card page-box">
    <div class="toolbar">
      <div class="filters">
        <el-radio-group v-model="filters.scope" @change="load">
          <el-radio-button value="all">全部连接记录</el-radio-button>
          <el-radio-button value="active">仅实时连接中</el-radio-button>
        </el-radio-group>
        <el-input
          v-model="filters.keyword"
          placeholder="搜索被控/主控 ID、名称、IP 或账号"
          clearable
          style="width: 270px"
          @keyup.enter="load"
          @clear="load"
        />
        <el-button type="primary" @click="load">查询</el-button>
        <el-tag type="success" size="small" effect="plain">每 5 秒自动刷新</el-tag>
      </div>
      <el-button :loading="loading" @click="load">立即刷新</el-button>
    </div>

    <el-table :data="items" v-loading="loading" stripe>
      <el-table-column label="会话状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">
            {{ row.status === 'active' ? '连接中' : '已结束' }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column label="被控端信息（连接代码 / 密码 / IP）" min-width="280">
        <template #default="{ row }">
          <div class="peer-block">
            <div class="peer-top">
              <span class="mono-id" @click="copyText(row.agent_id, '被控连接代码')">
                {{ formatCode(row.agent_id) }}
              </span>
              <span class="bold">{{ row.agent_name || '-' }}</span>
              <el-tag v-if="row.agent_group" size="small" effect="plain">{{ row.agent_group }}</el-tag>
            </div>
            <div class="sub-line">
              <span>密码：</span>
              <code v-if="row.agent_password" class="pwd-chip" @click="copyText(row.agent_password, '被控密码')">
                {{ row.agent_password }}
              </code>
              <span v-else>-</span>
              <span class="sep">|</span>
              <span>内网 IP: {{ row.agent_local_ip || '-' }}</span>
              <span class="sep">|</span>
              <span>公网 IP: {{ row.agent_remote_ip || '-' }}</span>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="主控端信息（ID / 操作人 / IP）" min-width="280">
        <template #default="{ row }">
          <div class="peer-block">
            <div class="peer-top">
              <span class="mono-id" @click="copyText(row.console_id, '主控ID')">
                {{ formatCode(row.console_id) || '未知主控' }}
              </span>
              <span class="bold">{{ row.console_name || '-' }}</span>
              <el-tag v-if="row.console_user" size="small" type="primary" effect="plain">
                {{ row.console_user }}
              </el-tag>
            </div>
            <div class="sub-line">
              <span>内网 IP: {{ row.console_local_ip || '-' }}</span>
              <span class="sep">|</span>
              <span>公网 IP: {{ row.console_remote_ip || '-' }}</span>
              <template v-if="row.console_os">
                <span class="sep">|</span>
                <span>{{ row.console_os }}</span>
              </template>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="类型" width="95">
        <template #default="{ row }">
          <el-tag size="small" effect="plain">{{ connTypeLabel(row.conn_type) }}</el-tag>
        </template>
      </el-table-column>

      <el-table-column label="开始 / 结束时间" width="185">
        <template #default="{ row }">
          <div>起: {{ formatTime(row.started_at) }}</div>
          <div class="sub-line">
            {{ row.ended_at ? `止: ${formatTime(row.ended_at)}` : '进行中' }}
          </div>
        </template>
      </el-table-column>

      <el-table-column label="操作" width="110" fixed="right">
        <template #default="{ row }">
          <el-popconfirm
            v-if="row.status === 'active'"
            title="确认强制断开该被控端当前所有会话？"
            @confirm="forceDisconnect(row.agent_id)"
          >
            <template #reference>
              <el-button link type="danger">强制断开</el-button>
            </template>
          </el-popconfirm>
          <span v-else class="sub-line">-</span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/client'

const loading = ref(false)
const items = ref<any[]>([])
let timer: number | undefined

const filters = reactive({
  scope: 'all',
  keyword: ''
})

async function load() {
  loading.value = true
  try {
    const res: any = await api.get('/sessions', { params: filters })
    items.value = res.items || []
  } finally {
    loading.value = false
  }
}

async function forceDisconnect(agentId: string) {
  await api.post(`/devices/${agentId}/disconnect`)
  ElMessage.success('已下发断连指令')
  load()
}

function connTypeLabel(t: number) {
  switch (t) {
    case 1: return '文件传输'
    case 2: return '端口映射'
    case 3: return '摄像头'
    case 4: return '远程终端'
    default: return '桌面控制'
  }
}

function formatCode(id: string) {
  if (!id) return ''
  const clean = id.replace(/\s+/g, '')
  if (clean.length === 9) {
    return `${clean.slice(0, 3)} ${clean.slice(3, 6)} ${clean.slice(6, 9)}`
  }
  return id
}

function copyText(val: string, label: string) {
  if (!val) return
  navigator.clipboard?.writeText(val)
  ElMessage.success(`已复制${label}: ${val}`)
}

function formatTime(v: string) {
  return v ? new Date(v).toLocaleString('zh-CN', { hour12: false }) : '-'
}

onMounted(() => {
  load()
  timer = window.setInterval(load, 5000)
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
.filters {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
.peer-block {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.peer-top {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.mono-id {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-weight: 700;
  color: #1e6fff;
  cursor: pointer;
}
.bold { font-weight: 600; }
.sub-line {
  font-size: 12px;
  color: var(--polaris-muted);
}
.sep { margin: 0 6px; color: #cbd5e1; }
.pwd-chip {
  background: #f1f5f9;
  padding: 1px 6px;
  border-radius: 4px;
  color: #0f172a;
  font-weight: 600;
  cursor: pointer;
}
</style>
