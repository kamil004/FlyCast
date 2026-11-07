<template>
  <main class="page" v-if="auth.initialized">
    <div class="panel">
      <h2>{{ t('settings.title') }}</h2>
      <p class="muted">{{ t('settings.subtitle') }}</p>
      <div class="kv">
        <span>{{ t('settings.version') }}</span>
        <span>0.1.0</span>
        <span>{{ t('settings.http_port') }}</span>
        <span>8100</span>
        <span>Streaming</span>
        <span>Direct Play + remux + transkodowanie</span>
      </div>
    </div>

    <div class="panel">
      <h3>{{ t('settings.users') }}</h3>
      <div class="users">
        <div v-for="u in users" :key="u.id" class="user">
          <div class="wholine">
            <span class="avatar sm">{{ (u.display_name || u.username).slice(0, 2).toUpperCase() }}</span>
            <span>{{ u.username }}</span>
          </div>
          <div class="row-right">
            <span class="tag" :class="acctClass(u)">{{ acctLabel(u) }}</span>
            <span class="tag scope" v-if="!u.is_admin">
              {{ u.restricted ? `${(u.library_ids || []).length} ${t('settings.libs_count')}` : t('settings.all_libs_short') }}
            </span>
            <span class="tag you" v-if="u.id === me?.id">{{ t('settings.is_you') }}</span>
            <button
              v-if="isAdmin"
              class="icon-btn"
              :title="t('settings.edit_user')"
              @click="openEdit(u)"
            >✏️</button>
            <button
              v-if="isAdmin && u.id !== me?.id"
              class="icon-btn danger"
              :title="t('settings.delete_user')"
              @click="removeUser(u)"
            >🗑️</button>
          </div>
        </div>
      </div>

      <template v-if="isAdmin">
        <h3 style="margin-top:8px">{{ t('settings.add_user') }}</h3>
        <form class="create-form" @submit.prevent="createUser">
          <input class="input" v-model="form.username" :placeholder="t('settings.username')" autocomplete="off" />
          <input class="input" v-model="form.display" :placeholder="t('settings.display_optional')" autocomplete="off" />
          <input
            class="input"
            v-model="form.password"
            type="password"
            :placeholder="form.type === 'guest' ? t('settings.password_guest') : t('settings.password')"
            :disabled="form.type === 'guest'"
            autocomplete="new-password"
          />
          <label class="field">
            <span class="lbl">{{ t('settings.account_type') }}</span>
            <select class="select" v-model="form.type">
              <option value="user">{{ t('settings.type_user') }}</option>
              <option value="admin">{{ t('settings.type_admin') }}</option>
              <option value="guest">{{ t('settings.type_guest') }}</option>
            </select>
          </label>

          <div class="access" v-if="form.type !== 'admin'">
            <span class="lbl">{{ t('settings.library_access') }}</span>
            <label class="check">
              <input type="radio" value="all" v-model="form.scope" />
              <span>{{ t('settings.all_libs') }}</span>
            </label>
            <label class="check">
              <input type="radio" value="selected" v-model="form.scope" />
              <span>{{ t('settings.selected_libs') }}</span>
            </label>
            <div class="lib-list" v-if="form.scope === 'selected'">
              <label v-for="l in libraries" :key="l.id" class="check">
                <input type="checkbox" :value="l.id" v-model="form.library_ids" />
                <span>{{ l.name || l.path }}</span>
              </label>
              <p v-if="!libraries.length" class="muted">{{ t('settings.no_libs') }}</p>
            </div>
          </div>

          <button class="btn primary" :disabled="creating" type="submit">
            {{ creating ? t('settings.creating') : t('settings.create_btn') }}
          </button>
        </form>
        <p v-if="message" :class="ok ? 'chip ok' : 'error'">{{ message }}</p>
      </template>
      <p v-else class="muted">{{ t('settings.admin_only') }}</p>
    </div>

    <div v-if="editing" class="modal-backdrop" @click.self="closeEdit">
      <div class="modal panel">
        <div class="modal-head">
          <h3>{{ t('settings.edit_header', { username: editing.username }) }}</h3>
          <button class="icon-btn" @click="closeEdit">✕</button>
        </div>
        <label class="field">
          <span class="lbl">{{ t('settings.display_name') }}</span>
          <input class="input" v-model="editForm.display_name" />
        </label>
        <label class="field">
          <span class="lbl">{{ t('settings.new_password') }}</span>
          <input
            class="input"
            v-model="editForm.password"
            type="password"
            :disabled="editForm.type === 'guest'"
            :placeholder="editForm.type === 'guest' ? t('settings.password_empty') : ''"
            autocomplete="new-password"
          />
        </label>
        <label class="field">
          <span class="lbl">{{ t('settings.account_type') }}</span>
          <select class="select" v-model="editForm.type">
            <option value="user">{{ t('settings.type_user') }}</option>
            <option value="admin">{{ t('settings.type_admin') }}</option>
            <option value="guest">{{ t('settings.type_guest') }}</option>
          </select>
        </label>

        <div class="access" v-if="editForm.type !== 'admin'">
          <span class="lbl">{{ t('settings.library_access') }}</span>
          <label class="check">
            <input type="radio" value="all" v-model="editForm.scope" />
            <span>{{ t('settings.all_libs') }}</span>
          </label>
          <label class="check">
            <input type="radio" value="selected" v-model="editForm.scope" />
            <span>{{ t('settings.selected_libs') }}</span>
          </label>
          <div class="lib-list" v-if="editForm.scope === 'selected'">
            <label v-for="l in libraries" :key="l.id" class="check">
              <input type="checkbox" :value="l.id" v-model="editForm.library_ids" />
              <span>{{ l.name || l.path }}</span>
            </label>
            <p v-if="!libraries.length" class="muted">{{ t('settings.no_libs') }}</p>
          </div>
        </div>

        <p v-if="editMsg" :class="editOk ? 'chip ok' : 'error'">{{ editMsg }}</p>
        <div class="modal-actions">
          <button class="btn" @click="closeEdit">{{ t('detail.cancel') }}</button>
          <button class="btn primary" :disabled="saving" @click="saveEdit">
            {{ saving ? t('settings.saving') : t('settings.save_btn') }}
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
import { t } from '@/i18n'

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
  return u.is_admin ? t('settings.type_admin') : u.is_guest ? t('settings.type_guest') : t('settings.type_user')
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
    message.value = form.type === 'guest' ? t('settings.create_prompt_guest') : t('settings.create_prompt')
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
    message.value = t('settings.created', { username: form.username.trim() })
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
    message.value = err === 'username_exists'
      ? t('settings.username_exists')
      : t('settings.create_failed')
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
    editMsg.value = t('settings.saved')
    await load()
    setTimeout(closeEdit, 600)
  } catch (e) {
    const err = e?.response?.data?.err
    editMsg.value =
      err === 'cannot_demote_self'
        ? t('settings.no_demote_self')
        : err === 'last_admin'
          ? t('settings.no_last_admin')
          : t('settings.save_failed')
    editOk.value = false
  } finally {
    saving.value = false
  }
}

async function removeUser(u) {
  if (!confirm(t('settings.confirm_delete_user', { username: u.username }))) return
  message.value = ''
  try {
    await axios.delete(API + '/users/' + u.id)
    await load()
  } catch (e) {
    const err = e?.response?.data?.err
    message.value =
      err === 'cannot_delete_self'
        ? t('settings.no_delete_self')
        : err === 'last_admin'
          ? t('settings.no_last_admin_delete')
          : t('settings.delete_failed')
    ok.value = false
  }
}

onMounted(() => {
  console.log('Settings mounted, auth:', auth.user?.username, 'isAdmin:', auth.isAdmin, 'initialized:', auth.initialized)
  load().then(() => console.log('Users loaded:', users.value.length)).catch(e => console.error('Load users failed:', e))
  loadLibraries().then(() => console.log('Libraries loaded:', libraries.value.length)).catch(e => console.error('Load libs failed:', e))
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
