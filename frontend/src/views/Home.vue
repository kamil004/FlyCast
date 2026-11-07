<template>
  <main class="page">
    <div v-if="loading" class="loading">Loading...</div>

    <template v-else>
      <section class="rail" v-if="libraries.length">
        <div class="rail-head">
          <h2>Libraries</h2>
        </div>
        <div class="lib-grid">
          <router-link
            v-for="lib in libraries"
            :key="lib.id"
            class="lib-card"
            :to="{ name: 'library', query: { folder: lib.id } }"
          >
            <LibThumb :thumbs="lib.thumbs" />
            <div class="meta">
              <div class="n">{{ lib.name }}</div>
              <div class="c">{{ lib.count }} {{ plural(lib.count) }}</div>
              <div class="c" v-if="fmtSize(lib.size)">{{ fmtSize(lib.size) }}</div>
            </div>
          </router-link>
        </div>
      </section>

      <MediaRow title="Continue Watching" :items="continueItems" />

      <div v-if="!libraries.length && !continueItems.length" class="empty">
        <div class="big">🎬</div>
        <div>Library is empty.</div>
        <div class="muted">
          Add media directories and start scanning (set <code>FLYCAST_LIBRARY</code>).
        </div>
      </div>
    </template>
  </main>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import axios from 'axios'
import MediaRow from '@/components/MediaRow.vue'
import LibThumb from '@/components/LibThumb.vue'

const loading = ref(true)
const continueItems = ref([])
const libraries = ref([])

async function load() {
  loading.value = true
  try {
    const [c, l] = await Promise.all([
      axios.get('/api/library/continue', { params: { limit: 24 } }),
      axios.get('/api/library/libraries')
    ])
    continueItems.value = c.data.items || []
    libraries.value = l.data.items || []
  } finally {
    loading.value = false
  }
}

function plural(n) {
  if (n === 1) return 'film'
  return 'films'

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

onMounted(load)
</script>
