<template>
  <div class="lib-thumb" :class="{ poster }">
    <div v-if="images.length" class="mosaic" :class="'n' + images.length">
      <img
        v-for="(t, i) in images"
        :key="i"
        :src="imageSrc(t)"
        alt=""
        @error="onErr(i)"
      />
    </div>
    <div v-else class="ic">📁</div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { mediaUrl, thumbVersion } from '@/composables/media.js'

const props = defineProps({
  thumbs: { type: Array, default: () => [] },
  // portrait cover (9:16) used for movie/series folders
  poster: { type: Boolean, default: false }
})

const failed = ref({})

watch(
  () => props.thumbs,
  () => { failed.value = {} }
)

const images = computed(() => {
  const list = (props.thumbs || []).filter((_, i) => !failed.value[i])
  return (props.poster ? list.slice(0, 1) : list).slice(0, props.poster ? 1 : 4)
})

function imageSrc(t) {
  let u = mediaUrl(t)
  if (thumbVersion.value && t.startsWith('/api/thumb/')) {
    u += (u.includes('?') ? '&' : '?') + 'v=' + thumbVersion.value
  }
  return u
}

function onErr(i) {
  failed.value = { ...failed.value, [i]: true }
}
</script>
