<template>
  <main class="page">
    <div class="crumbs">
      <router-link :to="{ name: 'library' }">{{ t('library.breadcrumb') }}</router-link>
      <template v-for="(cr, i) in crumbs" :key="i">
        <span class="sep">/</span>
        <router-link v-if="i < crumbs.length - 1" :to="{ name: 'library', query: cr.q }">
          {{ cr.name }}
        </router-link>
        <span v-else>{{ cr.name }}</span>
      </template>
      <template v-if="searchMode">
        <span class="sep">/</span>
        <span>{{ t('library.results', { q }) }}</span>
      </template>
      <template v-if="allMode">
        <span class="sep">/</span>
        <span>{{ t('library.all_files') }}</span>
      </template>
    </div>

    <div v-if="loading" class="loading">{{ t('library.loading') }}</div>

    <template v-else>
      <div v-if="folders.length || videos.length" class="lib-toolbar">
        <div class="tb-group">
          <span class="tb-label">{{ t('library.sort') }}</span>
          <select v-model="sortBy" class="tb-select" @change="onSortChange">
            <option value="name">{{ t('library.sort_name') }}</option>
            <option value="date">{{ t('library.sort_date') }}</option>
            <option value="filedate">{{ t('library.sort_filedate') }}</option>
            <option value="size">{{ t('library.sort_size') }}</option>
          </select>
          <button
            class="tb-btn"
            :title="sortDir === 'asc' ? t('library.ascending') : t('library.descending')"
            @click="sortDir = sortDir === 'asc' ? 'desc' : 'asc'"
          >{{ sortDir === 'asc' ? '↑' : '↓' }}</button>

          <div v-if="canFetchMetadata" class="tb-group tb-meta">
            <button
              class="tb-btn meta"
              :disabled="metaJob.running"
              :title="metaJob.running ? t('library.meta_downloading') : t('library.meta_download')"
              @click="fetchLibraryMetadata"
            >
              <span>{{ t('library.meta_download_btn') }}</span>
            </button>
            <select v-model="metaSource" class="tb-select" :disabled="metaJob.running" :title="t('library.meta_source')">
              <option value="imdb">IMDb</option>
              <option value="filmweb">Filmweb</option>
            </select>
            <span v-if="metaJob.running" class="tb-label meta-progress">
              {{ metaJob.done }}/{{ metaJob.total }}
              <button class="meta-cancel" :title="t('library.meta_cancel')" @click="cancelMetadata">✕</button>
            </span>
            <span v-else-if="metaNotice" class="tb-label meta-progress">{{ metaNotice }}</span>
          </div>
        </div>
        <div class="tb-group">
          <button
            class="tb-btn" :class="{ active: viewMode === 'grid' }"
            :title="t('library.view_thumbs')" @click="viewMode = 'grid'"
          >▦</button>
          <button
            class="tb-btn" :class="{ active: viewMode === 'large' }"
            :title="t('library.view_large')" @click="viewMode = 'large'"
          >▣</button>
          <button
            class="tb-btn" :class="{ active: viewMode === 'list' }"
            :title="t('library.view_list')" @click="viewMode = 'list'"
          >☰</button>
        </div>
      </div>

      <div v-if="folders.length" class="lib-grid" :class="gridClass()" style="margin-bottom:26px">
        <div
          v-for="f in sortedFolders"
          :key="f.id"
          class="lib-card clickable"
          @click="openFolder(f)"
        >
          <LibThumb :thumbs="f.thumbs" :poster="isPosterLib" />
          <div class="meta">
            <div class="n">{{ f.name }}</div>
            <div class="c">{{ f.count || 0 }} {{ t('library.files') }}</div>
            <div class="c" v-if="fmtSize(f.size)">{{ fmtSize(f.size) }}</div>
          </div>
          <button
            v-if="isAdmin"
            class="add-lib"
            :title="t('library.add_as_lib')"
            @click.stop="addAsLibrary(f)"
            style="right:8px;width:20px"
          >＋</button>
        </div>
      </div>

      <div v-if="videos.length" class="grid" :class="gridClass()">
        <MediaCard v-for="v in sortedVideos" :key="v.id" :item="v" :list="viewMode === 'list'" />
      </div>

      <div v-if="!folders.length && !videos.length" class="empty">
        <div class="big">🔍</div>
        <div v-if="searchMode">{{ t('library.no_results', { q }) }}</div>
        <div v-else>{{ t('library.no_media') }}</div>
      </div>
    </template>
  </main>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import MediaCard from '@/components/MediaCard.vue'
import LibThumb from '@/components/LibThumb.vue'
import { useAuthStore } from '@/stores/auth'
import { bumpThumbs } from '@/composables/media.js'
import { t } from '@/i18n'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const isAdmin = computed(() => !!auth.user?.is_admin)

const sortBy = ref(localStorage.getItem('flycast_sort') || 'name')
const sortDir = ref(localStorage.getItem('flycast_sort_dir') || 'asc')
const viewMode = ref(localStorage.getItem('flycast_view') || 'grid')
watch(sortBy, (v) => localStorage.setItem('flycast_sort', v))
watch(sortDir, (v) => localStorage.setItem('flycast_sort_dir', v))
watch(viewMode, (v) => localStorage.setItem('flycast_view', v))

const dirMul = computed(() => (sortDir.value === 'asc' ? 1 : -1))

function onSortChange() {
  sortDir.value = sortBy.value === 'name' ? 'asc' : 'desc'
}

