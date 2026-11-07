package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// FFmpegCaps describes what a given ffmpeg build can do. QNAP ships several
// very different ffmpeg builds: the system one decodes only VP8/VP9, the one
// bundled with StreamDVR can decode H.264 but not encode it, and Emby's build
// (behind the emby-ffmpeg wrapper) is full-featured.
type FFmpegCaps struct {
	Cmd     string
	H264Enc bool // libx264 or a hardware H.264 encoder
	H264Dec bool
	AACEnc  bool
	AACDec  bool
	VP9Dec  bool
	HWEnc   string // "qsv", "vaapi" or "" (software only)
}

func (c FFmpegCaps) Score() int {
	s := 0
	if c.H264Enc {
		s += 100
	}
	if c.H264Dec {
		s += 20
	}
	if c.AACEnc {
		s += 10
	}
	if c.AACDec {
		s += 5
	}
	if c.VP9Dec {
		s += 2
	}
	return s
}

// SelectFFmpeg probes a list of candidate commands and returns the most capable
// one. A non-empty FFMPEG_PATH env var forces that command (but still probes it
// for capabilities).
func SelectFFmpeg() FFmpegCaps {
	forced := os.Getenv("FFMPEG_PATH")
	if forced != "" {
		if caps, ok := probe(forced); ok {
			return caps
		}
		return FFmpegCaps{Cmd: forced}
	}

	best := FFmpegCaps{}
	bestScore := -1
	seen := map[string]bool{}
	for _, c := range ffmpegCandidates() {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		caps, ok := probe(c)
		if !ok {
			continue
		}
		if sc := caps.Score(); sc > bestScore {
			best, bestScore = caps, sc
		}
	}
	if best.Cmd == "" {
		best.Cmd = "ffmpeg"
	}
	return best
}

func ffmpegCandidates() []string {
	var cands []string

	// Bundled next to our own executable (self-contained QPKG). The static
	// binary is shipped both arch-agnostic ("ffmpeg") and arch-specific.
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		cands = append(cands, filepath.Join(dir, "ffmpeg"))
		if runtime.GOARCH == "arm64" {
			cands = append(cands, filepath.Join(dir, "ffmpeg_arm64"))
		} else {
			cands = append(cands, filepath.Join(dir, "ffmpeg_amd64"))
		}
	}

	// Known QNAP locations, most capable first.
	globs := []string{
		"/share/*/.qpkg/EmbyServer/bin/emby-ffmpeg",
		"/share/*/.qpkg/EmbyServer/bin/ffmpeg",
		"/share/*/.qpkg/JellyfinServer/bin/jellyfin-ffmpeg",
		"/share/*/.qpkg/*/bin/ffmpeg",
		"/share/*/.qpkg/StreamDVR/twitchdvr/ffmpeg",
		"/usr/local/medialibrary/bin/ffmpeg",
		"/opt/bin/ffmpeg",
	}
	for _, g := range globs {
		if m, _ := filepath.Glob(g); len(m) > 0 {
			cands = append(cands, m...)
		}
	}

	cands = append(cands, "ffmpeg")
	return cands
}

func probe(cmd string) (FFmpegCaps, bool) {
	if cmd == "" {
		return FFmpegCaps{}, false
	}
	if strings.ContainsRune(cmd, filepath.Separator) {
		if _, err := os.Stat(cmd); err != nil {
			return FFmpegCaps{}, false
		}
	} else if _, err := exec.LookPath(cmd); err != nil {
		return FFmpegCaps{}, false
	}

	enc, err := runList(cmd, "-encoders")
	if err != nil {
		return FFmpegCaps{}, false
	}
	dec, err := runList(cmd, "-decoders")
	if err != nil {
		return FFmpegCaps{}, false
	}

	return FFmpegCaps{
		Cmd:     cmd,
		H264Enc: hasAny(enc, "libx264", "h264_qsv", "h264_vaapi", "h264_nvenc", "h264_v4l2m2m", "openh264"),
		H264Dec: hasAny(dec, "h264"),
		AACEnc:  hasAny(enc, "aac"),
		AACDec:  hasAny(dec, "aac"),
		VP9Dec:  hasAny(dec, "vp9"),
		HWEnc:   hwEncoder(enc),
	}, true
}

