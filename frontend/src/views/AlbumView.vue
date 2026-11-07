<template>
  <main class="page">
    <div class="crumbs">
      <router-link to="/albums">Albumy</router-link>
      <span class="sep">/</span>
      <span>{{ album.name }}</span>
      <div class="spacer"></div>
      <button class="btn ghost danger" @click="removeAlbum">🗑️ Usuń album</button>
    </div>

    <div class="grid" v-if="album.items && album.items.length">
      <div v-for="item in album.items" :key="item.id" class="album-item">
        <MediaCard :item="item" />
        <button class="item-del" title="Usuń z albumu" @click.stop="removeItem(item)">✕</button>
      </div>
    </div>

    <div v-else class="empty">
      <div class="big">🎞️</div>
      <div>Ten album jest pusty.</div>
    </div>
  </main>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import MediaCard from '@/components/MediaCard.vue'

const route = useRoute()
const router = useRouter()
const API = '/api'
const album = ref({ items: [] })

async function load() {
  const res = await axios.get(API + '/albums/' + route.params.id)
  album.value = res.data
}
async function removeAlbum() {
  if (!confirm(`Usunąć album „${album.value.name}”?`)) return
  try {
    await axios.delete(API + '/albums/' + route.params.id)
    router.push({ name: 'albums' })
  } catch (e) {
    alert('Nie udało się usunąć albumu')
  }
}
async function removeItem(item) {
  try {
    await axios.post(API + '/albums/' + route.params.id + '/remove', { video_id: item.id })
    await load()
  } catch (e) {
    alert('Nie udało się usunąć pozycji')
  }
}
onMounted(load)
</script>

<style scoped>
.album-item {
  position: relative;
}
.album-item .item-del {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 28px;
  height: 28px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: rgba(0, 0, 0, 0.65);
  color: #fff;
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.15s, background 0.15s;
}
.album-item:hover .item-del {
  opacity: 1;
}
.album-item .item-del:hover {
  background: #d64545;
  border-color: #d64545;
}
.btn.danger {
  color: #ff8080;
  border-color: rgba(255, 128, 128, 0.4);
}
</style>
