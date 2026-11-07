<template>
  <main class="page">
    <div class="panel">
      <h2>Ustawienia</h2>
      <p class="muted">Podstawowe informacje o FlyCast</p>
      <div class="kv">
        <span>Wersja</span>
        <span>0.1.0</span>
        <span>Port HTTP</span>
        <span>8100</span>
        <span>Streaming</span>
        <span>Direct Play + remux + transkodowanie</span>
      </div>
    </div>

    <div class="panel">
      <h3>Użytkownicy</h3>
      <div class="users">
        <div v-for="u in users" :key="u.id" class="user">
          <div class="wholine">
            <span class="avatar sm">{{ (u.display_name || u.username).slice(0, 2).toUpperCase() }}</span>
            <span>{{ u.username }}</span>
          </div>
          <div class="row-right">
            <span class="tag" :class="acctClass(u)">{{ acctLabel(u) }}</span>
            <span class="tag scope" v-if="!u.is_admin">
              {{ u.restricted ? `${(u.library_ids || []).length} bibl.` : 'wszystkie bibl.' }}
            </span>
            <span class="tag you" v-if="u.id === me?.id">to Ty</span>
            <button
              v-if="isAdmin"
              class="icon-btn"
              title="Edytuj użytkownika"
              @click="openEdit(u)"
            >✏️</button>
            <button
              v-if="isAdmin && u.id !== me?.id"
              class="icon-btn danger"
              title="Usuń użytkownika"
              @click="removeUser(u)"
            >🗑️</button>
          </div>
        </div>
      </div>

      <template v-if="isAdmin">
        <h3 style="margin-top:8px">Dodaj użytkownika</h3>
        <form class="create-form" @submit.prevent="createUser">
          <input class="input" v-model="form.username" placeholder="Nazwa użytkownika" autocomplete="off" />
          <input class="input" v-model="form.display" placeholder="Nazwa wyświetlana (opcjonalnie)" autocomplete="off" />
          <input
            class="input"
            v-model="form.password"
            type="password"
            :placeholder="form.type === 'guest' ? 'Hasło (niepotrzebne dla gościa)' : 'Hasło'"
            :disabled="form.type === 'guest'"
            autocomplete="new-password"
          />
          <label class="field">
            <span class="lbl">Typ konta</span>
            <select class="select" v-model="form.type">
              <option value="user">Użytkownik</option>
              <option value="admin">Administrator</option>
              <option value="guest">Gość (tylko oglądanie)</option>
            </select>
          </label>

          <div class="access" v-if="form.type !== 'admin'">
            <span class="lbl">Dostęp do bibliotek</span>
            <label class="check">
              <input type="radio" value="all" v-model="form.scope" />
              <span>Wszystkie biblioteki</span>
            </label>
            <label class="check">
              <input type="radio" value="selected" v-model="form.scope" />
              <span>Wybrane biblioteki</span>
            </label>
            <div class="lib-list" v-if="form.scope === 'selected'">
              <label v-for="l in libraries" :key="l.id" class="check">
                <input type="checkbox" :value="l.id" v-model="form.library_ids" />
                <span>{{ l.name || l.path }}</span>
              </label>
              <p v-if="!libraries.length" class="muted">Brak bibliotek.</p>
            </div>
          </div>

          <button class="btn primary" :disabled="creating" type="submit">
            {{ creating ? '⏳ Tworzę...' : '＋ Utwórz użytkownika' }}
          </button>
        </form>
        <p v-if="message" :class="ok ? 'chip ok' : 'error'">{{ message }}</p>
      </template>
      <p v-else class="muted">Tylko administrator może zarządzać użytkownikami.</p>
    </div>

    <div v-if="editing" class="modal-backdrop" @click.self="closeEdit">
      <div class="modal panel">
        <div class="modal-head">
          <h3>Edytuj: {{ editing.username }}</h3>
          <button class="icon-btn" @click="closeEdit">✕</button>
        </div>
        <label class="field">
          <span class="lbl">Nazwa wyświetlana</span>
          <input class="input" v-model="editForm.display_name" />
        </label>
        <label class="field">
          <span class="lbl">Nowe hasło (pozostaw puste, aby nie zmieniać)</span>
          <input
            class="input"
            v-model="editForm.password"
            type="password"
            :disabled="editForm.type === 'guest'"
            :placeholder="editForm.type === 'guest' ? 'Niepotrzebne dla gościa' : ''"
            autocomplete="new-password"
          />
        </label>
        <label class="field">
          <span class="lbl">Typ konta</span>
          <select class="select" v-model="editForm.type">
            <option value="user">Użytkownik</option>
            <option value="admin">Administrator</option>
            <option value="guest">Gość (tylko oglądanie)</option>
          </select>
        </label>

        <div class="access" v-if="editForm.type !== 'admin'">
          <span class="lbl">Dostęp do bibliotek</span>
          <label class="check">
            <input type="radio" value="all" v-model="editForm.scope" />
            <span>Wszystkie biblioteki</span>
          </label>
          <label class="check">
            <input type="radio" value="selected" v-model="editForm.scope" />
            <span>Wybrane biblioteki</span>
          </label>
          <div class="lib-list" v-if="editForm.scope === 'selected'">
            <label v-for="l in libraries" :key="l.id" class="check">
              <input type="checkbox" :value="l.id" v-model="editForm.library_ids" />
              <span>{{ l.name || l.path }}</span>
            </label>
            <p v-if="!libraries.length" class="muted">Brak bibliotek.</p>
          </div>
        </div>

        <p v-if="editMsg" :class="editOk ? 'chip ok' : 'error'">{{ editMsg }}</p>
        <div class="modal-actions">
          <button class="btn" @click="closeEdit">Anuluj</button>
          <button class="btn primary" :disabled="saving" @click="saveEdit">
            {{ saving ? '⏳ Zapisuję...' : 'Zapisz' }}
          </button>
        </div>
      </div>
    </div>
  </main>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import axios from 'axios'
