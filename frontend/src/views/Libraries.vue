<template>
  <main class="page">
    <div class="head-row">
      <div>
        <h2>Biblioteki</h2>
        <p class="muted">Dodawaj katalogi mediów i uruchamiaj skanowanie.</p>
      </div>
      <button class="btn" :disabled="scanning" @click="scanAll">
        {{ scanning ? '⏳ Skanowanie...' : '🔄 Skanuj wszystko' }}
      </button>
    </div>

    <div class="lib-list" v-if="libraries.length">
      <div v-for="lib in libraries" :key="lib.id" class="lib-row">
        <div class="ic">📁</div>
        <div class="info">
          <div class="n">{{ lib.name }}</div>
          <div class="p">{{ lib.path }}</div>
        </div>
        <span class="chip">{{ typeLabel(lib.media_type) }}</span>
        <span class="count">{{ lib.count }} filmów</span>
        <span v-if="!lib.exists" class="chip warn">brak dostępu</span>
        <div class="actions">
          <button class="btn ghost" :disabled="scanning" @click="scanOne(lib)" title="Skanuj">🔄</button>
          <button class="btn ghost" :disabled="clearing === lib.id" @click="clearMetadata(lib)" title="Usuń wszystkie metadane">🧹</button>
          <button class="btn ghost" @click="confirmRemove(lib)" title="Usuń">🗑️</button>
        </div>
      </div>
    </div>
    <div v-else class="empty">
      <div class="big">📚</div>
      <div>Brak skonfigurowanych bibliotek.</div>
    </div>

    <div class="panel-card">
      <h3 style="margin-bottom:14px">Dodaj bibliotekę</h3>
      <div class="form">
        <div class="field">
          <label>Ścieżka</label>
          <div class="path-row">
            <input class="input" v-model="form.path" placeholder="/share/Multimedia/Filmy" />
            <button class="btn" @click="picker = true">📂 Przeglądaj</button>
          </div>
        </div>
        <div class="field">
          <label>Nazwa (opcjonalnie)</label>
          <input class="input" v-model="form.name" placeholder="np. Filmy" />
        </div>
        <div class="field">
          <label>Typ</label>
          <select class="input" v-model="form.media_type">
            <option value="mixed">Mieszane</option>
            <option value="movies">Filmy</option>
            <option value="tv">Seriale</option>
            <option value="home">Nagrania / Domowe</option>
          </select>
        </div>
        <div>
          <button class="btn primary" :disabled="adding || !form.path" @click="add">
            {{ adding ? 'Dodaję...' : '＋ Dodaj bibliotekę' }}
          </button>
        </div>
        <p v-if="error" class="error">{{ error }}</p>
        <p v-if="notice" class="chip">{{ notice }}</p>
      </div>
    </div>

    <FolderPicker :open="picker" :start="form.path" @close="picker = false" @select="onPick" />
  </main>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import FolderPicker from '@/components/FolderPicker.vue'

const route = useRoute()
const router = useRouter()

const libraries = ref([])
const picker = ref(false)
const adding = ref(false)
const scanning = ref(false)
const clearing = ref(null)
const error = ref('')
const notice = ref('')
const form = reactive({ path: '', name: '', media_type: 'mixed' })

async function load() {
  const res = await axios.get('/api/libraries')
  libraries.value = res.data.items || []
}

function onPick(path) {
  form.path = path
  if (!form.name) {
    const base = path.replace(/\/+$/, '').split('/').pop()
    if (base) form.name = base
  }
}

async function add() {
  error.value = ''
  notice.value = ''
  adding.value = true
  try {
    await axios.post('/api/libraries', { ...form })
    form.path = ''
    form.name = ''
    form.media_type = 'mixed'
    notice.value = 'Dodano bibliotekę. Skanowanie w toku...'
    await delay(1500)
    await load()
  } catch (e) {
    error.value = e.response?.data?.err === 'already_exists'
      ? 'Ta biblioteka już istnieje.'
      : 'Nie udało się dodać biblioteki.'
  } finally {
    adding.value = false
  }
}

async function confirmRemove(lib) {
  if (!confirm(`Usunąć bibliotekę „${lib.name}” i wszystkie jej filmy z bazy?`)) return
  await axios.delete('/api/libraries/' + lib.id)
  await load()
}

async function scanAll() {
  scanning.value = true
  notice.value = 'Skanowanie w toku...'
  await axios.post('/api/libraries/scan')
  await delay(2500)
  await load()
  scanning.value = false
}

async function scanOne(lib) {
  scanning.value = true
  await axios.post(`/api/libraries/${lib.id}/scan`)
  await delay(2000)
  await load()
  scanning.value = false
}

async function clearMetadata(lib) {
  if (!confirm(`Usunąć wszystkie metadane (tytuły, opisy, okładki) dla biblioteki "${lib.name}"?`)) {
    return
  }
  clearing.value = lib.id
  try {
    const res = await axios.post(`/api/libraries/${lib.id}/metadata/clear`)
    notice.value = `Wyczyszczono metadane (${res.data.count || 0} pozycji).`
    await load()
  } catch (e) {
    notice.value = 'Nie udało się wyczyścić metadanych.'
  } finally {
    clearing.value = null
  }
}

function typeLabel(t) {
  return { mixed: 'Mieszane', movies: 'Filmy', tv: 'Seriale', home: 'Domowe' }[t] || t
}
function delay(ms) {
  return new Promise((r) => setTimeout(r, ms))
}

onMounted(async () => {
  await load()
  if (route.query.add) {
    form.path = String(route.query.add)
    if (!form.name) {
      const base = form.path.replace(/\/+$/, '').split('/').pop()
      if (base) form.name = base
    }
    router.replace({ name: 'libraries' })
  }
})
</script>

<style scoped>
.head-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 22px;
}
.head-row h2 {
  font-size: 22px;
  margin-bottom: 4px;
}
.lib-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 26px;
}
.lib-row {
  display: flex;
  align-items: center;
  gap: 14px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 12px 16px;
}
.lib-row .ic {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  background: rgba(79, 140, 255, 0.14);
  color: var(--accent);
}
.lib-row .info {
  flex: 1;
  min-width: 0;
}
.lib-row .n {
  font-weight: 600;
}
.lib-row .p {
  font-size: 12px;
  color: var(--dim-2);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.lib-row .count {
  font-size: 12px;
  color: var(--dim);
}
.chip.warn {
  color: #ffb020;
  border-color: #ffb020;
}
.lib-row .actions {
  display: flex;
  gap: 4px;
}
.panel-card {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 22px;
  max-width: 640px;
}
.form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.path-row {
  display: flex;
  gap: 8px;
}
.path-row .input {
  flex: 1;
}
</style>
