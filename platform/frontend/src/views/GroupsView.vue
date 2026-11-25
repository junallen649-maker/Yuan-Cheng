<template>
  <div class="polaris-card page-box">
    <div class="toolbar">
      <div class="desc">通过机房/分组统一下发无人值守与安全策略，客户端将在 15 秒内自动同步</div>
      <el-button type="primary" @click="openDialog()">新建机房分组</el-button>
    </div>

    <el-table :data="groups" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="分组名称" width="180" />
      <el-table-column prop="description" label="说明" min-width="200" />
      <el-table-column prop="device_count" label="设备数" width="90" align="center" />
      <el-table-column label="验证模式" width="160">
        <template #default="{ row }">
          <el-tag size="small" effect="plain">
            {{ row.strategy?.['approve-mode'] || '默认' }} / {{ row.strategy?.['verification-method'] || '默认' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="权限策略" min-width="220">
        <template #default="{ row }">
          <el-tag v-if="row.strategy?.['enable-file-transfer'] === 'N'" size="small" type="info" style="margin-right: 4px">禁传文件</el-tag>
          <el-tag v-if="row.strategy?.['enable-clipboard'] === 'N'" size="small" type="info" style="margin-right: 4px">禁剪贴板</el-tag>
          <el-tag v-if="row.strategy?.['disable-settings'] === 'Y'" size="small" type="warning">锁定设置</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDialog(row)">配置策略</el-button>
          <el-popconfirm
            v-if="row.id > 2"
            title="删除后该分组设备将移回默认机房，确认删除？"
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
            <el-option label="仅使用固定密码" value="use-permanent-password" />
            <el-option label="同时支持固定与临时密码" value="use-both-passwords" />
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
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/client'

const loading = ref(false)
const groups = ref<any[]>([])
const dialogVisible = ref(false)

const form = reactive<any>({
  id: 0,
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

async function load() {
  loading.value = true
  try {
    const res: any = await api.get('/groups')
    groups.value = res.items || []
  } finally {
    loading.value = false
  }
}

function openDialog(row?: any) {
  if (row) {
    form.id = row.id
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

async function deleteGroup(id: number) {
  await api.delete(`/groups/${id}`)
  ElMessage.success('已删除分组')
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
