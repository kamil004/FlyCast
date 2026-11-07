<template>
  <aside class="sidebar">
    <div class="side-head">
      <router-link to="/" class="brand">
        <span class="logo">▶</span>
        <span>FlyCast</span>
      </router-link>
      <button class="collapse" title="Hide menu" @click="emit('collapse')">«</button>
    </div>

    <nav class="nav">
      <router-link to="/" :class="{ active: isHome }">
        <span class="ico">🏠</span>
        <span>Home</span>
      </router-link>

      <div class="group-label">Libraries</div>
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
        <div class="group-label">Collections</div>
        <router-link to="/favorites" :class="{ active: isFavorites }">
          <span class="ico">⭐</span>
          <span>Favorites</span>
        </router-link>
        <router-link to="/albums" :class="{ active: isAlbums }">
          <span class="ico">🗂️</span>
          <span>Albums</span>
        </router-link>
      </template>

      <template v-if="auth.user && auth.user.is_admin">
        <div class="group-label">Management</div>
        <router-link to="/libraries" :class="{ active: route.name === 'libraries' }">
          <span class="ico">📚</span>
          <span>Libraries</span>
        </router-link>
      </template>

      <router-link to="/settings" :class="{ active: route.name === 'settings' }">
        <span class="ico">⚙️</span>
        <span>Settings</span>
      </router-link>
    </nav>

    <div class="foot" v-if="auth.user">
      <div class="avatar">{{ initials }}</div>
      <div class="who">
        <span class="name">{{ auth.user.username }}</span>
        <span class="role">{{ roleLabel }}</span>
      </div>
      <button class="btn ghost" style="margin-left:auto" title="Logout" @click="logout">⎋</button>
    </div>
  </aside>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import { useAuthStore } from '@/stores/auth'

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
const roleLabel = computed(() =>
  auth.user?.is_admin ? 'Administrator' : auth.user?.is_guest ? 'Guest' : 'User'
)
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