// hwEncoder reports the usable hardware H.264 encoder of an ffmpeg build,
// preferring Intel Quick Sync (QSV) over VA-API. v4l2m2m is deliberately
// ignored: Intel iGPUs do not expose a V4L2 M2M encoder, so it only works on
// ARM SoCs and would fail at runtime on QNAP x86 models.
func hwEncoder(enc map[string]bool) string {
	switch {
	case enc["h264_qsv"]:
		return "qsv"
	case enc["h264_vaapi"]:
		return "vaapi"
	default:
		return ""
	}
}

// HWInfo describes a hardware-capable ffmpeg found on the system, kept separate
// from the primary ffmpeg so that probing, thumbnails and remuxing keep using
// our self-contained (stable) build while only transcoding goes through the GPU.
type HWInfo struct {
	Cmd    string
	Enc    string   // "qsv" | "vaapi"
	Env    []string // extra environment (LIBVA_*)
	Device string   // DRM render node for VA-API
}

// SelectHardware looks for an ffmpeg build that can encode H.264 in hardware,
// but only if a usable Intel GPU render node is present. Returns a zero HWInfo
// when hardware transcoding is unavailable.
func SelectHardware() HWInfo {
	if os.Getenv("FLYCAST_NO_HW") != "" {
		return HWInfo{}
	}
	dev := driDevice()
	if dev == "" {
		return HWInfo{}
	}

	var best HWInfo
	for _, c := range ffmpegCandidates() {
		if c == "" {
			continue
		}
		caps, ok := probe(c)
		if !ok || caps.HWEnc == "" {
			continue
		}
		if caps.HWEnc == "qsv" {
			return HWInfo{Cmd: c, Enc: "qsv", Env: hwEnvFor(c), Device: dev}
		}
		if best.Enc == "" {
			best = HWInfo{Cmd: c, Enc: caps.HWEnc, Env: hwEnvFor(c), Device: dev}
		}
	}
	return best
}

// driDevice returns the first accessible DRM render node. Opening it proves the
// process (which runs as "admin" under QNAP) can actually reach the iGPU.
func driDevice() string {
	for _, p := range []string{"/dev/dri/renderD128", "/dev/dri/renderD129"} {
		f, err := os.OpenFile(p, os.O_RDWR, 0)
		if err == nil {
			_ = f.Close()
			return p
		}
	}
	return ""
}

// hwEnvFor derives the LIBVA environment an ffmpeg build needs. Emby/Jellyfin
// bundle their own Intel media driver (iHD/i965) under <pkg>/lib/dri; we point
// libva at it, otherwise the NAS has no usable driver installed system-wide.
func hwEnvFor(cmd string) []string {
	dir := filepath.Dir(cmd)  // .../bin
	base := filepath.Dir(dir) // .../<pkg>
	var dri string
	for _, cand := range []string{
		filepath.Join(base, "lib", "dri"),
		filepath.Join(dir, "..", "lib", "dri"),
	} {
		if fileExists(filepath.Join(cand, "iHD_drv_video.so")) ||
			fileExists(filepath.Join(cand, "i965_drv_video.so")) {
			dri = cand
			break
		}
	}
	name := "iHD"
	if dri != "" && !fileExists(filepath.Join(dri, "iHD_drv_video.so")) {
		name = "i965"
	}
	env := []string{"LIBVA_DRIVER_NAME=" + name}
	if dri != "" {
		env = append(env, "LIBVA_DRIVERS_PATH="+dri)
	}
	return env
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func runList(cmd, flag string) (map[string]bool, error) {
	out, err := exec.Command(cmd, "-hide_banner", flag).CombinedOutput()
	if err != nil {
		return nil, err
	}
	return parseCodecList(string(out)), nil
}

// parseCodecList turns ffmpeg's -encoders/-decoders output into a set of codec
// names. Each data line looks like " V....D libx264   libx264 H.264 ...".
func parseCodecList(out string) map[string]bool {
	res := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && isFlags(f[0]) {
			res[f[1]] = true
		}
	}
	return res
}

func isFlags(s string) bool {
	if len(s) < 3 {
		return false
	}
	for _, r := range s {
		switch r {
		case '.', 'V', 'A', 'S', 'F', 'X', 'B', 'D':
		default:
			return false
		}
	}
	return true
}

func hasAny(set map[string]bool, names ...string) bool {
	for _, n := range names {
		if set[n] {
			return true
		}
	}
	return false
}
