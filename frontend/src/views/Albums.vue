<template>
  <main class="page">
    <section class="panel-card">
      <h2 style="margin-bottom:14px">Albums</h2>
      <div class="row">
        <input class="input" v-model="name" placeholder="New album name" @keyup.enter="create" />
        <button class="btn primary" @click="create">＋ Create</button>
      </div>
    </section>

    <div class="lib-grid" v-if="albums.length">
      <div v-for="a in albums" :key="a.id" class="lib-card clickable" @click="open(a)">
        <div class="ic">🗂️</div>
        <div class="meta">
          <div class="n">{{ a.name }}</div>
          <div class="c">{{ a.count || 0 }} items</div>
        </div>
        <button class="add-lib del" title="Delete album" @click.stop="remove(a)">🗑️</button>
      </div>
    </div>

    <div v-else class="empty">
      <div class="big">🗂️</div>
      <div>No albums yet. Create one above.</div>
    </div>
  </main>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'

const API = '/api'
const router = useRouter()
const albums = ref([])
const name = ref('')

async function load() {
  const res = await axios.get(API + '/albums')
  albums.value = res.data.albums || []
}
async function create() {
  if (!name.value.trim()) return
  await axios.post(API + '/albums', { name: name.value.trim() })
  name.value = ''
  load()
}
function open(a) {
  router.push({ name: 'album-view', params: { id: a.id } })
}
async function remove(a) {
  if (!confirm(`Delete album "${a.name}"?`)) return
  try {
    await axios.delete(API + '/albums/' + a.id)
    await load()
  } catch (e) {
    alert('Failed to delete album')
  }
}
onMounted(load)
</script>

<style scoped>
.panel-card {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 22px;
  margin-bottom: 26px;
}
.row {
  display: flex;
  gap: 10px;
  max-width: 560px;
}
</style>
