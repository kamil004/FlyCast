<template>
  <main class="page">
    <div class="crumbs">
      <span>{{ t('favorites.title') }}</span>
    </div>

    <div v-if="loading" class="loading">{{ t('favorites.loading') }}</div>

    <div v-else-if="visible.length" class="grid">
      <MediaCard v-for="v in visible" :key="v.id" :item="v" />
    </div>

    <div v-else class="empty">
      <div class="big">⭐</div>
      <div>{{ t('favorites.empty') }}</div>
    </div>
  </main>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import axios from 'axios'
import MediaCard from '@/components/MediaCard.vue'
import { useFavoritesStore } from '@/stores/favorites'
import { t } from '@/i18n'

const fav = useFavoritesStore()
const items = ref([])
const loading = ref(true)

const visible = computed(() => items.value.filter((v) => fav.has(v.id)))

async function load() {
  loading.value = true
  await fav.load(true)
  try {
    const res = await axios.get('/api/favorites')
    items.value = res.data.items || []
  } catch (e) {
    items.value = []
  }
  loading.value = false
}

onMounted(load)
</script>
