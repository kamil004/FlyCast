import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'

const API = '/api'

export const useAuthStore = defineStore('auth', () => {
  const user = ref(null)
  const token = ref(localStorage.getItem('flycast_token') || '')
  const hasUsers = ref(true)
  const initialized = ref(false)

  const isAdmin = computed(() => !!(user.value && user.value.is_admin))
  const isGuest = computed(() => !!(user.value && user.value.is_guest))

  if (token.value) {
    axios.defaults.headers.common['Authorization'] = 'Bearer ' + token.value
  }

  async function checkInit() {
    try {
      const res = await axios.get(API + '/auth/status')
      hasUsers.value = !!res.data.hasUsers
    } catch (e) {
      hasUsers.value = true
    }
    if (token.value) {
      try {
        const me = await axios.get(API + '/users/me')
        user.value = me.data
      } catch (e) {
        logout()
      }
    }
    initialized.value = true
  }

  async function initAdmin(username, password) {
    const res = await axios.post(API + '/auth/init', { username, password })
    token.value = res.data.token
    localStorage.setItem('flycast_token', token.value)
    axios.defaults.headers.common['Authorization'] = 'Bearer ' + token.value
    user.value = res.data.user
    hasUsers.value = true
  }

  async function login(username, password) {
    const res = await axios.post(API + '/auth/login', { username, password })
    token.value = res.data.token
    localStorage.setItem('flycast_token', token.value)
    axios.defaults.headers.common['Authorization'] = 'Bearer ' + token.value
    user.value = res.data.user
  }

  async function guestLogin() {
    const res = await axios.post(API + '/auth/guest')
    token.value = res.data.token
    localStorage.setItem('flycast_token', token.value)
    axios.defaults.headers.common['Authorization'] = 'Bearer ' + token.value
    user.value = res.data.user
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('flycast_token')
    delete axios.defaults.headers.common['Authorization']
  }

  return { user, token, hasUsers, initialized, isAdmin, isGuest, checkInit, initAdmin, login, guestLogin, logout }
})