const sortedFolders = computed(() => {
  const arr = [...folders.value]
  arr.sort((a, b) =>
    (a.name || '').localeCompare(b.name || '', 'pl', { numeric: true, sensitivity: 'base' }) * dirMul.value
  )
  return arr
})

const sortedVideos = computed(() => {
  const arr = [...videos.value]
  arr.sort((a, b) => {
    let r = 0
    if (sortBy.value === 'name') {
      r = (a.title || a.filename || '').localeCompare(
        b.title || b.filename || '', 'pl', { numeric: true, sensitivity: 'base' })
    } else if (sortBy.value === 'date') {
      r = String(a.created_at || '').localeCompare(String(b.created_at || ''))
    } else if (sortBy.value === 'filedate') {
      r = (a.mtime || 0) - (b.mtime || 0)
    } else if (sortBy.value === 'size') {
      r = (a.size || 0) - (b.size || 0)
    }
    return r * dirMul.value
  })
  return arr
})

function openFolder(f) {
  router.push({ name: 'library', query: { folder: f.id, name: f.name } })
}

function fmtSize(bytes) {
  let n = Number(bytes) || 0
  if (n <= 0) return ''
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  const dec = n >= 100 || i === 0 ? 0 : 1
  return n.toFixed(dec) + ' ' + units[i]
}

function addAsLibrary(f) {
  router.push({ name: 'libraries', query: { add: f.path } })
}

const loading = ref(true)
const folders = ref([])
const videos = ref([])
const crumbs = ref([])
const q = ref('')
const searchMode = ref(false)
const allMode = ref(false)

const mediaType = ref('')
const libraryId = ref('')
const metaScopeId = computed(() => route.query.folder || libraryId.value)
const metaJob = ref({ running: false, done: 0, total: 0, updated: 0, failed: 0 })
const metaNotice = ref('')
const metaSource = ref(localStorage.getItem('flycast_meta_source') || 'imdb')
let metaTimer = null
watch(metaSource, (v) => localStorage.setItem('flycast_meta_source', v))
const isPosterLib = computed(() => mediaType.value === 'movies' || mediaType.value === 'tv')

const canFetchMetadata = computed(
  () => (mediaType.value === 'movies' || mediaType.value === 'tv') && !!metaScopeId.value
)

function gridClass() {
  return 'view-' + viewMode.value + (isPosterLib.value ? ' posters' : '')
}

async function fetchLibraryMetadata() {
  if (metaJob.value.running) return
  metaNotice.value = ''
  try {
    await axios.post(`/api/metadata/library/${metaScopeId.value}/refresh`, {
      source: metaSource.value
    })
    metaJob.value = { running: true, done: 0, total: 0, updated: 0, failed: 0 }
    pollMetaJob()
  } catch (e) {
    metaNotice.value = e.response?.data?.err === 'busy'
      ? t('library.busy')
      : t('library.failed_start')
  }
}

function pollMetaJob() {
  clearTimeout(metaTimer)
  metaTimer = setTimeout(async () => {
    try {
      const res = await axios.get('/api/metadata/job')
      metaJob.value = res.data
      if (res.data.running) {
        pollMetaJob()
      } else {
        bumpThumbs()
        metaNotice.value = t('library.meta_progress_done', { updated: res.data.updated, failed: res.data.failed })
        setTimeout(() => (metaNotice.value = ''), 8000)
        await load()
      }
    } catch (e) {
      metaJob.value.running = false
    }
  }, 1200)
}

async function cancelMetadata() {
  try {
    await axios.post('/api/metadata/job/cancel')
  } catch (e) {
    /* ignore */
  }
}

function notHidden(item) {
  return !(item.name || '').startsWith('.')
}

async function load() {
  loading.value = true
  folders.value = []
  videos.value = []
  mediaType.value = ''
  libraryId.value = ''
  const query = route.query

  if (query.q) {
    searchMode.value = true
    allMode.value = false
    q.value = query.q
    const res = await axios.get('/api/library/search', { params: { q: query.q } })
    videos.value = res.data.items || []
    crumbs.value = []
  } else if (query.all) {
    searchMode.value = false
    allMode.value = true
    const res = await axios.get('/api/library/all', { params: { limit: 500 } })
    videos.value = res.data.items || []
    crumbs.value = []
  } else if (query.folder) {
    searchMode.value = false
    allMode.value = false
    const res = await axios.get('/api/library/browse', { params: { folder: query.folder } })
    const items = res.data.items || []
    folders.value = items.filter((x) => x.type === 'folder' && notHidden(x))
    videos.value = items.filter((x) => x.type === 'video')
    mediaType.value = res.data.media_type || ''
    libraryId.value = res.data.library_id || ''
    crumbs.value = (res.data.crumbs || []).map((c) => ({
      name: c.name,
      q: { folder: c.id, name: c.name }
    }))
  } else {
    searchMode.value = false
    allMode.value = false
    crumbs.value = []
    const res = await axios.get('/api/library/browse')
    folders.value = (res.data.items || []).filter(notHidden)
  }
  loading.value = false
}

onMounted(async () => {
  await load()
  try {
    const res = await axios.get('/api/metadata/job')
    if (res.data.running) {
      metaJob.value = res.data
      pollMetaJob()
    }
  } catch (e) {
    /* ignore */
  }
})
onUnmounted(() => clearTimeout(metaTimer))
watch(() => route.query, load)
</script>
