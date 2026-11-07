<template>
  <main class="page">
    <div class="player-wrap" ref="wrapRef">
      <div class="top" v-show="ui.show">
        <button class="btn ghost" @click="$router.back()">{{ t('player.back') }}</button>
        <div class="title-wrap">
          <span class="title">{{ info.title || info.filename }}</span>
          <span class="muted" v-if="info.year">{{ info.year }}</span>
        </div>
        <div class="spacer"></div>
        <router-link class="btn" :to="{ name: 'detail', params: { id: route.params.id } }">{{ t('player.details') }}</router-link>
      </div>
      <div class="stage" ref="stageRef" :style="stageStyle" @mousemove="showUI" @click="togglePlay" @dblclick="toggleFullscreen">
        <video ref="videoRef" class="video-js vjs-big-play-centered" playsinline></video>
        <transition name="hud">
          <div v-if="hud.show" class="hud">
            <span class="hud-ico">{{ hud.icon }}</span>
            <span class="hud-txt">{{ hud.text }}</span>
            <span v-if="hud.time" class="hud-time">{{ hud.time }}</span>
          </div>
        </transition>
        <div v-if="ui.show" class="controls-bottom">
          <div class="prog">
            <input type="range" min="0" :max="duration||0" :value="currentTime" @input="seek" class="prog-bar" />
            <div class="time">
              <span>{{ fmt(currentTime) }}</span>
              <span>/ {{ fmt(duration||0) }}</span>
            </div>
          </div>
          <div class="btns">
            <button class="btn" @click.stop="prevSeek(-10)">-10s</button>
            <button class="btn" @click.stop="togglePlay">{{ playing ? '⏸' : '▶' }}</button>
            <button class="btn" @click.stop="prevSeek(10)">+10s</button>
            <button class="btn" @click.stop="cycleRate">{{ rate }}x</button>
            <div class="quality-wrap">
              <button class="btn" @click.stop="toggleQualityMenu">{{ qualityLabel }}</button>
              <transition name="qmenu">
                <div v-if="qualityMenuOpen" class="quality-dropdown" @click.stop>
                  <div class="q-source" :class="{active: qualityMode==='source'}" @click="selectQuality('source')">
                    <span class="q-label">{{ t('player.quality_original') }}</span>
                    <span class="q-bit">{{ sourceBitrateKbps }} kbps</span>
                  </div>
                  <div class="q-divider"></div>
                  <div
                    v-for="opt in qualityOptions"
                    :key="opt.label"
                    class="q-option"
                    :class="{active: qualityMode==='custom' && customBitrate===opt.bitrate}"
                    @click="selectQuality('custom', opt.bitrate)"
                  >
                    <span class="q-label">{{ opt.label }}</span>
                    <span class="q-bit">{{ opt.bitrate }} kbps</span>
                  </div>
                </div>
              </transition>
            </div>
            <button class="btn" @click.stop="toggleMute">{{ muted ? '🔇' : '🔊' }}</button>
            <input type="range" min="0" max="1" step="0.01" :value="volume" @input.stop="setVol" class="vol" />
            <button class="btn" @click.stop="toggleFullscreen">⛶</button>
          </div>
        </div>
      </div>
      <div v-if="errorMsg" class="error-banner">
        ⚠️ {{ errorMsg }}
        <button class="btn" @click="forceTranscode">{{ t('player.try_transcode') }}</button>
      </div>
    </div>
  </main>
</template>
<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import videojs from 'video.js'
import { mediaUrl } from '@/composables/media.js'
import { useAuthStore } from '@/stores/auth.js'
import { t } from '@/i18n'
const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const videoRef = ref(null)
const wrapRef = ref(null)
const stageRef = ref(null)
const player = ref(null)
const info = ref({})
const stageStyle = ref({})
const playing = ref(false)
const currentTime = ref(0)
const duration = ref(0)
const muted = ref(false)
const volume = ref(localStorage.getItem('flycast_vol') ? Number(localStorage.getItem('flycast_vol')) : 0.8)
const rate = ref(1)
const errorMsg = ref('')
const hud = ref({show:false,icon:'',text:''})
const ui = ref({show:true})
let hudTimer=null, uiTimer=null

