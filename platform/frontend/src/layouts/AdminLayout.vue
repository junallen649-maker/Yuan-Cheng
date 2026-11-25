<template>
  <el-container class="layout-root">
    <el-aside width="232px" class="sidebar">
      <div class="sidebar-brand">
        <div class="logo-box">
          <el-icon :size="20" color="#fff"><Monitor /></el-icon>
        </div>
        <div class="brand-text">
          <div class="title">Polaris 校园后台</div>
          <div class="subtitle">Console & Agent 统一管控</div>
        </div>
      </div>

      <el-menu :default-active="route.path" router class="nav-menu">
        <el-menu-item index="/overview">
          <el-icon><DataBoard /></el-icon>
          <span>总览大盘</span>
        </el-menu-item>
        <el-menu-item index="/schools">
          <el-icon><School /></el-icon>
          <span>学校与发号</span>
        </el-menu-item>
        <el-menu-item index="/devices">
          <el-icon><Cpu /></el-icon>
          <span>全部终端</span>
        </el-menu-item>
        <el-menu-item index="/sessions">
          <el-icon><Connection /></el-icon>
          <span>实时会话</span>
        </el-menu-item>
        <el-menu-item index="/groups">
          <el-icon><FolderOpened /></el-icon>
          <span>机房与策略</span>
        </el-menu-item>
        <el-menu-item v-if="isAdmin" index="/users">
          <el-icon><UserFilled /></el-icon>
          <span>账号与权限</span>
        </el-menu-item>
        <el-menu-item index="/audits">
          <el-icon><Document /></el-icon>
          <span>审计日志</span>
        </el-menu-item>
        <el-menu-item index="/settings">
          <el-icon><Setting /></el-icon>
          <span>系统设置</span>
        </el-menu-item>
      </el-menu>

      <div class="sidebar-footer">
        <div class="status-pill">
          <span class="dot"></span>
          <span>easyreach.asia 在线</span>
        </div>
      </div>
    </el-aside>

    <el-container>
      <el-header class="topbar">
        <div class="page-title">{{ route.meta.title || '管理平台' }}</div>
        <div class="topbar-right">
          <el-tag size="small" effect="plain" type="primary">
            {{ user?.is_admin ? '超级管理员' : (user?.role === 'teacher' ? '教师/运维' : '只读观察') }}
          </el-tag>
          <el-dropdown @command="onMenuCommand">
            <span class="user-trigger">
              <el-avatar :size="28" style="background: #1e6fff">
                {{ (user?.display_name || user?.username || 'A').slice(0, 1).toUpperCase() }}
              </el-avatar>
              <span>{{ user?.display_name || user?.username || '管理员' }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="settings">修改密码 / 设置</el-dropdown-item>
                <el-dropdown-item divided command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

const user = computed(() => {
  try {
    return JSON.parse(localStorage.getItem('polaris_admin_user') || '{}')
  } catch {
    return {}
  }
})

const isAdmin = computed(() => Boolean(user.value?.is_admin || user.value?.role === 'admin'))

function onMenuCommand(cmd: string) {
  if (cmd === 'settings') {
    router.push('/settings')
  } else if (cmd === 'logout') {
    localStorage.removeItem('polaris_admin_token')
    localStorage.removeItem('polaris_admin_user')
    router.push('/login')
  }
}
</script>

<style scoped>
.layout-root {
  height: 100vh;
}

.sidebar {
  background: #ffffff;
  border-right: 1px solid var(--polaris-border);
  display: flex;
  flex-direction: column;
}

.sidebar-brand {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 20px 18px 16px;
  border-bottom: 1px solid #f1f5f9;
}

.logo-box {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: linear-gradient(135deg, #3b82f6, #1e6fff);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.brand-text .title {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
}

.brand-text .subtitle {
  font-size: 11px;
  color: #64748b;
  margin-top: 2px;
}

.nav-menu {
  flex: 1;
  border-right: none;
  padding: 10px 8px;
}

.nav-menu :deep(.el-menu-item) {
  border-radius: 8px;
  margin-bottom: 4px;
  height: 42px;
}

.nav-menu :deep(.el-menu-item.is-active) {
  background-color: rgba(30, 111, 255, 0.1);
  color: #1e6fff;
  font-weight: 600;
}

.sidebar-footer {
  padding: 14px 16px;
  border-top: 1px solid #f1f5f9;
}

.status-pill {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: #475569;
}

.status-pill .dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 6px rgba(16, 185, 129, 0.6);
}

.topbar {
  height: 58px;
  background: #ffffff;
  border-bottom: 1px solid var(--polaris-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
}

.page-title {
  font-size: 17px;
  font-weight: 600;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 14px;
}

.user-trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 14px;
}

.main-content {
  padding: 20px 24px;
  overflow-y: auto;
}
</style>