import { useAuthStore } from '@/stores/auth.js'

const API = '/api'
const auth = useAuthStore()
const users = ref([])
const libraries = ref([])
const creating = ref(false)
const message = ref('')
const ok = ref(false)

const editing = ref(null)
const editForm = reactive({ display_name: '', password: '', type: 'user', scope: 'all', library_ids: [] })
const saving = ref(false)
const editMsg = ref('')
const editOk = ref(false)

const me = computed(() => auth.user)
const isAdmin = computed(() => !!auth.user?.is_admin)

const form = reactive({
  username: '',
  password: '',
  display: '',
  type: 'user',
  scope: 'all',
  library_ids: []
})

function acctLabel(u) {
  return u.is_admin ? 'admin' : u.is_guest ? 'gość' : 'użytkownik'
}
function acctClass(u) {
  return u.is_admin ? 'admin' : u.is_guest ? 'guest' : 'user'
}

async function load() {
  try {
    const res = await axios.get(API + '/users')
    users.value = res.data.users || []
  } catch (e) {
    users.value = []
  }
}

async function loadLibraries() {
  if (!isAdmin.value) return
  try {
    const res = await axios.get(API + '/libraries')
    libraries.value = res.data.items || []
  } catch (e) {
    libraries.value = []
  }
}

function scopeToRestricted(scope) {
  return scope === 'selected'
}

async function createUser() {
  message.value = ''
  if (!form.username.trim() || (form.type !== 'guest' && !form.password)) {
    message.value = form.type === 'guest' ? 'Podaj nazwę użytkownika.' : 'Podaj nazwę użytkownika i hasło.'
    ok.value = false
    return
  }
  creating.value = true
  try {
    await axios.post(API + '/users', {
      username: form.username.trim(),
      password: form.password,
      display_name: form.display.trim(),
      is_admin: form.type === 'admin',
      is_guest: form.type === 'guest',
      restricted: form.type !== 'admin' && scopeToRestricted(form.scope),
      library_ids: form.scope === 'selected' ? form.library_ids : []
    })
    message.value = `Utworzono użytkownika „${form.username.trim()}”.`
    ok.value = true
    form.username = ''
    form.password = ''
    form.display = ''
    form.type = 'user'
    form.scope = 'all'
    form.library_ids = []
    await load()
  } catch (e) {
    const err = e?.response?.data?.err
    message.value = err === 'username_exists' ? 'Taka nazwa użytkownika już istnieje.' : 'Nie udało się utworzyć użytkownika.'
    ok.value = false
  } finally {
    creating.value = false
  }
}