// Transcode seek tracking: for live pipe streams player.currentTime() resets to 0
// after each seek, so we track the offset ourselves.
let _transcodeStart = 0
let _transcodeElapsed = 0

// Quality mode: 'source' = direct play (no transcode), 'custom' = transcode to bitrate
const qualityMode = ref('source') // 'source' | 'custom'
const customBitrate = ref(0)     // target bitrate in kbps when custom
const qualityMenuOpen = ref(false)

// Close quality menu when clicking outside
function closeQualityMenu(e){
  if(!wrapRef.value?.contains(e.target)){
    qualityMenuOpen.value = false
  }
}

// sourceBitrate: calculated from file size and duration (kbps)
const sourceBitrate = computed(() => {
  const size = Number(info.value.size) || 0
  const dur = Number(info.value.duration) || 0
  return dur > 0 && size > 0 ? Math.round(size * 8 / dur / 1000) : 0
})
const sourceBitrateKbps = computed(() => {
  const b = sourceBitrate.value
  return b > 0 ? (b >= 1000 ? (b/1000).toFixed(1) + ' Mbps' : b + ' kbps') : '—'
})

// qualityOptions: preset bitrates with human-readable labels
const qualityOptions = computed(() => {
  const src = sourceBitrate.value
  if (src <= 0) {
    return [
      { label: t('player.quality_low'), bitrate: 1000 },
      { label: t('player.quality_medium'), bitrate: 2500 },
      { label: t('player.quality_high'), bitrate: 5000 },
      { label: t('player.quality_very_high'), bitrate: 8000 },
    ]
  }
  // Suggest bitrates relative to source
  const options = []
  // Always include source
  // Presets: ~25%, ~50%, ~75%, ~100% of source, clamped to sensible range
  const presets = [
    Math.round(src * 0.25),
    Math.round(src * 0.5),
    Math.round(src * 0.75),
    src,
  ]
  // Deduplicate and clamp to 500-16000
  const seen = new Set()
  for (const p of presets) {
    const clamped = Math.max(500, Math.min(16000, p))
    if (!seen.has(clamped)) {
      seen.add(clamped)
      let label
      if (clamped >= 1000) {
        label = (clamped / 1000).toFixed(1) + ' Mbps'
      } else {
        label = clamped + ' kbps'
      }
      // Add ratio hint for non-source options
      const ratio = Math.round(clamped / src * 100)
      if (clamped < src) {
        label = ratio + '% (' + label + ')'
      }
      options.push({ label, bitrate: clamped })
    }
  }
  return options
})

// needsTranscode: true when we're transcoding (custom bitrate)
const needsTranscode = computed(() => {
  if (qualityMode.value === 'source') {
    // Only transcode if the file can't direct play
    return info.value.stream_mode === 'transcode' || info.value.stream_mode === 'remux'
  }
  return true // custom bitrate always transcodes
})

// targetBitrate: the bitrate param to send to the backend (0 = use default)
const targetBitrate = computed(() => {
  if (qualityMode.value === 'custom') return customBitrate.value
  return 0
})

// qualityLabel: displayed on the button
const qualityLabel = computed(() => {
  if (qualityMode.value === 'source') {
    return t('player.quality_label', { label: t('player.quality_original') })
  }
  const b = customBitrate.value
  if (b >= 1000) return t('player.quality_label', { label: (b/1000).toFixed(1) + ' Mbps' })
  return t('player.quality_label', { label: b + ' kbps' })
})

// buildStreamUrl: builds the stream URL with appropriate bitrate/transcode params
let _lastStartPos = 0
function buildStreamUrl(startOverride) {
  const id = route.params.id
  let url = API + '/stream/' + id
  const start = startOverride !== undefined ? startOverride : _lastStartPos
  // If custom bitrate is set, use it
  if (qualityMode.value === 'custom' && customBitrate.value > 0) {
    url += '?bitrate=' + customBitrate.value
  }
  if (needsTranscode.value && start > 0) {
    url += (url.includes('?') ? '&' : '?') + 'start=' + Math.round(start)
  }
  return mediaUrl(url)
}

