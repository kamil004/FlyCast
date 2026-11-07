<template>
  <main class="page">
    <div v-if="loading" class="loading">{{ t('detail.loading') }}</div>

    <template v-else-if="video">
      <div class="detail-head">
        <img v-if="backdropSrc" class="bg" :src="backdropSrc" @error="hideBg" />
        <div class="scrim"></div>
        <div class="inner">
          <div class="poster" :style="{ aspectRatio: posterRatio }">
            <img v-if="posterSrc" :src="posterSrc" @error="hideImg" @load="onPosterLoad" />
            <div v-else class="ph" style="display:flex;align-items:center;justify-content:center;height:100%;color:var(--dim-2);font-size:30px">🎞️</div>
          </div>
          <div class="info">
            <h1>{{ video.title || video.filename }}</h1>
            <div class="sub">
              <span v-if="video.year">{{ video.year }}</span>
              <span v-if="video.duration">{{ fmt(video.duration) }}</span>
              <span v-if="video.size">{{ size(video.size) }}</span>
              <span v-if="video.metadata_source">{{ t('detail.source') }} {{ video.metadata_source }}</span>
            </div>
            <div class="actions">
              <router-link class="btn primary" :to="{ name: 'player', params: { id: video.id } }">
                {{ t('detail.play') }}
              </router-link>
              <button
                class="btn"
                :disabled="!hasProgress"
                :title="hasProgress ? t('detail.resume_title') : t('detail.no_progress')"
                @click="resume"
              >
                {{ t('detail.resume') }}
              </button>
              <template v-if="!auth.isGuest">
                <select v-model="metaSource" class="tb-select" :disabled="refreshing">
                  <option value="imdb">IMDb</option>
                  <option value="filmweb">Filmweb</option>
                </select>
                <button class="btn" :disabled="refreshing" @click="refreshMetadata">
                  {{ refreshing ? t('detail.searching') : t('detail.download_meta') }}
                </button>
                <button class="btn" @click="startEdit">{{ t('detail.edit') }}</button>
                <button v-if="isAdmin" class="btn danger" :disabled="deleting" @click="deleteVideo">
                  {{ deleting ? t('detail.deleting') : t('detail.delete') }}
                </button>
                <button
                  v-if="isAdmin"
                  class="btn"
                  style="margin-left:24px"
                  :title="t('detail.convert_title')"
                  :disabled="converting"
                  @click="convertVideo"
                >
                  {{ converting ? t('detail.converting', { pct: Math.round(convertPct) }) : t('detail.convert_btn') }}
                </button>
              </template>
            </div>
          </div>
        </div>
      </div>

      <div class="detail-body">
        <div class="chips">
          <span class="chip">{{ video.container || t('detail.container_unknown') }}</span>
          <span class="chip" v-if="video.imdb_id">{{ t('detail.imdb_label') }} {{ video.imdb_id }}</span>
          <span class="chip" v-if="video.tmdb_id">{{ t('detail.tmdb_label') }} {{ video.tmdb_id }}</span>
        </div>

        <div v-if="video.overview" class="overview">{{ video.overview }}</div>
        <p v-else class="muted">
          <template v-if="auth.isGuest">{{ t('detail.no_description_guest') }}</template>
          <template v-else>{{ t('detail.no_description') }}</template>
        </p>

        <div class="section">
          <h3>{{ t('detail.file_path') }}</h3>
          <p class="muted" style="word-break:break-all">{{ video.path }}</p>
        </div>

        <p v-if="message" :class="ok ? 'chip' : 'error'">{{ message }}</p>

        <form v-if="editing" class="edit-form" @submit.prevent="saveEdit">
          <h3>{{ t('detail.edit_meta') }}</h3>
          <div class="ef-row">
            <label>{{ t('detail.label_title') }}</label>
            <input v-model="form.title" class="input" required />
          </div>
          <div class="ef-row">
            <label>{{ t('detail.label_year') }}</label>
            <input v-model.number="form.year" class="input" type="number" min="0" max="2999" />
          </div>
          <div class="ef-row">
            <label>{{ t('detail.label_desc') }}</label>
            <textarea v-model="form.overview" class="input" rows="5"></textarea>
          </div>
          <div class="ef-row">
            <label>{{ t('detail.label_imdb') }}</label>
            <div class="ef-inline">
              <input v-model="form.imdb_id" class="input" placeholder="tt0103064" />
              <button class="btn" type="button" :disabled="fetchingId" @click="fetchById">
                {{ fetchingId ? '...' : t('detail.by_id') }}
              </button>
            </div>
          </div>
          <div class="ef-row">
            <label>{{ t('detail.label_tmdb') }}</label>
            <input v-model="form.tmdb_id" class="input" :placeholder="t('detail.tmdb_placeholder')" />
          </div>
          <div class="ef-row">
            <label>{{ t('detail.label_cover') }}</label>
            <input v-model="form.poster_url" class="input" placeholder="https://..." />
            <p class="muted small">{{ t('detail.cover_hint') }}</p>
          </div>
          <div class="ef-actions">
            <button class="btn primary" type="submit" :disabled="saving">
              {{ saving ? t('detail.saving') : t('detail.save_btn') }}
            </button>
            <button class="btn" type="button" @click="editing = false">{{ t('detail.cancel') }}</button>
          </div>
        </form>
      </div>
    </template>

    <div v-else class="empty">
      <div class="big">🚫</div>
      <div>{{ t('detail.not_found') }}</div>
    </div>
  </main>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import { mediaUrl, thumbVersion, bumpThumbs } from '@/composables/media.js'