function openEdit(u) {
  editing.value = u
  editForm.display_name = u.display_name || ''
  editForm.password = ''
  editForm.type = u.is_admin ? 'admin' : u.is_guest ? 'guest' : 'user'
  editForm.scope = u.restricted ? 'selected' : 'all'
  editForm.library_ids = [...(u.library_ids || [])]
  editMsg.value = ''
  editOk.value = false
}

function closeEdit() {
  editing.value = null
}

async function saveEdit() {
  if (!editing.value) return
  saving.value = true
  editMsg.value = ''
  try {
    await axios.put(API + '/users/' + editing.value.id, {
      display_name: editForm.display_name.trim(),
      password: editForm.password,
      is_admin: editForm.type === 'admin',
      is_guest: editForm.type === 'guest',
      restricted: editForm.type !== 'admin' && scopeToRestricted(editForm.scope),
      library_ids: editForm.scope === 'selected' ? editForm.library_ids : []
    })
    editOk.value = true
    editMsg.value = 'Zapisano zmiany.'
    await load()
    setTimeout(closeEdit, 600)
  } catch (e) {
    const err = e?.response?.data?.err
    editMsg.value =
      err === 'cannot_demote_self'
        ? 'Nie można odebrać sobie uprawnień administratora.'
        : err === 'last_admin'
          ? 'Nie można odebrać uprawnień ostatniemu administratorowi.'
          : 'Nie udało się zapisać zmian.'
    editOk.value = false
  } finally {
    saving.value = false
  }
}

async function removeUser(u) {
  if (!confirm(`Usunąć użytkownika „${u.username}”?`)) return
  message.value = ''
  try {
    await axios.delete(API + '/users/' + u.id)
    await load()
  } catch (e) {
    const err = e?.response?.data?.err
    message.value =
      err === 'cannot_delete_self'
        ? 'Nie można usunąć własnego konta.'
        : err === 'last_admin'
          ? 'Nie można usunąć ostatniego administratora.'
          : 'Nie udało się usunąć użytkownika.'
    ok.value = false
  }
}

onMounted(() => {
  load()
  loadLibraries()
})
</script>

<style scoped>
.panel {
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 22px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: var(--panel);
  margin-bottom: 22px;
  max-width: 720px;
}
.kv {
  display: grid;
  grid-template-columns: 200px 1fr;
  gap: 8px;
  color: var(--dim);
}
.kv span:nth-child(odd) {
  color: var(--dim-2);
}
.users {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.user {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 8px 12px;
  background: var(--bg-2);
}
.wholine {
  display: flex;
  align-items: center;
  gap: 10px;
}
.row-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.avatar.sm {
  width: 28px;
  height: 28px;
  font-size: 12px;
}
.tag {
  font-size: 11px;
  color: var(--accent);
  border: 1px solid var(--accent);
  border-radius: 20px;
  padding: 2px 10px;
}
.tag.admin {
  color: var(--accent);
  border-color: var(--accent);
}
.tag.user {
  color: var(--dim);
  border-color: var(--border);
}
.tag.guest {
  color: #e0b357;
  border-color: #e0b357;
}
.tag.you {
  color: var(--dim);
  border-color: var(--border);
}
.tag.scope {
  color: var(--dim);
  border-color: var(--border);
}
.icon-btn {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 4px 8px;
  cursor: pointer;
  color: var(--dim);
}
.icon-btn.danger:hover {
  border-color: #d64545;
  color: #ff8080;
}
.create-form {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.field .lbl {
  font-size: 12px;
  color: var(--dim-2);
}
.access {
  grid-column: 1 / -1;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.access .lbl {
  font-size: 12px;
  color: var(--dim-2);
}
.lib-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg-2);
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--dim);
}
.create-form .btn {
  grid-column: 1 / -1;
  justify-self: start;
}
.error {
  color: #ff8080;
  font-size: 13px;
}
.chip.ok {
  color: #7ee0a1;
}
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 50;
  padding: 20px;
}
.modal {
  width: 100%;
  max-width: 480px;
  margin: 0;
  max-height: 90vh;
  overflow-y: auto;
}
.modal-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