// reloadStream: reload the video stream preserving playback position
function reloadStream(pos) {
  if (!player.value) return
  const safePos = needsTranscode.value ? Math.max(0, pos) : 0
  _lastStartPos = safePos
  if (needsTranscode.value) {
    _transcodeStart = safePos
    _transcodeElapsed = 0
  }
  const url = buildStreamUrl(safePos)
  const type = needsTranscode.value ? 'video/mp4' : (info.value.stream_type || 'video/mp4')
  player.value.src({ src: url, type })
  if (needsTranscode.value) {
    player.value.play().catch(() => {})
  } else {
    player.value.one('canplay', () => {
      player.value.play().catch(() => {})
    })
  }
}

function forceTranscode(){ window.location.reload() }

function fmt(s){ if(!s||isNaN(s)||s<0) return '0:00'; const h=Math.floor(s/3600),m=Math.floor((s%3600)/60),sec=Math.floor(s%60); return h>0?`${h}:${String(m).padStart(2,'0')}:${String(sec).padStart(2,'0')}`:`${m}:${String(sec).padStart(2,'0')}`}
function showHud(i,t,el=0,du=0){ hud.value={show:true,icon:i,text:t,time:el>0?fmt(el)+' / '+fmt(du):''}; if(hudTimer) clearTimeout(hudTimer); hudTimer=setTimeout(()=>{hud.value.show=false},900)}
function showUI(){ ui.value.show=true; if(uiTimer) clearTimeout(uiTimer); uiTimer=setTimeout(()=>{ui.value.show=false},3000)}
function togglePlay(e){ if(e) e.stopPropagation(); if(!player.value) return; if(player.value.paused()){ player.value.play(); playing.value=true; showHud('▶',t('player.playing')) } else { player.value.pause(); playing.value=false; showHud('⏸',t('player.paused')) }}

// getEffectiveTime: returns the displayed currentTime. For transcoded streams
// the player reports time from 0 (fresh pipe), so we track the offset ourselves.
function getEffectiveTime() {
  if (needsTranscode.value) {
    return _transcodeStart + _transcodeElapsed
  }
  return player.value ? player.value.currentTime() : 0
}

// prevSeek: -10s/+10s buttons. For transcoded streams, reload with start= param.
function prevSeek(d){
  if(!player.value) return
  const effTime = getEffectiveTime()
  const t = Math.max(0, Math.min(duration.value||0, effTime + d))
  if(needsTranscode.value){
    _transcodeStart = t
    _transcodeElapsed = 0
    _lastStartPos = t
    player.value.src({src: buildStreamUrl(t), type:'video/mp4'})
    player.value.play().catch(()=>{})
    showHud(d>0?'+':'-', Math.abs(d)+'s', t, duration.value||0)
  } else {
    player.value.currentTime(t)
    showHud(d>0?'+':'-', Math.abs(d)+'s', t, duration.value||0)
  }
}

// seek: drag the progress bar. For transcoded streams, reload with start= param.
function seek(e){
  const t = Number(e.target.value)
  if(!player.value) return
  if(needsTranscode.value){
    _transcodeStart = t
    _transcodeElapsed = 0
    _lastStartPos = t
    player.value.src({src: buildStreamUrl(t), type:'video/mp4'})
    player.value.play().catch(()=>{})
  } else {
    player.value.currentTime(t)
  }
}

// Quality menu
function toggleQualityMenu(){
  qualityMenuOpen.value = !qualityMenuOpen.value
  if(qualityMenuOpen.value) document.addEventListener('click', closeQualityMenu)
}
function selectQuality(mode, bitrate){
  qualityMenuOpen.value = false
  if (mode === 'source') {
    qualityMode.value = 'source'
    const pos = getEffectiveTime()
    reloadStream(pos)
    showHud('🎯', t('player.quality_original'))
  } else {
    qualityMode.value = 'custom'
    customBitrate.value = bitrate
    const pos = getEffectiveTime()
    reloadStream(pos)
    const b = bitrate >= 1000 ? (bitrate/1000).toFixed(1)+' Mbps' : bitrate+' kbps'
    showHud('🎯', b)
  }
}

