<template>
  <div v-if="open" class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal">
      <div class="modal-head">
        <h3>Select folder</h3>
        <button class="btn ghost" @click="$emit('close')">✕</button>
      </div>

      <div class="crumbs" style="margin-bottom:12px">
        <a href="#" @click.prevent="goTo('')">Roots</a>
        <template v-for="(seg, i) in segments" :key="i">
          <span class="sep">/</span>
          <a href="#" @click.prevent="goTo(seg.path)">{{ seg.name }}</a>
        </template>
      </div>

      <div class="picker-list">
        <div v-if="loading" class="loading">Loading...</div>
        <template v-else>
          <div v-if="parent !== null" class="picker-row" @click="goTo(parent)">
            <span class="ico">↩</span>
            <span>..</span>
          </div>
          <div v-if="!dirs.length && !parent" class="muted" style="padding:14px">
            No available folders (allowed: /share, /volume1-4, /mnt).
          </div>
          <div
            v-for="d in dirs"
            :key="d.path"
            class="picker-row"
            @click="goTo(d.path)"
          >
            <span class="ico">📁</span>
            <span>{{ d.name }}</span>
          </div>
        </template>
      </div>

      <div class="modal-foot">
        <span class="muted" style="overflow:hidden;text-overflow:ellipsis">{{ current || '—' }}</span>
        <div class="spacer"></div>
        <button class="btn" @click="$emit('close')">Cancel</button>
        <button class="btn primary" :disabled="!current" @click="choose">Select this folder</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import axios from 'axios'

const props = defineProps({
  open: { type: Boolean, default: false },
  start: { type: String, default: '' }
})
const emit = defineEmits(['close', 'select'])

const loading = ref(false)
const current = ref('')
const parent = ref(null)
const dirs = ref([])

const segments = computed(() => {
  if (!current.value) return []
  const parts = current.value.split('/').filter(Boolean)
  const out = []
  let acc = ''
  for (const p of parts) {
    acc += '/' + p
    out.push({ name: p, path: acc })
  }
  return out
})

async function goTo(path) {
  loading.value = true
  try {
    const res = await axios.get('/api/fs/browse', { params: { path } })
    current.value = res.data.path || ''
    const roots = res.data.roots
    parent.value = roots ? null : (res.data.parent || (current.value ? '/' : null))
    dirs.value = res.data.dirs || []
  } catch (e) {
    dirs.value = []
    parent.value = ''
  } finally {
    loading.value = false
  }
}

function choose() {
  emit('select', current.value)
  emit('close')
}

watch(
  () => props.open,
  (o) => {
    if (o) goTo(props.start || '')
  }
)
</script>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(3px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 24px;
}
.modal {
  width: 100%;
  max-width: 620px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-2);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.6);
  overflow: hidden;
}
.modal-head {
  display: flex;
  align-items: center;
  padding: 16px 18px;
  border-bottom: 1px solid var(--border);
}
.modal-head h3 {
  flex: 1;
  font-size: 16px;
}
.modal > .crumbs {
  padding: 12px 18px 0;
}
.picker-list {
  overflow-y: auto;
  min-height: 220px;
  padding: 6px 10px;
}
.picker-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
}
.picker-row:hover {
  background: var(--card-hover);
}
.picker-row .ico {
  width: 18px;
  text-align: center;
}
.modal-foot {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 18px;
  border-top: 1px solid var(--border);
}
.modal-foot .muted {
  max-width: 45%;
  white-space: nowrap;
  font-size: 12px;
}
</style>
