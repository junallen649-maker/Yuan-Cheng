<template>
  <div class="overview">
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="polaris-card stat-card">
          <div class="stat-label">接入学校 / 注册终端</div>
          <div class="stat-val">{{ data.total_devices || 0 }}</div>
          <div class="stat-sub">学校 {{ data.total_schools || 1 }} 所 · 已分发ID {{ data.assigned_devices || 0 }} 台</div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="polaris-card stat-card">
          <div class="stat-label">在线被控端 (Agent)</div>
          <div class="stat-val text-green">{{ data.online_agents || 0 }}</div>
          <div class="stat-sub">离线 {{ Math.max(0, (data.total_agents || 0) - (data.online_agents || 0)) }} 台</div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="polaris-card stat-card">
          <div class="stat-label">在线主控端 (Console)</div>
          <div class="stat-val text-blue">{{ data.online_consoles || 0 }}</div>
          <div class="stat-sub">注册主控 {{ data.total_consoles || 0 }} 台</div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="polaris-card stat-card">
          <div class="stat-label">活跃远控会话</div>
          <div class="stat-val text-amber">{{ data.active_sessions || 0 }}</div>
          <div class="stat-sub">今日审计记录 {{ data.today_audits || 0 }} 条</div>
        </div>
      </el-col>
    </el-row>

    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :xs="24" :lg="10">
        <div class="polaris-card panel">
          <div class="panel-header">
            <span>机房 / 分组在线分布</span>
            <el-button link type="primary" @click="$router.push('/groups')">管理分组</el-button>
          </div>
          <div v-if="groupEntries.length === 0" class="empty-tip">暂无设备上报</div>
          <div v-for="[name, st] in groupEntries" :key="name" class="group-row">
            <div class="group-meta">
              <span>{{ name }}</span>
              <span class="muted">{{ st.online }} / {{ st.total }} 在线</span>
            </div>
            <el-progress
              :percentage="st.total ? Math.round((st.online / st.total) * 100) : 0"
              :stroke-width="8"
              color="#1e6fff"
            />
          </div>

          <el-divider />
          <div class="server-info">
            <div><span>平台名称：</span>{{ data.school_name || '校园远控运维平台' }}</div>
            <div><span>信令节点：</span><code>{{ data.server_host }}:{{ data.rendezvous_port }}</code></div>
            <div><span>中继节点：</span><code>{{ data.server_host }}:{{ data.relay_port }}</code></div>
          </div>
        </div>
      </el-col>

      <el-col :xs="24" :lg="14">
        <div class="polaris-card panel">
          <div class="panel-header">
            <span>近期连接与操作审计</span>
            <el-button link type="primary" @click="$router.push('/audits')">查看全部</el-button>
          </div>
          <el-table :data="data.recent_audits || []" size="small" stripe>
            <el-table-column label="时间" width="160">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column prop="audit_type" label="类型" width="80">
              <template #default="{ row }">
                <el-tag size="small" :type="row.audit_type === 'alarm' ? 'danger' : 'info'">
                  {{ row.audit_type }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="device_id" label="目标设备" width="120" />
            <el-table-column prop="peer_id" label="远端ID" width="120" />
            <el-table-column prop="action" label="动作" width="80" />
            <el-table-column prop="client_ip" label="来源IP" min-width="120" />
          </el-table>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import api from '@/api/client'

const data = ref<any>({})
let timer: number | undefined

const groupEntries = computed(() => {
  const stats = data.value?.group_stats || {}
  return Object.entries(stats) as [string, { total: number; online: number }][]
})

function formatTime(v: string) {
  if (!v) return '-'
  return new Date(v).toLocaleString('zh-CN', { hour12: false })
}

async function load() {
  data.value = await api.get('/overview')
}

onMounted(() => {
  load()
  timer = window.setInterval(load, 10000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.stat-card {
  padding: 20px;
  margin-bottom: 12px;
}

.stat-label {
  font-size: 13px;
  color: var(--polaris-muted);
}

.stat-val {
  font-size: 30px;
  font-weight: 700;
  margin: 8px 0 4px;
}

.stat-sub {
  font-size: 12px;
  color: var(--polaris-muted);
}

.text-green { color: #10b981; }
.text-blue { color: #1e6fff; }
.text-amber { color: #f59e0b; }

.panel {
  padding: 18px 20px;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
  font-size: 15px;
  margin-bottom: 16px;
}

.group-row {
  margin-bottom: 14px;
}

.group-meta {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  margin-bottom: 4px;
}

.muted {
  color: var(--polaris-muted);
}

.empty-tip {
  color: var(--polaris-muted);
  font-size: 13px;
  padding: 12px 0;
}

.server-info {
  font-size: 13px;
  color: #475569;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.server-info span {
  color: var(--polaris-muted);
}

.server-info code {
  background: #f1f5f9;
  padding: 2px 6px;
  border-radius: 4px;
}
</style>
