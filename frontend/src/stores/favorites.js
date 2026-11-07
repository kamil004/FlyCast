import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'

export const useFavoritesStore = defineStore('favorites', () => {
  const ids = ref(new Set())
  const loaded = ref(false)

  async function load(force = false) {
    if (loaded.value && !force) return
    try {
      const res = await axios.get('/api/favorites/ids')
      ids.value = new Set(res.data.ids || [])
    } catch (e) {
      ids.value = new Set()
    }
    loaded.value = true
  }

  function has(id) {
    return ids.value.has(id)
  }

  async function toggle(id) {
    const isFav = ids.value.has(id)
    const next = new Set(ids.value)
    if (isFav) next.delete(id)
    else next.add(id)
    ids.value = next
    try {
      if (isFav) await axios.delete(`/api/favorites/${id}`)
      else await axios.post(`/api/favorites/${id}`)
    } catch (e) {
      const reverted = new Set(ids.value)
      if (isFav) reverted.add(id)
      else reverted.delete(id)
      ids.value = reverted
    }
  }

  function reset() {
    ids.value = new Set()
    loaded.value = false
  }

  return { ids, loaded, load, has, toggle, reset }
})
