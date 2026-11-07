<template>
  <div class="auth-wrap">
    <div class="auth-card">
      <div class="brand">
        <span class="logo">▶</span>
        <span>FlyCast</span>
      </div>
      <p class="hint">Pierwsza konfiguracja — utwórz konto administratora.</p>
      <form @submit.prevent="submit">
        <div class="field">
          <label>Login</label>
          <input class="input" v-model="username" required />
        </div>
        <div class="field">
          <label>Hasło</label>
          <input class="input" v-model="password" type="password" required />
        </div>
        <button class="btn primary" type="submit" style="justify-content:center">Utwórz konto</button>
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
    error.value = 'Nie udało się utworzyć konta'
  }
}
</script>
