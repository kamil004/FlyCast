<template>
  <section class="rail" v-if="items.length">
    <div class="rail-head">
      <h2>{{ title }}</h2>
      <router-link v-if="more" :to="more">Zobacz wszystkie</router-link>
    </div>
    <div class="rail-track">
      <MediaCard v-for="it in items" :key="it.id" :item="it" />
    </div>
  </section>
</template>

<script setup>
import MediaCard from '@/components/MediaCard.vue'

defineProps({
  title: { type: String, required: true },
  items: { type: Array, default: () => [] },
  more: { type: [String, Object], default: null }
})
</script>