import { useAuthStore } from '@/stores/auth.js'
import { t } from '@/i18n'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const isAdmin = computed(() => !!auth.user?.is_admin)
const loading = ref(true)
const video = ref(null)
const progress = ref({ position: 0, duration: 0 })
const deleting = ref(false)
const refreshing = ref(false)
const saving = ref(false)
const editing = ref(false)
const converting = ref(false)
const convertPct = ref(0)
let convertTimer = null
const message = ref('')
const ok = ref(false)
const metaSource = ref(localStorage.getItem('flycast_meta_source') || 'imdb')
const form = ref({ title: '', year: 0, overview: '', imdb_id: '', tmdb_id: '', poster_url: '' })
const fetchingId = ref(false)

watch(metaSource, (v) => localStorage.setItem('flycast_meta_source', v))

function bust(u) {
  if (!u || !thumbVersion.value || !u.startsWith('/api/')) return u
  return u + (u.includes('?') ? '&' : '?') + 'v=' + thumbVersion.value
}
const posterSrc = computed(() => bust(mediaUrl(video.value?.thumb)))
const backdropSrc = computed(() => bust(mediaUrl(video.value?.backdrop || video.value?.thumb)))

const posterRatio = ref('16 / 10')
function onPosterLoad(e) {
  const img = e.target
  if (img.naturalWidth && img.naturalHeight) {
    posterRatio.value = `${img.naturalWidth} / ${img.naturalHeight}`
  }
}
watch(posterSrc, () => (posterRatio.value = '16 / 10'))

async function load() {
  loading.value = true
  try {
    const res = await axios.get(`/api/videos/${route.params.id}`)
    video.value = res.data
  } catch (e) {
    video.value = null
  } finally {
    loading.value = false
  }
  try {
    const pr = await axios.get(`/api/progress/${route.params.id}`)
    progress.value = pr.data
  } catch (e) {
    progress.value = { position: 0, duration: 0 }
  }
}

const hasProgress = computed(() => {
  const pos = Number(progress.value.position) || 0
  const dur = Number(progress.value.duration) || 0
  if (pos <= 10) return false
  if (dur > 0 && pos >= dur - Math.min(30, dur * 0.05)) return false
  return true
})

function resume() {
  if (!hasProgress.value) return
  router.push({ name: 'player', params: { id: video.value.id }, query: { resume: '1' } })
}

async function deleteVideo() {
  const name = video.value.title || video.value.filename
  if (!confirm(t('detail.delete_confirm', { name }))) return
  deleting.value = true
  try {
    await axios.delete(`/api/videos/${video.value.id}`)
    bumpThumbs()
    const back = window.history.state && window.history.state.back
    if (back) router.back()
    else router.push({ name: 'library' })
  } catch (e) {
    message.value = t('detail.delete_failed')
    ok.value = false
    deleting.value = false
  }
}

async function refreshMetadata() {
  refreshing.value = true
  message.value = ''
  try {
    const title = video.value.title || video.value.filename
    const search = await axios.get('/api/metadata/search', {
      params: { q: title, source: metaSource.value }
    })
    const results = search.data.results || []
    if (!results.length) {
      message.value = t('detail.no_results', { source: metaSource.value })
      ok.value = false
      return
    }
    const r = results[0]
    await axios.post(`/api/metadata/apply/${video.value.id}`, {
      title: r.title || title,
      year: Number(r.year) || 0,
      overview: r.overview || '',
      imdb_id: r.imdb_id || '',
      tmdb_id: r.tmdb_id || '',
      poster_path: r.poster_path || '',
      backdrop_path: r.backdrop_path || '',
      source: r.source || metaSource.value
    })
    message.value = t('detail.updated')
    ok.value = true
    bumpThumbs()
    await load()
  } catch (e) {
    message.value = t('detail.download_failed')
    ok.value = false
  } finally {
    refreshing.value = false
  }
}

