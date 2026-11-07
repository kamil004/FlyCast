<template>
  <aside class="sidebar">
    <div class="side-head">
      <router-link to="/" class="brand">
        <span class="logo">▶</span>
        <span>FlyCast</span>
      </router-link>
      <button class="collapse" :title="t('topbar.hide_menu')" @click="emit('collapse')">«</button>
    </div>

    <nav class="nav">
      <router-link to="/" :class="{ active: isHome }">
        <span class="ico">🏠</span>
        <span>{{ t('sidebar.home') }}</span>
      </router-link>

      <div class="group-label">{{ t('sidebar.libraries') }}</div>
      <router-link
        v-for="lib in libraries"
        :key="lib.id"
        :to="{ name: 'library', query: { folder: lib.id, name: lib.name } }"
        :class="{ active: isLibraryActive(lib) }"
      >
        <span class="ico">📁</span>
        <span class="lib-name">{{ lib.name }}</span>
        <span class="count">{{ lib.count }}</span>
      </router-link>

      <template v-if="!auth.isGuest">
        <div class="group-label">{{ t('sidebar.collections') }}</div>
        <router-link to="/favorites" :class="{ active: isFavorites }">
          <span class="ico">⭐</span>
          <span>{{ t('sidebar.favorites') }}</span>
        </router-link>
        <router-link to="/albums" :class="{ active: isAlbums }">
          <span class="ico">🗂️</span>
          <span>{{ t('sidebar.albums') }}</span>
        </router-link>
      </template>

      <template v-if="auth.user && auth.user.is_admin">
        <div class="group-label">{{ t('sidebar.management') }}</div>
        <router-link to="/libraries" :class="{ active: route.name === 'libraries' }">
          <span class="ico">📚</span>
          <span>{{ t('sidebar.libs') }}</span>
        </router-link>
      </template>

      <router-link v-if="auth.user && !auth.isGuest" to="/settings" :class="{ active: route.name === 'settings' }">
        <span class="ico">⚙️</span>
        <span>{{ t('sidebar.settings') }}</span>
      </router-link>
    </nav>

    <div class="foot" v-if="auth.user">
      <div class="avatar">{{ initials }}</div>
      <div class="who">
        <span class="name">{{ auth.user.username }}</span>
        <span class="role">{{ roleLabel }}</span>
      </div>
      <button class="btn ghost" style="margin-left:auto" :title="t('sidebar.logout')" @click="logout">⎋</button>
    </div>
  </aside>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import { useAuthStore } from '@/stores/auth'
import { t } from '@/i18n'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const libraries = ref([])
const emit = defineEmits(['collapse'])

function logout() {
  auth.logout()
  router.push({ name: 'login' })
}

const initials = computed(() => (auth.user?.username || '?').slice(0, 2))
const roleLabel = computed(() => {
  if (auth.user?.is_admin) return t('sidebar.admin')
  if (auth.user?.is_guest) return t('sidebar.guest')
  return t('sidebar.user')
})
const isHome = computed(() => route.name === 'home')
const isAlbums = computed(() => route.name === 'albums' || route.name === 'album-view')
const isFavorites = computed(() => route.name === 'favorites')

function isLibraryActive(lib) {
  return route.name === 'library' && !route.query.all && route.query.folder === lib.id
}

async function loadLibraries() {
  try {
    const res = await axios.get('/api/library/libraries')
    libraries.value = res.data.items || []
  } catch (e) {
    libraries.value = []
  }
}

onMounted(loadLibraries)
</script>

<style scoped>
.side-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-right: 12px;
}
.side-head .brand {
  flex: 1;
  min-width: 0;
}
.collapse {
  flex-shrink: 0;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--dim);
  cursor: pointer;
  padding: 4px 8px;
  font-size: 14px;
  line-height: 1;
}
.collapse:hover {
  color: var(--text);
  border-color: #3a3a4d;
}
.lib-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
