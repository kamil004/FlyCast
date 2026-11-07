import { createRouter, createWebHistory } from 'vue-router'
import Home from '@/views/Home.vue'
import Library from '@/views/Library.vue'
import Player from '@/views/Player.vue'
import Login from '@/views/Login.vue'
import Setup from '@/views/Setup.vue'
import Albums from '@/views/Albums.vue'
import AlbumView from '@/views/AlbumView.vue'
import Favorites from '@/views/Favorites.vue'
import Settings from '@/views/Settings.vue'
import Detail from '@/views/Detail.vue'
import Libraries from '@/views/Libraries.vue'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: Home, meta: { auth: true } },
    { path: '/setup', name: 'setup', component: Setup },
    { path: '/login', name: 'login', component: Login },
    { path: '/library', name: 'library', component: Library, meta: { auth: true } },
    { path: '/detail/:id', name: 'detail', component: Detail, meta: { auth: true } },
    { path: '/albums', name: 'albums', component: Albums, meta: { auth: true, noGuest: true } },
    { path: '/albums/:id', name: 'album-view', component: AlbumView, meta: { auth: true, noGuest: true } },
    { path: '/favorites', name: 'favorites', component: Favorites, meta: { auth: true, noGuest: true } },
    { path: '/player/:id', name: 'player', component: Player, meta: { auth: true } },
    { path: '/libraries', name: 'libraries', component: Libraries, meta: { auth: true, admin: true } },
    { path: '/settings', name: 'settings', component: Settings, meta: { auth: true } }
  ]
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.initialized) {
    await auth.checkInit()
  }
  if (!auth.hasUsers && to.name !== 'setup') {
    return { name: 'setup' }
  }
  if (to.meta.auth && !auth.user) {
    return { name: 'login' }
  }
  if (to.name === 'login' && auth.user) {
    return { name: 'home' }
  }
  if (to.name === 'setup' && auth.hasUsers) {
    return { name: 'login' }
  }
  if (to.meta.guest && auth.user) {
    return { name: 'home' }
  }
  if (to.meta.admin && !auth.isAdmin) {
    return { name: 'home' }
  }
  if (to.meta.noGuest && auth.isGuest) {
    return { name: 'home' }
  }
  return true
})

export default router
