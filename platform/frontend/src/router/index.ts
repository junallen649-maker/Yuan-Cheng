import { createRouter, createWebHashHistory } from 'vue-router'
import AdminLayout from '@/layouts/AdminLayout.vue'
import LoginView from '@/views/LoginView.vue'
import OverviewView from '@/views/OverviewView.vue'
import SchoolsView from '@/views/SchoolsView.vue'
import DevicesView from '@/views/DevicesView.vue'
import SessionsView from '@/views/SessionsView.vue'
import GroupsView from '@/views/GroupsView.vue'
import UsersView from '@/views/UsersView.vue'
import AuditsView from '@/views/AuditsView.vue'
import SettingsView from '@/views/SettingsView.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: LoginView
    },
    {
      path: '/',
      component: AdminLayout,
      redirect: '/overview',
      children: [
        { path: 'overview', name: 'overview', component: OverviewView, meta: { title: '总览大盘' } },
        { path: 'schools', name: 'schools', component: SchoolsView, meta: { title: '学校与发号' } },
        { path: 'devices', name: 'devices', component: DevicesView, meta: { title: '全部终端' } },
        { path: 'sessions', name: 'sessions', component: SessionsView, meta: { title: '实时会话' } },
        { path: 'groups', name: 'groups', component: GroupsView, meta: { title: '机房与策略' } },
        { path: 'users', name: 'users', component: UsersView, meta: { title: '账号与权限' } },
        { path: 'audits', name: 'audits', component: AuditsView, meta: { title: '审计日志' } },
        { path: 'settings', name: 'settings', component: SettingsView, meta: { title: '系统设置' } }
      ]
    }
  ]
})

router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('polaris_admin_token')
  if (to.path !== '/login' && !token) {
    next('/login')
  } else if (to.path === '/login' && token) {
    next('/overview')
  } else {
    next()
  }
})

export default router
