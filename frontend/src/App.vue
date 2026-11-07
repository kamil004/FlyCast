<template>
  <div v-if="!auth.user" class="guest">
    <router-view />
  </div>
  <div v-else class="shell" :class="{ collapsed: !sidebarOpen }">
    <AppSidebar v-show="sidebarOpen" @collapse="closeSidebar" />
    <button
      v-if="!sidebarOpen"
      class="sidebar-tab"
      title="Show menu"
      @click="toggleSidebar"
    >›</button>
    <div class="content">
      <header class="topbar">
        <span class="page-title">{{ pageTitle }}</span>
        <button v-if="showBack" class="back-btn" title="Back" @click="goBack">← Back</button>
        <div class="topbar-right">
          <span v-if="appVersion" class="app-ver" title="Wersja aplikacji">v{{ appVersion }}</span>
          <div class="search">
            <span class="ico">🔎</span>
            <input
              v-model="query"
              placeholder="Search movies..."
              @keyup.enter="doSearch"
            />
          </div>
        </div>
      </header>
      <router-view />
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useFavoritesStore } from '@/stores/favorites'
import AppSidebar from '@/components/AppSidebar.vue'

const auth = useAuthStore()
const fav = useFavoritesStore()
const route = useRoute()
const router = useRouter()
const query = ref('')
const appVersion = import.meta.env.VITE_FLYCAST_VERSION || ''

function initialSidebar() {
  const saved = localStorage.getItem('flycast_sidebar')
  if (saved === '0') return false
  if (saved === '1') return true
  return window.innerWidth > 768
}
const sidebarOpen = ref(initialSidebar())
function persistSidebar() {
  localStorage.setItem('flycast_sidebar', sidebarOpen.value ? '1' : '0')
}
function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value
  persistSidebar()
}
function closeSidebar() {
  sidebarOpen.value = false
  persistSidebar()
}

watch(
  () => auth.user,
  (u) => {
    if (!u) {
      fav.reset()
      if (route.meta?.auth) router.push({ name: 'login' })
    } else {
      fav.load(true)
    }
  },
  { immediate: true }
)

const titles = {
  home: 'Home',
  library: 'Library',
  albums: 'Albums',
  'album-view': 'Album',
  favorites: 'Favorites',
  detail: 'Details',
  player: 'Player',
  settings: 'Settings'
const pageTitle = computed(() => titles[route.name] || 'FlyCast')

// Pages that are "deeper" than the main navigation get a back button so the
// user can always step back (detail page, player, album, folder browsing).
const backRoutes = ['detail', 'player', 'album-view', 'library']
const showBack = computed(() => backRoutes.includes(route.name))

function goBack() {
  const back = window.history.state && window.history.state.back
  if (back) router.back()
  else router.push({ name: 'home' })
}

function doSearch() {
  const q = query.value.trim()
  if (!q) return
  router.push({ name: 'library', query: { q } })
}
</script>

<style scoped>
.guest {
  height: 100vh;
}
.sidebar-tab {
  position: fixed;
  top: 12px;
  left: 10px;
  z-index: 30;
  width: 34px;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--sidebar);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--dim);
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
  padding: 0;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.35);
}
.sidebar-tab:hover {
  color: var(--text);
  background: var(--card-hover);
}
.back-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg-2);
  color: var(--dim);
  font-size: 13px;
  line-height: 1;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}
.back-btn:hover {
  color: var(--text);
  border-color: #3a3a4d;
  background: var(--card-hover);
}
.topbar-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 12px;
}
.app-ver {
  font-size: 12px;
  color: var(--dim);
  background: var(--bg-2);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 4px 8px;
  white-space: nowrap;
  user-select: none;
}
</style>
