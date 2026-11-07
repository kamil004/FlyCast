<template>
  <router-link class="mcard" :to="{ name: 'detail', params: { id: item.id } }">
    <div class="thumb">
      <img v-if="thumbSrc" :src="thumbSrc" :alt="item.title" @error="failed = true" v-show="!failed" />
      <div v-if="!thumbSrc || failed" class="ph">🎞️</div>
      <button class="play" title="Play" @click.prevent.stop="play">▶</button>
      <button
        class="fav"
        :class="{ active: isFav }"
        :title="isFav ? 'Remove from favorites' : 'Add to favorites'"
        @click.prevent.stop="toggleFav"
      >{{ isFav ? '★' : '☆' }}</button>
      <span v-if="duration" class="dur">{{ duration }}</span>
      <div v-if="progress" class="bar"><span :style="{ width: pct }"></span></div>
    </div>
    <div class="body">
      <div class="title">{{ item.title || item.filename }}</div>
      <div class="sub">{{ sub }}</div>
    </div>
  </router-link>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { mediaUrl, thumbVersion } from '@/composables/media.js'
import { useFavoritesStore } from '@/stores/favorites'

const props = defineProps({
  item: { type: Object, required: true },
  list: { type: Boolean, default: false }
})
const failed = ref(false)
const router = useRouter()
const fav = useFavoritesStore()

const isFav = computed(() => fav.has(props.item.id))

function toggleFav() {
  fav.toggle(props.item.id)
}

function play() {
  router.push({ name: 'player', params: { id: props.item.id } })
}

const thumbSrc = computed(() => {
  if (!props.item.thumb) return ''
  let u = mediaUrl(props.item.thumb)
  if (thumbVersion.value && props.item.thumb.startsWith('/api/thumb/')) {
    u += (u.includes('?') ? '&' : '?') + 'v=' + thumbVersion.value
  }
  return u
})

const progress = computed(() => {
  const p = props.item.progress
  return typeof p === 'number' && p > 0 && p < 1
})
const pct = computed(() => Math.min(100, Math.round((props.item.progress || 0) * 100)) + '%')

const duration = computed(() => {
  const s = props.item.duration
  if (!s || isNaN(s)) return ''
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = Math.floor(s % 60).toString().padStart(2, '0')
  return h > 0 ? `${h}:${m.toString().padStart(2, '0')}:${sec}` : `${m}:${sec}`
})

function fmtDate(v) {
  const d = String(v).slice(0, 10).split('-')
  if (d.length !== 3) return ''
  return `${d[2]}.${d[1]}.${d[0]}`
}

function fmtSize(bytes) {
  const n = Number(bytes) || 0
  if (n <= 0) return ''
  const mb = n / 1048576
  return mb >= 1024 ? (mb / 1024).toFixed(2) + ' GB' : mb.toFixed(0) + ' MB'
}

const sub = computed(() => {
  const parts = []
  if (props.item.year) parts.push(props.item.year)
  if (props.item.progress) parts.push('Continue')
  if (props.list) {
    const d = props.item.created_at ? fmtDate(props.item.created_at) : ''
    if (d) parts.push(d)
    const s = fmtSize(props.item.size)
    if (s) parts.push(s)
  }
  return parts.join(' • ')
})
</script>
