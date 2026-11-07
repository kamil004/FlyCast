<template>
  <div class="auth-wrap">
    <div class="auth-card">
      <div class="brand">
        <span class="logo">▶</span>
        <span>FlyCast</span>
      </div>
      <p class="hint">Initial setup — create an administrator account.</p>
      <form @submit.prevent="submit">
        <div class="field">
          <label>Username</label>
          <input class="input" v-model="username" required />
        </div>
        <div class="field">
          <label>Password</label>
          <input class="input" v-model="password" type="password" required />
        </div>
        <button class="btn primary" type="submit" style="justify-content:center">Create Account</button>
        <p v-if="error" class="error">{{ error }}</p>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const username = ref('')
const password = ref('')
const error = ref('')

async function submit() {
  try {
    error.value = ''
    await auth.initAdmin(username.value, password.value)
    router.push({ name: 'home' })
  } catch (e) {
    error.value = 'Failed to create account'
  }
}
</script>