function toggleMute(){ if(player.value){ player.value.muted(!player.value.muted()); muted.value=player.value.muted(); showHud(muted.value?'🔇':'🔊', muted.value?t('player.muted'):t('player.unmuted')) }}
function setVol(e){ const v=Number(e.target.value); volume.value=v; localStorage.setItem('flycast_vol', String(v)); if(player.value){ player.value.volume(v); player.value.muted(false); muted.value=false }}
function cycleRate(){ const r=[1,1.25,1.5,1.75,2,0.75,0.5]; const i=(r.indexOf(rate.value)+1)%r.length; rate.value=r[i]; if(player.value){ player.value.playbackRate(rate.value); showHud('⚡', rate.value+'x') }}
function toggleFullscreen(){ const el=wrapRef.value||stageRef.value||document.body; if(!document.fullscreenElement){ el.requestFullscreen?.().catch(()=>{}) } else { document.exitFullscreen?.() }}

onMounted(async()=>{
  try{ const r=await axios.get(API+'/videos/'+route.params.id); info.value=r.data }catch(e){}
  if(player.value) player.value.dispose()
  player.value=videojs(videoRef.value, {autoplay:true,controls:false,preload:'auto',responsive:true,fluid:true,sources:[{src:buildStreamUrl(), type:needsTranscode.value?'video/mp4':(info.value.stream_type||'video/mp4')}]}),
  player.value.volume(volume.value)

  // For transcoded streams, total duration comes from the API (info.value.duration)
  // because the live stream only reports buffered duration.
  duration.value = Number(info.value.duration) || 0

  player.value.on('timeupdate',()=>{
    if(!needsTranscode.value){
      currentTime.value = player.value.currentTime()
      duration.value = player.value.duration() || duration.value
    } else {
      _transcodeElapsed = player.value ? player.value.currentTime() : 0
      currentTime.value = getEffectiveTime()
    }
  })
  player.value.on('progress',()=>{ if(!needsTranscode.value){ duration.value=player.value.duration()||duration.value } })
  player.value.on('play',()=>{playing.value=true })
  player.value.on('pause',()=>{playing.value=false})
  player.value.on('volumechange',()=>{muted.value=player.value.muted(); volume.value=player.value.volume(); localStorage.setItem('flycast_vol', String(volume.value))})
  player.value.ready(()=>{ showUI() })
  document.addEventListener('keydown', keyHandler)
})
function keyHandler(e){
  if(e.target.tagName==='INPUT') return
  if(e.code==='Space'){ e.preventDefault(); togglePlay() }
  if(e.code==='ArrowLeft'){ e.preventDefault(); prevSeek(-10) }
  if(e.code==='ArrowRight'){ e.preventDefault(); prevSeek(10) }
  if(e.code==='ArrowUp'){ e.preventDefault(); volume.value=Math.min(1,volume.value+0.05); if(player.value){ player.value.volume(volume.value); player.value.muted(false); muted.value=false } showHud('🔊', Math.round(volume.value*100)+'%') }
  if(e.code==='ArrowDown'){ e.preventDefault(); volume.value=Math.max(0,volume.value-0.05); if(player.value){ player.value.volume(volume.value); player.value.muted(false); muted.value=false } showHud('🔊', Math.round(volume.value*100)+'%') }
  if(e.code==='KeyF'){ e.preventDefault(); toggleFullscreen() }
  if(e.code==='KeyM'){ e.preventDefault(); toggleMute() }
  if(e.shiftKey && (e.key==='<' || e.code==='Comma')){ e.preventDefault(); const r=Math.max(0.25, +(rate.value-0.25).toFixed(2)); rate.value=r; if(player.value){ player.value.playbackRate(r); showHud('⚡', r+'x') } }
  if(e.shiftKey && (e.key==='>' || e.code==='Period')){ e.preventDefault(); const r=Math.min(4, +(rate.value+0.25).toFixed(2)); rate.value=r; if(player.value){ player.value.playbackRate(r); showHud('⚡', r+'x') } }
}
onBeforeUnmount(()=>{ if(hudTimer) clearTimeout(hudTimer); if(uiTimer) clearTimeout(uiTimer); if(player.value) player.value.dispose(); document.removeEventListener('keydown', keyHandler); document.removeEventListener('click', closeQualityMenu) })
</script>
<style scoped>
.player-wrap{display:flex;flex-direction:column;height:calc(100vh - 100px);max-height:calc(100vh - 100px);overflow:hidden;margin:0 12px}
.top{display:flex;align-items:center;gap:10px;padding:8px 10px;position:absolute;top:0;left:50%;transform:translateX(-50%);width:100%;max-width:100%;background:linear-gradient(180deg,rgba(0,0,0,.9),rgba(0,0,0,.5),transparent);z-index:5;box-sizing:border-box}
.title-wrap{display:flex;flex-direction:column;gap:2px}
.title{font-weight:600;font-size:13px}
.muted{color:var(--dim-2);font-size:11px}
.spacer{flex:1}
.stage{flex:1;position:relative;background:#000;display:flex;align-items:center;justify-content:center;overflow:hidden;border-radius:10px}
:::deep(.video-js){width:100%!important;height:100%!important}
.controls-bottom{bottom:0;position:absolute;bottom:0;left:0;right:0;width:100%;max-width:100%;box-sizing:border-box;background:linear-gradient(0deg,rgba(0,0,0,.95),rgba(0,0,0,.6),transparent);padding:12px 12px 6px;display:flex;flex-direction:column;gap:8px;z-index:5}
.prog{display:flex;flex-direction:column;gap:4px}
.prog-bar{width:100%}
.time{display:flex;justify-content:space-between;font-size:12px;color:#ccc}
.btns{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.vol{width:100px}

/* Quality dropdown */
.quality-wrap{position:relative;display:inline-block}
.quality-dropdown{
  position:absolute;bottom:calc(100% + 8px);left:50%;transform:translateX(-50%);
  background:rgba(20,20,30,.97);border:1px solid rgba(255,255,255,.12);
  border-radius:10px;padding:6px 0;min-width:220px;z-index:10;
  backdrop-filter:blur(12px);box-shadow:0 8px 32px rgba(0,0,0,.6);
}
.q-source,.q-option{
  display:flex;justify-content:space-between;align-items:center;
  padding:8px 16px;cursor:pointer;font-size:13px;color:#ddd;
  transition:background .15s;
}
.q-source:hover,.q-option:hover{background:rgba(255,255,255,.08)}
.q-source.active,.q-option.active{background:rgba(80,140,255,.2);color:#fff}
.q-label{font-weight:500}
.q-bit{font-family:monospace;color:#999;font-size:12px}
.q-source.active .q-bit,.q-option.active .q-bit{color:#aac4ff}
.q-divider{height:1px;background:rgba(255,255,255,.08);margin:4px 0}

.hud{position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);background:rgba(0,0,0,.75);padding:12px 20px;border-radius:12px;display:flex;align-items:center;gap:10px;backdrop-filter:blur(4px);z-index:6}
.hud-time{color:#fff;font-size:14px;font-weight:600}
.error-banner{padding:10px;background:var(--danger);color:#fff;display:flex;align-items:center;gap:10px}

/* Transitions */
.qmenu-enter-active{transition:all .15s ease}
.qmenu-leave-active{transition:all .1s ease}
.qmenu-enter-from{opacity:0;transform:translateX(-50%) translateY(8px)}
.qmenu-leave-to{opacity:0;transform:translateX(-50%) translateY(8px)}
.hud-enter-active{transition:all .2s ease}
.hud-leave-active{transition:all .15s ease}
.hud-enter-from,.hud-leave-to{opacity:0;transform:translate(-50%,-50%) scale(.9)}
</style>
