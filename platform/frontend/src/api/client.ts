import axios from 'axios'
import { ElMessage } from 'element-plus'

const api = axios.create({
  baseURL: './api/admin',
  timeout: 15000
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('polaris_admin_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (resp) => resp.data,
  (err) => {
    const status = err?.response?.status
    const msg = err?.response?.data?.error || err.message || '请求失败'
    if (status === 401) {
      localStorage.removeItem('polaris_admin_token')
      localStorage.removeItem('polaris_admin_user')
      if (!window.location.hash.includes('/login')) {
        window.location.hash = '#/login'
      }
    } else {
      ElMessage.error(msg)
    }
    return Promise.reject(err)
  }
)

export default api
