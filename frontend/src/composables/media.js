// Media URLs are consumed by <img>/<video>, which cannot send the
// Authorization header. We therefore append the JWT as a ?token= query
// parameter so the API can authenticate these requests.
import { ref } from 'vue'

// Bumped whenever thumbnails change on the server (e.g. after a metadata
// refresh downloads new posters) so <img> tags reload instead of using cache.
export const thumbVersion = ref(0)
export function bumpThumbs() {
  thumbVersion.value++
}

export function mediaUrl(url) {
  if (!url) return url
  // never attach the JWT to external hosts (e.g. poster URLs from metadata
  // providers); there is nothing to authenticate and it would leak the token.
  if (/^https?:\/\//i.test(url)) return url
  const token = localStorage.getItem('flycast_token') || ''
  if (!token) return url
  const sep = url.includes('?') ? '&' : '?'
  return url + sep + 'token=' + encodeURIComponent(token)
}
