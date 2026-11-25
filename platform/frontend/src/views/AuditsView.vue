<template>
  <div class="polaris-card page-box">
    <div class="toolbar">
      <div class="filters">
        <el-select v-model="filters.audit_type" style="width: 130px" @change="load">
          <el-option label="全部类型" value="all" />
          <el-option label="远控连接 (conn)" value="conn" />
          <el-option label="文件传输 (file)" value="file" />
          <el-option label="安全告警 (alarm)" value="alarm" />
        </el-select>
        <el-input
          v-model="filters.keyword"
          placeholder="搜索设备 ID / 远端 ID / 操作人 / IP"
          clearable
          style="width: 260px"
          @keyup.enter="load"
          @clear="load"
        />
        <el-button type="primary" @click="load">查询</el-button>
      </div>
      <el-button :loading="loading" @click="load">刷新</el-button>
    </div>

    <el-table :data="items" v-loading="loading" stripe>
      <el-table-column label="时间" width="165">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column prop="audit_type" label="类型" width="90">
        <template #default="{ row }">
          <el-tag size="small" :type="row.audit_type === 'alarm' ? 'danger' : (row.audit_type === 'file' ? 'warning' : 'primary')">
            {{ row.audit_type }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="device_id" label="目标设备 ID" width="130" />
      <el-table-column prop="peer_id" label="远端 ID" width="130" />
      <el-table-column prop="peer_name" label="远端主机名" width="140" />
      <el-table-column prop="operator_user" label="操作账号" width="120" />
      <el-table-column prop="action" label="动作" width="85" />
      <el-table-column prop="client_ip" label="来源 IP" width="140" />
      <el-table-column prop="note" label="备注" min-width="140" />
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="filters.page"
        :page-size="filters.page_size"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="load"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import api from '@/api/client'

const loading = ref(false)
const items = ref<any[]>([])
const total = ref(0)

const filters = reactive({
  audit_type: 'all',
  keyword: '',
  page: 1,
  page_size: 30
})

async function load() {
  loading.value = true
  try {
    const res: any = await api.get('/audits', { params: filters })
    items.value = res.items || []
    total.value = res.total || 0
  } finally {
    loading.value = false
  }
}

function formatTime(v: string) {
  return v ? new Date(v).toLocaleString('zh-CN', { hour12: false }) : '-'
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
.filters {
  display: flex;
  gap: 10px;
}
.pager {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
</style>