function startEdit() {
  form.value = {
    title: video.value.title || video.value.filename || '',
    year: video.value.year || 0,
    overview: video.value.overview || '',
    imdb_id: video.value.imdb_id || '',
    tmdb_id: video.value.tmdb_id || '',
    poster_url: ''
  }
  editing.value = true
  message.value = ''
  ok.value = false
}

async function fetchById() {
  const id = (form.value.imdb_id || '').trim()
  if (!id) {
    message.value = t('detail.enter_imdb')
    ok.value = false
    return
  }
  fetchingId.value = true
  message.value = ''
  try {
    const res = await axios.get('/api/metadata/search', { params: { imdb: id } })
    const m = (res.data.results || [])[0]
    if (!m) {
      message.value = t('detail.not_found_id')
      ok.value = false
      return
    }
    form.value.title = m.title || form.value.title
    form.value.year = m.year || form.value.year
    form.value.overview = m.overview || form.value.overview
    form.value.imdb_id = m.imdb_id || id
    if (m.tmdb_id) form.value.tmdb_id = m.tmdb_id
    if (m.poster_path) form.value.poster_url = m.poster_path
    message.value = t('detail.data_downloaded')
    ok.value = true
  } catch (e) {
    message.value = t('detail.data_failed')
    ok.value = false
  } finally {
    fetchingId.value = false
  }
}

async function saveEdit() {
  if (!form.value.title.trim()) {
    message.value = t('detail.title_required')
    ok.value = false
    return
  }
  saving.value = true
  try {
    await axios.put(`/api/videos/${video.value.id}/meta`, {
      title: form.value.title,
      year: Number(form.value.year) || 0,
      overview: form.value.overview,
      imdb_id: form.value.imdb_id,
      tmdb_id: form.value.tmdb_id,
      poster_url: form.value.poster_url
    })
    message.value = t('detail.saved')
    ok.value = true
    editing.value = false
    bumpThumbs()
    await load()
  } catch (e) {
    message.value = e.response?.data?.err === 'poster_download_failed'
      ? t('detail.cover_failed')
      : t('detail.save_failed')
    ok.value = false
  } finally {
    saving.value = false
  }
}

function fmt(s) {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return h > 0 ? t('detail.duration_hour', { h, m }) : t('detail.duration_min', { m })
}
function size(b) {
  if (!b) return ''
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let n = b
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return n.toFixed(1) + ' ' + units[i]
}
function hideBg(e) {
  e.target.style.display = 'none'
}
function hideImg(e) {
  e.target.style.display = 'none'
}

async function convertVideo() {
  if (converting.value) return
  try {
    await axios.post(`/api/videos/${video.value.id}/convert`)
  } catch (e) {
    if (e.response?.status === 409) {
      message.value = t('detail.convert_busy')
    } else {
      message.value = t('detail.convert_start_failed')
    }
    ok.value = false
    return
  }
  converting.value = true
  convertPct.value = 0
  message.value = t('detail.convert_bg')
  ok.value = true
  startConvertPoll()
}

function startConvertPoll() {
  stopConvertPoll()
  convertTimer = setInterval(pollConvert, 2000)
}
function stopConvertPoll() {
  if (convertTimer) clearInterval(convertTimer)
  convertTimer = null
}
async function pollConvert() {
  try {
    const res = await axios.get('/api/convert/job')
    const job = res.data || {}
    if (Number(job.percent) > 0) convertPct.value = Number(job.percent)
    if (job.running) return
    stopConvertPoll()
    converting.value = false
    if (job.error) {
      message.value = t('detail.convert_failed', { error: job.error })
      ok.value = false
    } else {
      convertPct.value = 100
      message.value = t('detail.convert_done')
      ok.value = true
    }
  } catch (e) { /* keep polling */ }
}

onMounted(async () => {
  await load()
  try {
    const res = await axios.get('/api/convert/job')
    const job = res.data || {}
    if (job.running) {
      converting.value = true
      convertPct.value = Number(job.percent) || 0
      startConvertPoll()
    }
  } catch (e) { /* ignore */ }
})

onBeforeUnmount(stopConvertPoll)
</script>
