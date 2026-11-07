<template>
  <main class="page">
    <div class="auth-wrap">
      <div class="auth-card">
        <div class="brand">
          <span class="logo">▶</span>
          <span>FlyCast</span>
        </div>
        <p class="hint">{{ t('setup.hint') }}</p>
        <form @submit.prevent="submit">
          <div class="field">
            <label>{{ t('login.login_label') }}</label>
            <input class="input" v-model="username" required />
          </div>
          <div class="field">
            <label>{{ t('login.password_label') }}</label>
            <input class="input" v-model="password" type="password" required />
          </div>
          <button class="btn primary" type="submit" style="justify-content:center">{{ t('setup.create_btn') }}</button>
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

async function submit() {
  try {
    error.value = ''
    await auth.initAdmin(username.value, password.value)
    router.push({ name: 'home' })
  } catch (e) {
    error.value = t('setup.error')
  }
}
</script>
