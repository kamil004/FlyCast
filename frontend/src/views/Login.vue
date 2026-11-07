<template>
  <div class="auth-wrap">
    <div class="auth-card">
      <div class="brand">
        <span class="logo">▶</span>
        <span>FlyCast</span>
      </div>
      <p class="hint">Log in to continue.</p>
      <form @submit.prevent="submit">
        <div class="field">
          <label>Username</label>
          <input class="input" v-model="username" autocomplete="username" required />
        </div>
        <div class="field">
          <label>Password</label>
          <input class="input" v-model="password" type="password" autocomplete="current-password" required />
        </div>
        <div class="btn-row">
          <button class="btn primary" type="submit">Log in</button>
          <button class="btn" type="button" :disabled="busy" @click="guest">Guest</button>
        </div>
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
const busy = ref(false)

async function submit() {
  try {
    error.value = ''
    await auth.login(username.value, password.value)
    router.push({ name: 'home' })
  } catch (e) {
    error.value = 'Invalid username or password'
  }
}

async function guest() {
  busy.value = true
  try {
    error.value = ''
    await auth.guestLogin()
    router.push({ name: 'home' })
  } catch (e) {
    const err = e?.response?.data?.err
    error.value =
      err === 'no_guest'
        ? 'No guest account. Ask an administrator to create one.'
        : 'Failed to log in as guest.'
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.btn-row {
  display: flex;
  gap: 10px;
  margin-top: 4px;
}
.btn-row .btn {
  flex: 1 1 50%;
  justify-content: center;
}
</style>
