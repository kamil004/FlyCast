<template>
  <main class="page">
    <div class="auth-wrap">
      <div class="auth-card">
        <div class="brand">
          <span class="logo">▶</span>
          <span>FlyCast</span>
        </div>
        <p class="hint">{{ t('login.hint') }}</p>
        <form @submit.prevent="submit">
          <div class="field">
            <label>{{ t('login.login_label') }}</label>
            <input class="input" v-model="username" autocomplete="username" required />
          </div>
          <div class="field">
            <label>{{ t('login.password_label') }}</label>
            <input class="input" v-model="password" type="password" autocomplete="current-password" required />
          </div>
          <div class="btn-row">
            <button class="btn primary" type="submit">{{ t('login.login_btn') }}</button>
            <button class="btn" type="button" :disabled="busy" @click="guest">{{ t('login.guest_btn') }}</button>
          </div>
          <p v-if="error" class="error">{{ error }}</p>
        </form>
      </div>
    </div>
  </main>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { t } from '@/i18n'

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
    error.value = t('login.error')
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
    error.value = err === 'no_guest'
      ? t('login.no_guest')
      : t('login.guest_failed')
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
